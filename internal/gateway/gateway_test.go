package gateway_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gateway"
)

func testConfig(baseURL string) config.Config {
	return config.Config{
		Host:          "127.0.0.1",
		Port:          8787,
		GatewayAPIKey: "client-test-key",
		Providers: map[string]config.Provider{
			"openrouter": {
				Profile:      "openai-chat",
				BaseURL:      baseURL,
				APIKey:       "or-test-key",
				Referrer:     "https://codex-proxy.local",
				Title:        "Claude Desktop Gateway",
				Capabilities: config.DefaultProviderCapabilities(),
			},
		},
		Routes: map[string][]config.Route{
			"claude-opus-4-7": {
				{Provider: "openrouter", Model: config.DefaultAliasModel, DisplayName: "OpenRouter Ring Free"},
			},
			"claude-sonnet-4-6": {
				{Provider: "openrouter", Model: config.DefaultAliasModel},
			},
			"claude-haiku-4-5": {
				{Provider: "openrouter", Model: config.DefaultAliasModel},
			},
		},
	}
}

func testAnthropicMessagesConfig(baseURL string) config.Config {
	cfg := testConfig(baseURL)
	provider := cfg.Providers["openrouter"]
	provider.Profile = "anthropic-messages"
	cfg.Providers["openrouter"] = provider
	cfg.Routes = map[string][]config.Route{
		"claude-ring-2-6-1t-free": {
			{Provider: "openrouter", Model: "inclusionai/ring-2.6-1t:free", DisplayName: "OpenRouter Ring Free"},
		},
	}
	return cfg
}

func authHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-test-key")
}

