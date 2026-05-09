package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultAliasModel     = "inclusionai/ring-2.6-1t:free"
	DefaultHost           = "127.0.0.1"
	DefaultPort           = 8787
	DefaultOpenRouterURL  = "https://openrouter.ai/api/v1"
	DefaultOpenRouterName = "openrouter"
	DefaultTitle          = "Claude Gateway"
)

type Provider struct {
	Profile      string
	BaseURL      string
	APIKey       string
	APIKeyEnv    string
	Referrer     string
	Title        string
	Capabilities ProviderCapabilities
}

type ProviderCapabilities struct {
	Streaming  bool
	Tools      bool
	JSONMode   bool
	configured bool
}

func DefaultProviderCapabilities() ProviderCapabilities {
	return ProviderCapabilities{Streaming: true, Tools: true, configured: true}
}

func (c ProviderCapabilities) Effective() ProviderCapabilities {
	if !c.configured {
		return DefaultProviderCapabilities()
	}
	return c
}

type Route struct {
	Provider    string
	Model       string
	DisplayName string
}

type DesktopModel struct {
	ID            string
	DisplayName   string
	Provider      string
	UpstreamModel string
}

type FileSummary struct {
	Path             string            `json:"path"`
	Host             string            `json:"host"`
	Port             int               `json:"port"`
	GatewayAPIKeyEnv string            `json:"gatewayApiKeyEnv"`
	TLSCertFile      string            `json:"tlsCertFile"`
	TLSKeyFile       string            `json:"tlsKeyFile"`
	Providers        []ProviderSummary `json:"providers"`
	Routes           []RouteSummary    `json:"routes"`
}

type ProviderSummary struct {
	Name         string               `json:"name"`
	Profile      string               `json:"profile"`
	BaseURL      string               `json:"baseUrl"`
	APIKeyEnv    string               `json:"apiKeyEnv"`
	Capabilities ProviderCapabilities `json:"capabilities"`
}

type RouteSummary struct {
	DesktopID     string `json:"desktopID"`
	DisplayName   string `json:"displayName"`
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstreamModel"`
}

type Config struct {
	Host          string
	Port          int
	GatewayAPIKey string
	TLSCertFile   string
	TLSKeyFile    string
	Providers     map[string]Provider
	Routes        map[string][]Route
}

func LoadFromOSEnv() (Config, error) {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	if path := strings.TrimSpace(env["CLAUDE_GATEWAY_CONFIG"]); path != "" {
		return LoadFromFile(path, env)
	}
	return LoadFromEnv(env)
}

