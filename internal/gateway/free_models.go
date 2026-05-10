package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/claude-desktop-gateway/internal/config"
)

const defaultFreeModelCatalogCacheTTL = 10 * time.Minute

type openRouterModelCatalog struct {
	client *http.Client
	now    func() time.Time
	mu     sync.Mutex
	cache  map[string]freeModelCatalogEntry
}

type freeModelCatalogEntry struct {
	expiresAt time.Time
	models    []string
}

type openRouterModelsResponse struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID                  string   `json:"id"`
	ContextLength       int      `json:"context_length"`
	SupportedParameters []string `json:"supported_parameters"`
	Pricing             struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`
}

type freeModelCandidate struct {
	id            string
	contextLength int
}

func newOpenRouterModelCatalog(client *http.Client) *openRouterModelCatalog {
	return &openRouterModelCatalog{
		client: client,
		now:    time.Now,
		cache:  map[string]freeModelCatalogEntry{},
	}
}

func (g *Gateway) dynamicFreeRoutes(ctx context.Context, provider config.Provider, route config.Route, req anthropicRequest) []config.Route {
	models, err := g.catalog.freeModels(ctx, provider, route.DynamicFreeModels, req)
	if err != nil || len(models) == 0 {
		models = fallbackFreeModels(route)
	} else {
		models = appendDistinctModels(models, route.DynamicFreeModels.Fallback...)
	}
	if len(models) == 0 && strings.TrimSpace(route.Model) != "" {
		models = []string{strings.TrimSpace(route.Model)}
	}

	routes := make([]config.Route, 0, len(models))
	for _, model := range models {
		next := route
		next.Model = model
		next.DynamicFreeModels = config.DynamicFreeModels{}
		routes = append(routes, next)
	}
	return routes
}

func (c *openRouterModelCatalog) freeModels(ctx context.Context, provider config.Provider, selector config.DynamicFreeModels, req anthropicRequest) ([]string, error) {
	required := effectiveRequiredParameters(selector, req)
	key := freeModelCatalogCacheKey(provider, selector, required)
	if cached, ok := c.cached(key); ok {
		return cached, nil
	}

	models, err := c.fetchFreeModels(ctx, provider, selector, required)
	if err != nil {
		return nil, err
	}
	c.store(key, models, freeModelCatalogTTL(selector))
	return append([]string(nil), models...), nil
}

func (c *openRouterModelCatalog) cached(key string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.cache[key]
	if !ok || !c.now().Before(entry.expiresAt) {
		return nil, false
	}
	return append([]string(nil), entry.models...), true
}

func (c *openRouterModelCatalog) store(key string, models []string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = freeModelCatalogEntry{
		expiresAt: c.now().Add(ttl),
		models:    append([]string(nil), models...),
	}
}

func (c *openRouterModelCatalog) fetchFreeModels(ctx context.Context, provider config.Provider, selector config.DynamicFreeModels, required []string) ([]string, error) {
	endpoint, err := url.Parse(strings.TrimRight(provider.BaseURL, "/") + "/models")
	if err != nil {
		return nil, fmt.Errorf("parse OpenRouter models URL: %w", err)
	}
	if len(required) > 0 {
		query := endpoint.Query()
		query.Set("supported_parameters", strings.Join(required, ","))
		endpoint.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter models request: %w", err)
	}
	setOpenAIHeaders(req.Header, provider)

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch OpenRouter models: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("fetch OpenRouter models: status %d", res.StatusCode)
	}

	var body openRouterModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode OpenRouter models: %w", err)
	}
	return filterFreeModels(body.Data, selector, required), nil
}

func filterFreeModels(models []openRouterModel, selector config.DynamicFreeModels, required []string) []string {
	candidates := make([]freeModelCandidate, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ID)
		if id == "" || !isZeroPrice(model.Pricing.Prompt) || !isZeroPrice(model.Pricing.Completion) {
			continue
		}
		if selector.MinContextLength > 0 && model.ContextLength < selector.MinContextLength {
			continue
		}
		if !modelSupportsAll(model.SupportedParameters, required) {
			continue
		}
		candidates = append(candidates, freeModelCandidate{id: id, contextLength: model.ContextLength})
	}
	sort.Slice(candidates, func(i int, j int) bool {
		if candidates[i].contextLength != candidates[j].contextLength {
			return candidates[i].contextLength > candidates[j].contextLength
		}
		return candidates[i].id < candidates[j].id
	})
	if selector.MaxModels > 0 && len(candidates) > selector.MaxModels {
		candidates = candidates[:selector.MaxModels]
	}

	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.id)
	}
	return result
}

func effectiveRequiredParameters(selector config.DynamicFreeModels, req anthropicRequest) []string {
	required := append([]string(nil), selector.RequiredParameters...)
	if len(req.Tools) > 0 {
		required = append(required, "tools")
		if len(req.ToolChoice) > 0 {
			required = append(required, "tool_choice")
		}
	}
	return cleanList(required)
}

func freeModelCatalogCacheKey(provider config.Provider, selector config.DynamicFreeModels, required []string) string {
	parts := []string{
		strings.TrimRight(provider.BaseURL, "/"),
		strings.Join(required, ","),
		strconv.Itoa(selector.MinContextLength),
		strconv.Itoa(selector.MaxModels),
	}
	return strings.Join(parts, "\x00")
}

func freeModelCatalogTTL(selector config.DynamicFreeModels) time.Duration {
	if selector.CatalogCacheTTLSeconds <= 0 {
		return defaultFreeModelCatalogCacheTTL
	}
	return time.Duration(selector.CatalogCacheTTLSeconds) * time.Second
}

func fallbackFreeModels(route config.Route) []string {
	models := append([]string(nil), route.DynamicFreeModels.Fallback...)
	if len(models) == 0 {
		models = append(models, route.Model)
	}
	return cleanList(models)
}

func appendDistinctModels(models []string, fallback ...string) []string {
	result := cleanList(models)
	seen := map[string]bool{}
	for _, model := range result {
		seen[model] = true
	}
	for _, model := range fallback {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		result = append(result, trimmed)
		seen[trimmed] = true
	}
	return result
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		cleaned = append(cleaned, trimmed)
		seen[trimmed] = true
	}
	return cleaned
}

func modelSupportsAll(supported []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	available := map[string]bool{}
	for _, parameter := range supported {
		available[strings.TrimSpace(parameter)] = true
	}
	for _, parameter := range required {
		if !available[parameter] {
			return false
		}
	}
	return true
}

func isZeroPrice(value string) bool {
	price, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && price == 0
}

func replaceRoute(routes []config.Route, index int, replacement []config.Route) []config.Route {
	if len(replacement) == 0 {
		return append(append([]config.Route(nil), routes[:index]...), routes[index+1:]...)
	}
	next := make([]config.Route, 0, len(routes)-1+len(replacement))
	next = append(next, routes[:index]...)
	next = append(next, replacement...)
	next = append(next, routes[index+1:]...)
	return next
}