func TestGatewayRequiresClientAPIKey(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	app := gateway.New(testConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
}

func TestGatewayListsConfiguredModels(t *testing.T) {
	app := gateway.New(testConfig("https://openrouter.ai/api/v1"), http.DefaultClient)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID          string `json:"id"`
			OwnedBy     string `json:"owned_by"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	if body.Object != "list" {
		t.Fatalf("object = %q", body.Object)
	}
	ids := map[string]bool{}
	displayNames := map[string]string{}
	owners := map[string]string{}
	for _, model := range body.Data {
		ids[model.ID] = true
		displayNames[model.ID] = model.DisplayName
		owners[model.ID] = model.OwnedBy
	}
	if !ids["claude-opus-4-7"] || ids[config.DefaultAliasModel] {
		t.Fatalf("model ids should expose desktop aliases only: %#v", ids)
	}
	if displayNames["claude-opus-4-7"] != "OpenRouter Ring Free" {
		t.Fatalf("display name = %q", displayNames["claude-opus-4-7"])
	}
	if owners["claude-opus-4-7"] != "openrouter" {
		t.Fatalf("owner = %q", owners["claude-opus-4-7"])
	}
}

func TestGatewayTranslatesAnthropicMessagesToOpenRouter(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		upstreamHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"gen-1","choices":[{"message":{"role":"assistant","content":"hello from openrouter"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":5}}`)
	}))
	defer upstream.Close()

	app := gateway.New(testConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":128,"system":"You are direct.","messages":[{"role":"user","content":"hello"}],"temperature":0.2}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := upstreamHeaders.Get("Authorization"); got != "Bearer or-test-key" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := upstreamHeaders.Get("HTTP-Referer"); got != "https://codex-proxy.local" {
		t.Fatalf("HTTP-Referer = %q", got)
	}
	if got := upstreamHeaders.Get("X-Title"); got != "Claude Desktop Gateway" {
		t.Fatalf("X-Title = %q", got)
	}
	if upstreamBody["model"] != config.DefaultAliasModel {
		t.Fatalf("upstream model = %v", upstreamBody["model"])
	}
	if upstreamBody["stream"] != false {
		t.Fatalf("upstream stream = %v", upstreamBody["stream"])
	}
	messages, ok := upstreamBody["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %#v", upstreamBody["messages"])
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	if body["id"] != "gen-1" || body["model"] != "claude-opus-4-7" || body["stop_reason"] != "end_turn" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestGatewayProxiesAnthropicMessagesProfileToOpenRouterMessages(t *testing.T) {
	var upstreamBody map[string]json.RawMessage
	var upstreamHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		upstreamHeaders = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"msg_1",
			"type":"message",
			"role":"assistant",
			"model":"inclusionai/ring-2.6-1t:free",
			"content":[
				{"type":"thinking","thinking":"use lookup","signature":""},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"key":"alpha"}}
			],
			"stop_reason":"tool_use",
			"stop_sequence":null,
			"usage":{
				"input_tokens":12,
				"output_tokens":3,
				"prompt_tokens_details":{"cached_tokens":4,"cache_write_tokens":8}
			}
		}`)
	}))
	defer upstream.Close()

	app := gateway.New(testAnthropicMessagesConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude-ring-2-6-1t-free",
		"max_tokens":256,
		"system":[{"type":"text","text":"stable prefix","cache_control":{"type":"ephemeral"}}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"use lookup"},{"type":"tool_result","tool_use_id":"toolu_1","content":"value"}]}
		],
		"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{"key":{"type":"string"}}}}],
		"tool_choice":{"type":"tool","name":"lookup"},
		"thinking":{"type":"enabled","budget_tokens":128},
		"stream":false,
		"temperature":0
	}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := upstreamHeaders.Get("Authorization"); got != "Bearer or-test-key" {
		t.Fatalf("Authorization = %q", got)
	}
	var upstreamModel string
	if err := json.Unmarshal(upstreamBody["model"], &upstreamModel); err != nil {
		t.Fatalf("upstream model JSON: %v", err)
	}
	if upstreamModel != "inclusionai/ring-2.6-1t:free" {
		t.Fatalf("upstream model = %q", upstreamModel)
	}
	for _, key := range []string{"system", "messages", "tools", "tool_choice", "thinking"} {
		if len(upstreamBody[key]) == 0 {
			t.Fatalf("upstream body missing %q: %#v", key, upstreamBody)
		}
	}
	if !strings.Contains(string(upstreamBody["system"]), "cache_control") {
		t.Fatalf("system cache_control was not preserved: %s", upstreamBody["system"])
	}
	if !strings.Contains(string(upstreamBody["messages"]), "tool_result") {
		t.Fatalf("tool_result was not preserved: %s", upstreamBody["messages"])
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("response JSON: %v", err)
	}
	var responseModel string
	if err := json.Unmarshal(body["model"], &responseModel); err != nil {
		t.Fatalf("response model JSON: %v", err)
	}
	if responseModel != "claude-ring-2-6-1t-free" {
		t.Fatalf("response model = %q", responseModel)
	}
	if !strings.Contains(string(body["usage"]), "prompt_tokens_details") {
		t.Fatalf("usage details were not preserved: %s", body["usage"])
	}
	if !strings.Contains(string(body["content"]), `"tool_use"`) {
		t.Fatalf("tool_use response block was not preserved: %s", body["content"])
	}
}

func TestGatewayPassesThroughAnthropicMessagesStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_start\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"inclusionai/ring-2.6-1t:free\",\"content\":[],\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
		_, _ = io.WriteString(w, "event: content_block_start\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		_, _ = io.WriteString(w, "event: content_block_delta\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "event: message_stop\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	defer upstream.Close()

	app := gateway.New(testAnthropicMessagesConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude-ring-2-6-1t-free",
		"max_tokens":256,
		"messages":[{"role":"user","content":"hello"}],
		"stream":true
	}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q", got)
	}
	text := res.Body.String()
	for _, want := range []string{"event: message_start", "event: content_block_delta", `"text":"hello"`, "event: message_stop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stream missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "inclusionai/ring-2.6-1t:free") {
		t.Fatalf("stream leaked upstream model id:\n%s", text)
	}
}

func TestGatewayRejectsUnsupportedProviderProfile(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	provider := cfg.Providers["openrouter"]
	provider.Profile = "custom-api"
	cfg.Providers["openrouter"] = provider

	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
	if !strings.Contains(res.Body.String(), "provider profile is not supported") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayIncludesSafeOpenRouterErrorMetadata(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{
			"error":{
				"message":"Provider returned error",
				"metadata":{
					"raw":"inclusionai/ring-2.6-1t:free is temporarily rate-limited upstream",
					"provider_name":"Novita"
				}
			}
		}`)
	}))
	defer upstream.Close()

	app := gateway.New(testConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"Provider returned error", "Novita", "temporarily rate-limited upstream"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestGatewayFallsBackToNextRouteOnRateLimit(t *testing.T) {
	upstreamModels := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		upstreamModels = append(upstreamModels, body.Model)
		if len(upstreamModels) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"Provider returned error","metadata":{"raw":"first model is rate-limited","provider_name":"FirstProvider"}}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"gen-2","choices":[{"message":{"role":"assistant","content":"fallback ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.Routes["claude-opus-4-7"] = []config.Route{
		{Provider: "openrouter", Model: "first/model"},
		{Provider: "openrouter", Model: "second/model"},
	}
	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if strings.Join(upstreamModels, ",") != "first/model,second/model" {
		t.Fatalf("upstream models = %#v", upstreamModels)
	}
	if !strings.Contains(res.Body.String(), "fallback ok") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayFallsBackToNextRouteOnProviderPaymentRequired(t *testing.T) {
	upstreamModels := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		upstreamModels = append(upstreamModels, body.Model)
		if len(upstreamModels) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = io.WriteString(w, `{"error":{"message":"Provider returned error","metadata":{"raw":"API key USD spend limit exceeded","provider_name":"Venice"}}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"msg_payment_fallback",
			"type":"message",
			"role":"assistant",
			"model":"second/model",
			"content":[{"type":"text","text":"fallback ok"}],
			"stop_reason":"end_turn",
			"stop_sequence":null,
			"usage":{"input_tokens":3,"output_tokens":2}
		}`)
	}))
	defer upstream.Close()

	cfg := testAnthropicMessagesConfig(upstream.URL)
	cfg.Routes["claude-ring-2-6-1t-free"] = []config.Route{
		{Provider: "openrouter", Model: "first/model"},
		{Provider: "openrouter", Model: "second/model"},
	}
	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-ring-2-6-1t-free","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if strings.Join(upstreamModels, ",") != "first/model,second/model" {
		t.Fatalf("upstream models = %#v", upstreamModels)
	}
	if !strings.Contains(res.Body.String(), "fallback ok") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayDoesNotFallbackOnAuthenticationError(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad upstream key"}}`)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.Routes["claude-opus-4-7"] = []config.Route{
		{Provider: "openrouter", Model: "first/model"},
		{Provider: "openrouter", Model: "second/model"},
	}
	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if upstreamCalls != 1 {
		t.Fatalf("upstreamCalls = %d, want 1", upstreamCalls)
	}
}

