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