func LoadFromEnv(env map[string]string) (Config, error) {
	apiKey := strings.TrimSpace(env["OPENROUTER_API_KEY"])
	if apiKey == "" {
		return Config{}, errors.New("OPENROUTER_API_KEY is required")
	}

	defaultModel := strings.TrimSpace(env["CLAUDE_GATEWAY_DEFAULT_MODEL"])
	if defaultModel == "" {
		defaultModel = DefaultAliasModel
	}

	baseURL := strings.TrimSpace(env["OPENROUTER_BASE_URL"])
	if baseURL == "" {
		baseURL = DefaultOpenRouterURL
	}

	cfg := Config{
		Host:          valueOrDefault(env["HOST"], DefaultHost),
		Port:          parsePort(env["PORT"]),
		GatewayAPIKey: strings.TrimSpace(env["CLAUDE_GATEWAY_API_KEY"]),
		TLSCertFile:   strings.TrimSpace(env["TLS_CERT_FILE"]),
		TLSKeyFile:    strings.TrimSpace(env["TLS_KEY_FILE"]),
		Providers: map[string]Provider{
			DefaultOpenRouterName: {
				Profile:      "openai-chat",
				BaseURL:      strings.TrimRight(baseURL, "/"),
				APIKey:       apiKey,
				Referrer:     strings.TrimSpace(env["OPENROUTER_REFERRER"]),
				Title:        valueOrDefault(env["OPENROUTER_TITLE"], DefaultTitle),
				Capabilities: DefaultProviderCapabilities(),
			},
		},
		Routes: routesFromAliases(parseAliasEnv(env["CLAUDE_MODEL_ALIASES"], defaultModel)),
	}

	if err := validateNetworkExposure(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

type fileProvider struct {
	Profile      string            `json:"profile"`
	BaseURL      string            `json:"baseUrl"`
	APIKey       string            `json:"apiKey,omitempty"`
	APIKeyEnv    string            `json:"apiKeyEnv"`
	Referrer     string            `json:"referrer"`
	Title        string            `json:"title"`
	Capabilities *fileCapabilities `json:"capabilities"`
}

type fileCapabilities struct {
	Streaming *bool `json:"streaming"`
	Tools     *bool `json:"tools"`
	JSONMode  *bool `json:"jsonMode"`
}

type fileRoute struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	DisplayName string `json:"displayName"`
}

type fileConfig struct {
	Host             string                  `json:"host"`
	Port             int                     `json:"port"`
	GatewayAPIKey    string                  `json:"gatewayApiKey,omitempty"`
	GatewayAPIKeyEnv string                  `json:"gatewayApiKeyEnv"`
	TLSCertFile      string                  `json:"tlsCertFile"`
	TLSKeyFile       string                  `json:"tlsKeyFile"`
	DefaultModel     string                  `json:"defaultModel"`
	Providers        map[string]fileProvider `json:"providers"`
	Routes           map[string][]fileRoute  `json:"routes"`
}

func LoadFromFile(path string, env map[string]string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}
	if strings.TrimSpace(raw.GatewayAPIKey) != "" {
		return Config{}, errors.New("gatewayApiKey is not allowed in config files; use gatewayApiKeyEnv")
	}

	defaultModel := strings.TrimSpace(raw.DefaultModel)
	if defaultModel == "" {
		defaultModel = DefaultAliasModel
	}

	gatewayAPIKeyEnv := strings.TrimSpace(raw.GatewayAPIKeyEnv)
	cfg := Config{
		Host:        valueOrDefault(raw.Host, DefaultHost),
		Port:        raw.Port,
		TLSCertFile: strings.TrimSpace(raw.TLSCertFile),
		TLSKeyFile:  strings.TrimSpace(raw.TLSKeyFile),
		Providers:   map[string]Provider{},
		Routes:      map[string][]Route{},
	}
	if gatewayAPIKeyEnv != "" {
		cfg.GatewayAPIKey = strings.TrimSpace(env[gatewayAPIKeyEnv])
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = DefaultPort
	}

	for name, provider := range raw.Providers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if strings.TrimSpace(provider.APIKey) != "" {
			return Config{}, fmt.Errorf("provider %q apiKey is not allowed in config files; use apiKeyEnv", name)
		}
		loaded := Provider{
			Profile:      valueOrDefault(provider.Profile, "openai-chat"),
			BaseURL:      strings.TrimRight(valueOrDefault(provider.BaseURL, DefaultOpenRouterURL), "/"),
			APIKeyEnv:    strings.TrimSpace(provider.APIKeyEnv),
			Referrer:     strings.TrimSpace(provider.Referrer),
			Title:        valueOrDefault(provider.Title, DefaultTitle),
			Capabilities: capabilitiesFromFile(provider.Capabilities),
		}
		if loaded.APIKey == "" && loaded.APIKeyEnv != "" {
			loaded.APIKey = strings.TrimSpace(env[loaded.APIKeyEnv])
		}
		if loaded.APIKey == "" {
			return Config{}, fmt.Errorf("provider %q apiKey or apiKeyEnv is required", name)
		}
		cfg.Providers[name] = loaded
	}

	if len(cfg.Providers) == 0 {
		return Config{}, errors.New("at least one provider is required")
	}

	if len(raw.Routes) == 0 {
		cfg.Routes = routesFromAliases(parseAliasEnv("", defaultModel))
	} else {
		for alias, routes := range raw.Routes {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			for _, route := range routes {
				provider := strings.TrimSpace(route.Provider)
				model := strings.TrimSpace(route.Model)
				if provider == "" || model == "" {
					continue
				}
				if _, ok := cfg.Providers[provider]; !ok {
					return Config{}, fmt.Errorf("route %q references unknown provider %q", alias, provider)
				}
				cfg.Routes[alias] = append(cfg.Routes[alias], Route{
					Provider:    provider,
					Model:       model,
					DisplayName: strings.TrimSpace(route.DisplayName),
				})
			}
		}
	}

	if len(cfg.Routes) == 0 {
		return Config{}, errors.New("at least one route is required")
	}

	if err := validateNetworkExposure(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func InspectFile(path string) (FileSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileSummary{}, fmt.Errorf("read config file: %w", err)
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return FileSummary{}, fmt.Errorf("parse config file: %w", err)
	}
	if strings.TrimSpace(raw.GatewayAPIKey) != "" {
		return FileSummary{}, errors.New("gatewayApiKey is not allowed in config files; use gatewayApiKeyEnv")
	}

	summary := FileSummary{
		Path:             path,
		Host:             valueOrDefault(raw.Host, DefaultHost),
		Port:             raw.Port,
		GatewayAPIKeyEnv: strings.TrimSpace(raw.GatewayAPIKeyEnv),
		TLSCertFile:      strings.TrimSpace(raw.TLSCertFile),
		TLSKeyFile:       strings.TrimSpace(raw.TLSKeyFile),
	}
	if summary.Port <= 0 || summary.Port > 65535 {
		summary.Port = DefaultPort
	}

	providerNames := make([]string, 0, len(raw.Providers))
	for name := range raw.Providers {
		name = strings.TrimSpace(name)
		if name != "" {
			providerNames = append(providerNames, name)
		}
	}
	sort.Strings(providerNames)
	for _, name := range providerNames {
		provider := raw.Providers[name]
		if strings.TrimSpace(provider.APIKey) != "" {
			return FileSummary{}, fmt.Errorf("provider %q apiKey is not allowed in config files; use apiKeyEnv", name)
		}
		summary.Providers = append(summary.Providers, ProviderSummary{
			Name:         name,
			Profile:      valueOrDefault(provider.Profile, "openai-chat"),
			BaseURL:      strings.TrimRight(valueOrDefault(provider.BaseURL, DefaultOpenRouterURL), "/"),
			APIKeyEnv:    strings.TrimSpace(provider.APIKeyEnv),
			Capabilities: capabilitiesFromFile(provider.Capabilities),
		})
	}
	if len(summary.Providers) == 0 {
		return FileSummary{}, errors.New("at least one provider is required")
	}

	routes := raw.Routes
	if len(routes) == 0 {
		defaultModel := strings.TrimSpace(raw.DefaultModel)
		if defaultModel == "" {
			defaultModel = DefaultAliasModel
		}
		routes = fileRoutesFromRoutes(routesFromAliases(parseAliasEnv("", defaultModel)))
	}
	knownProviders := map[string]bool{}
	for _, provider := range summary.Providers {
		knownProviders[provider.Name] = true
	}
	routeAliases := make([]string, 0, len(routes))
	for alias := range routes {
		alias = strings.TrimSpace(alias)
		if alias != "" {
			routeAliases = append(routeAliases, alias)
		}
	}
	sort.Strings(routeAliases)
	for _, alias := range routeAliases {
		for _, route := range routes[alias] {
			provider := strings.TrimSpace(route.Provider)
			model := strings.TrimSpace(route.Model)
			if provider == "" || model == "" {
				continue
			}
			if !knownProviders[provider] {
				return FileSummary{}, fmt.Errorf("route %q references unknown provider %q", alias, provider)
			}
			desktopID := alias
			if desktopID == model {
				desktopID = DefaultDesktopModelID(model)
			}
			displayName := strings.TrimSpace(route.DisplayName)
			if displayName == "" {
				displayName = desktopID
			}
			summary.Routes = append(summary.Routes, RouteSummary{
				DesktopID:     desktopID,
				DisplayName:   displayName,
				Provider:      provider,
				UpstreamModel: model,
			})
		}
	}
	if len(summary.Routes) == 0 {
		return FileSummary{}, errors.New("at least one route is required")
	}
	sort.Slice(summary.Routes, func(i int, j int) bool {
		return summary.Routes[i].DesktopID < summary.Routes[j].DesktopID
	})

	return summary, nil
}