func TestGatewayExpandsDynamicFreeModelsAndCachesCatalog(t *testing.T) {
	modelCatalogCalls := 0
	messageModels := []string{}
	var firstMessageHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			modelCatalogCalls++
			if got := r.URL.Query().Get("supported_parameters"); got != "tools,tool_choice" {
				t.Fatalf("supported_parameters = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{
				"data": [
					{
						"id": "paid/tools",
						"context_length": 1000000,
						"pricing": {"prompt": "0.01", "completion": "0.01"},
						"supported_parameters": ["tools", "tool_choice"]
					},
					{
						"id": "free/no-tools:free",
						"context_length": 1000000,
						"pricing": {"prompt": "0", "completion": "0"},
						"supported_parameters": ["max_tokens"]
					},
					{
						"id": "free/tools-small:free",
						"context_length": 16000,
						"pricing": {"prompt": "0", "completion": "0"},
						"supported_parameters": ["tools", "tool_choice"]
					},
					{
						"id": "free/tools-large:free",
						"context_length": 200000,
						"pricing": {"prompt": "0", "completion": "0"},
						"supported_parameters": ["tools", "tool_choice", "max_tokens"]
					}
				]
			}`)
		case "/messages":
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode upstream body: %v", err)
			}
			messageModels = append(messageModels, body.Model)
			if firstMessageHeaders == nil {
				firstMessageHeaders = r.Header.Clone()
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-OpenRouter-Cache-Status", "MISS")
			w.Header().Set("X-OpenRouter-Cache-Ttl", "300")
			_, _ = io.WriteString(w, `{
				"id":"msg_dynamic",
				"type":"message",
				"role":"assistant",
				"model":"free/tools-large:free",
				"content":[{"type":"text","text":"dynamic ok"}],
				"stop_reason":"end_turn",
				"stop_sequence":null,
				"usage":{"input_tokens":4,"output_tokens":2}
			}`)
		default:
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	cfg := testAnthropicMessagesConfig(upstream.URL)
	cfg.Routes = map[string][]config.Route{
		"claude-free-agent": {
			{
				Provider: "openrouter",
				Model:    "openrouter/free",
				DynamicFreeModels: config.DynamicFreeModels{
					Enabled:                true,
					RequiredParameters:     []string{"tools", "tool_choice"},
					MinContextLength:       32768,
					MaxModels:              2,
					CatalogCacheTTLSeconds: 900,
					Fallback:               []string{"inclusionai/ring-2.6-1t:free", "openrouter/free"},
				},
				Cache: config.RouteCache{Enabled: true, TTLSeconds: 300},
			},
		},
	}
	app := gateway.New(cfg, upstream.Client())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-free-agent",
			"max_tokens":64,
			"messages":[{"role":"user","content":"hello"}],
			"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{}}}],
			"tool_choice":{"type":"tool","name":"lookup"},
			"stream":false
		}`))
		authHeaders(req)
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i+1, res.Code, res.Body.String())
		}
		if got := res.Header().Get("X-OpenRouter-Cache-Status"); got != "MISS" {
			t.Fatalf("call %d X-OpenRouter-Cache-Status = %q", i+1, got)
		}
		if !strings.Contains(res.Body.String(), `"model":"claude-free-agent"`) {
			t.Fatalf("call %d response did not rewrite model: %s", i+1, res.Body.String())
		}
	}

	if modelCatalogCalls != 1 {
		t.Fatalf("modelCatalogCalls = %d, want 1", modelCatalogCalls)
	}
	if strings.Join(messageModels, ",") != "free/tools-large:free,free/tools-large:free" {
		t.Fatalf("messageModels = %#v", messageModels)
	}
	if got := firstMessageHeaders.Get("X-OpenRouter-Cache"); got != "true" {
		t.Fatalf("X-OpenRouter-Cache = %q", got)
	}
	if got := firstMessageHeaders.Get("X-OpenRouter-Cache-TTL"); got != "300" {
		t.Fatalf("X-OpenRouter-Cache-TTL = %q", got)
	}
}

func TestGatewayDynamicFreeModelsFallsBackWhenCatalogFails(t *testing.T) {
	messageModels := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"catalog unavailable"}}`)
		case "/messages":
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode upstream body: %v", err)
			}
			messageModels = append(messageModels, body.Model)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{
				"id":"msg_fallback",
				"type":"message",
				"role":"assistant",
				"model":"inclusionai/ring-2.6-1t:free",
				"content":[{"type":"text","text":"fallback ok"}],
				"stop_reason":"end_turn",
				"stop_sequence":null,
				"usage":{"input_tokens":4,"output_tokens":2}
			}`)
		default:
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	cfg := testAnthropicMessagesConfig(upstream.URL)
	cfg.Routes = map[string][]config.Route{
		"claude-free-agent": {
			{
				Provider: "openrouter",
				Model:    "openrouter/free",
				DynamicFreeModels: config.DynamicFreeModels{
					Enabled:            true,
					RequiredParameters: []string{"tools", "tool_choice"},
					Fallback:           []string{"inclusionai/ring-2.6-1t:free", "openrouter/free"},
				},
			},
		},
	}
	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude-free-agent",
		"max_tokens":64,
		"messages":[{"role":"user","content":"hello"}],
		"stream":false
	}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if strings.Join(messageModels, ",") != "inclusionai/ring-2.6-1t:free" {
		t.Fatalf("messageModels = %#v", messageModels)
	}
	if !strings.Contains(res.Body.String(), "fallback ok") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayRejectsStreamingWhenProviderDisablesIt(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	provider := cfg.Providers["openrouter"]
	provider.Capabilities.Streaming = false
	cfg.Providers["openrouter"] = provider

	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":true}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
	if !strings.Contains(res.Body.String(), "provider does not support streaming") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayRejectsToolsWhenProviderDisablesThem(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	provider := cfg.Providers["openrouter"]
	provider.Capabilities.Tools = false
	cfg.Providers["openrouter"] = provider

	app := gateway.New(cfg, upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude-opus-4-7",
		"max_tokens":16,
		"messages":[{"role":"user","content":"hello"}],
		"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{}}}]
	}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstreamCalls = %d, want 0", upstreamCalls)
	}
	if !strings.Contains(res.Body.String(), "provider does not support tools") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestGatewayTranslatesOpenRouterStreamToAnthropicSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"gen-stream\",\"choices\":[{\"delta\":{\"content\":\"hel\"},\"finish_reason\":null}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":4}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	app := gateway.New(testConfig(upstream.URL), upstream.Client())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-7","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":true}`))
	authHeaders(req)
	res := httptest.NewRecorder()

	app.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q", got)
	}
	text := res.Body.String()
	for _, want := range []string{"event: message_start", `"id":"gen-stream"`, "event: content_block_delta", `"text":"hel"`, `"text":"lo"`, "event: message_stop"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stream missing %q in:\n%s", want, text)
		}
	}
}