func (c Config) ResolveRoute(model string) (Route, bool) {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return Route{}, false
	}
	if routes := c.Routes[trimmed]; len(routes) > 0 {
		return routes[0], true
	}
	if _, ok := c.Providers[DefaultOpenRouterName]; ok {
		return Route{Provider: DefaultOpenRouterName, Model: trimmed}, true
	}
	return Route{}, false
}

func (c Config) ProviderFor(route Route) (Provider, bool) {
	provider, ok := c.Providers[route.Provider]
	return provider, ok
}

func (c Config) ModelIDs() []string {
	return c.DesktopModelIDs()
}

func (c Config) DesktopModels() []DesktopModel {
	models := make([]DesktopModel, 0, len(c.Routes))
	for alias, routes := range c.Routes {
		alias = strings.TrimSpace(alias)
		if alias == "" || len(routes) == 0 {
			continue
		}
		route := routes[0]
		upstreamModel := strings.TrimSpace(route.Model)
		if upstreamModel == "" {
			continue
		}
		id := alias
		if id == upstreamModel {
			id = DefaultDesktopModelID(upstreamModel)
		}
		displayName := strings.TrimSpace(route.DisplayName)
		if displayName == "" {
			displayName = id
		}
		models = append(models, DesktopModel{
			ID:            id,
			DisplayName:   displayName,
			Provider:      strings.TrimSpace(route.Provider),
			UpstreamModel: upstreamModel,
		})
	}
	sort.Slice(models, func(i int, j int) bool {
		return models[i].ID < models[j].ID
	})
	return models
}

func (c Config) DesktopModelIDs() []string {
	seen := map[string]bool{}
	for _, model := range c.DesktopModels() {
		if model.ID != "" {
			seen[model.ID] = true
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func DefaultDesktopModelID(upstream string) string {
	trimmed := strings.TrimSpace(upstream)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "claude-") || strings.HasPrefix(trimmed, "anthropic/claude-") {
		return trimmed
	}
	return "claude-" + trimmed
}

func routesFromAliases(aliases map[string]string) map[string][]Route {
	routes := make(map[string][]Route, len(aliases))
	for alias, model := range aliases {
		routes[alias] = []Route{{Provider: DefaultOpenRouterName, Model: model}}
	}
	return routes
}

func fileRoutesFromRoutes(routes map[string][]Route) map[string][]fileRoute {
	converted := make(map[string][]fileRoute, len(routes))
	for alias, routeList := range routes {
		for _, route := range routeList {
			converted[alias] = append(converted[alias], fileRoute{
				Provider:    route.Provider,
				Model:       route.Model,
				DisplayName: route.DisplayName,
			})
		}
	}
	return converted
}

func capabilitiesFromFile(raw *fileCapabilities) ProviderCapabilities {
	capabilities := DefaultProviderCapabilities()
	if raw == nil {
		return capabilities
	}
	if raw.Streaming != nil {
		capabilities.Streaming = *raw.Streaming
	}
	if raw.Tools != nil {
		capabilities.Tools = *raw.Tools
	}
	if raw.JSONMode != nil {
		capabilities.JSONMode = *raw.JSONMode
	}
	return capabilities
}

func parseAliasEnv(value string, defaultModel string) map[string]string {
	aliases := map[string]string{
		"claude-opus-4-7":   defaultModel,
		"claude-opus-4.7":   defaultModel,
		"claude-sonnet-4-6": defaultModel,
		"claude-sonnet-4.6": defaultModel,
		"claude-haiku-4-5":  defaultModel,
		"claude-haiku-4.5":  defaultModel,
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return aliases
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(value), &parsed); err == nil {
		for key, model := range parsed {
			key = strings.TrimSpace(key)
			model = strings.TrimSpace(model)
			if key != "" && model != "" {
				aliases[key] = model
			}
		}
		return aliases
	}

	for _, pair := range strings.Split(value, ",") {
		key, model, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		model = strings.TrimSpace(model)
		if key != "" && model != "" {
			aliases[key] = model
		}
	}
	return aliases
}

func parsePort(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultPort
	}
	port, err := strconv.Atoi(trimmed)
	if err != nil || port <= 0 || port > 65535 {
		return DefaultPort
	}
	return port
}

func valueOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) TLSEnabled() bool {
	return c.TLSCertFile != "" || c.TLSKeyFile != ""
}

func (c Config) Scheme() string {
	if c.TLSEnabled() {
		return "https"
	}
	return "http"
}

func validateNetworkExposure(cfg Config) error {
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return errors.New("both TLS_CERT_FILE and TLS_KEY_FILE are required when TLS is enabled")
	}
	if isLoopbackHost(cfg.Host) {
		return nil
	}
	if strings.TrimSpace(cfg.GatewayAPIKey) == "" {
		return fmt.Errorf("gateway API key is required when binding to non-loopback host %q; set CLAUDE_GATEWAY_API_KEY or gatewayApiKeyEnv", cfg.Host)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	trimmed := strings.TrimSpace(strings.Trim(host, "[]"))
	if trimmed == "" || strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}
