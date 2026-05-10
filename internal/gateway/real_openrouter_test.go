package gateway_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gateway"
)

func TestRealOpenRouterCompletesThreeSequentialCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}

	env := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	env["CLAUDE_GATEWAY_API_KEY"] = "real-gateway-test"
	env["CLAUDE_GATEWAY_DEFAULT_MODEL"] = config.DefaultAliasModel
	env["OPENROUTER_TITLE"] = "Claude Desktop Gateway Go Real Test"

	cfg, err := config.LoadFromEnv(env)
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	app := gateway.New(cfg, http.DefaultClient)

	for i := 1; i <= 3; i++ {
		expected := "gateway-ok-" + strconv.Itoa(i)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-opus-4-7",
			"max_tokens":96,
			"messages":[{"role":"user","content":"Reply with exactly: `+expected+`"}],
			"stream":false,
			"temperature":0
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer real-gateway-test")
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
		var body struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d response JSON: %v", i, err)
		}
		text := ""
		for _, block := range body.Content {
			text += block.Text
		}
		if !strings.Contains(text, expected) {
			t.Fatalf("call %d text = %q, want %q", i, text, expected)
		}
		if body.Usage.InputTokens <= 0 || body.Usage.OutputTokens <= 0 {
			t.Fatalf("call %d usage = %#v", i, body.Usage)
		}
	}
}

func TestRealOpenRouterAnthropicMessagesCompletesThreeSequentialCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}

	cfg := config.Config{
		Host:          "127.0.0.1",
		Port:          8787,
		GatewayAPIKey: "real-gateway-test",
		Providers: map[string]config.Provider{
			"openrouter": {
				Profile:      "anthropic-messages",
				BaseURL:      config.DefaultOpenRouterURL,
				APIKey:       strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
				Title:        "Claude Desktop Gateway Go Real Anthropic Messages Test",
				Capabilities: config.DefaultProviderCapabilities(),
			},
		},
		Routes: map[string][]config.Route{
			"claude-ring-2-6-1t-free": {
				{Provider: "openrouter", Model: config.DefaultAliasModel},
			},
		},
	}
	app := gateway.New(cfg, http.DefaultClient)

	for i := 1; i <= 3; i++ {
		expected := "gateway-anthropic-ok-" + strconv.Itoa(i)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-ring-2-6-1t-free",
			"max_tokens":256,
			"messages":[{"role":"user","content":"Reply with exactly: `+expected+`"}],
			"stream":false,
			"temperature":0
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer real-gateway-test")
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
		var body struct {
			Model   string `json:"model"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d response JSON: %v", i, err)
		}
		if body.Model != "claude-ring-2-6-1t-free" {
			t.Fatalf("call %d model = %q", i, body.Model)
		}
		text := ""
		for _, block := range body.Content {
			text += block.Text
		}
		if !strings.Contains(text, expected) {
			t.Fatalf("call %d text = %q, want %q", i, text, expected)
		}
		if body.Usage.InputTokens <= 0 || body.Usage.OutputTokens <= 0 {
			t.Fatalf("call %d usage = %#v", i, body.Usage)
		}
	}
}

func TestRealOpenRouterDynamicFreeModelsCompletesThreeSequentialCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}

	cfg := realDynamicFreeModelsConfig()
	app := gateway.New(cfg, http.DefaultClient)

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-free-agent",
			"max_tokens":128,
			"messages":[{"role":"user","content":"Reply with one short sentence that includes the word gateway."}],
			"stream":false,
			"temperature":0
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer real-gateway-test")
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
		var body struct {
			Model   string `json:"model"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d response JSON: %v", i, err)
		}
		if body.Model != "claude-free-agent" {
			t.Fatalf("call %d model = %q", i, body.Model)
		}
		text := ""
		for _, block := range body.Content {
			text += block.Text
		}
		if strings.TrimSpace(text) == "" {
			t.Fatalf("call %d empty content: %s", i, res.Body.String())
		}
	}
}

func TestRealOpenRouterAnthropicMessagesStreamsThreeSequentialCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}

	cfg := realAnthropicMessagesConfig()
	app := gateway.New(cfg, http.DefaultClient)

	for i := 1; i <= 3; i++ {
		expected := "gateway-anthropic-stream-ok-" + strconv.Itoa(i)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-ring-2-6-1t-free",
			"max_tokens":256,
			"messages":[{"role":"user","content":"Reply with exactly: `+expected+`"}],
			"stream":true,
			"temperature":0
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer real-gateway-test")
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
		text := res.Body.String()
		if !strings.Contains(text, "event: content_block_delta") || !strings.Contains(streamTextDeltas(t, text), expected) || !strings.Contains(text, "event: message_stop") {
			t.Fatalf("call %d stream = %s", i, text)
		}
		if strings.Contains(text, config.DefaultAliasModel) {
			t.Fatalf("call %d stream leaked upstream model id: %s", i, text)
		}
	}
}

func TestRealOpenRouterAnthropicMessagesToolsThreeSequentialCalls(t *testing.T) {
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Skip("OPENROUTER_API_KEY is not set")
	}

	cfg := realAnthropicMessagesConfig()
	app := gateway.New(cfg, http.DefaultClient)

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
			"model":"claude-ring-2-6-1t-free",
			"max_tokens":256,
			"messages":[{"role":"user","content":"Use the lookup tool with key alpha. Do not answer directly."}],
			"tools":[{"name":"lookup","description":"Lookup a value by key.","input_schema":{"type":"object","properties":{"key":{"type":"string"}},"required":["key"]}}],
			"tool_choice":{"type":"tool","name":"lookup"},
			"stream":false,
			"temperature":0
		}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer real-gateway-test")
		res := httptest.NewRecorder()

		app.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("call %d status = %d, body = %s", i, res.Code, res.Body.String())
		}
		var body struct {
			Model      string `json:"model"`
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type  string         `json:"type"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d response JSON: %v", i, err)
		}
		if body.Model != "claude-ring-2-6-1t-free" {
			t.Fatalf("call %d model = %q", i, body.Model)
		}
		if body.StopReason != "tool_use" {
			t.Fatalf("call %d stop_reason = %q, body = %s", i, body.StopReason, res.Body.String())
		}
		foundLookup := false
		for _, block := range body.Content {
			if block.Type == "tool_use" && block.Name == "lookup" && block.Input["key"] == "alpha" {
				foundLookup = true
			}
		}
		if !foundLookup {
			t.Fatalf("call %d missing lookup tool_use: %s", i, res.Body.String())
		}
	}
}

func streamTextDeltas(t *testing.T, stream string) string {
	t.Helper()

	var b strings.Builder
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Delta.Type == "text_delta" {
			b.WriteString(event.Delta.Text)
		}
	}
	return b.String()
}

func realAnthropicMessagesConfig() config.Config {
	return config.Config{
		Host:          "127.0.0.1",
		Port:          8787,
		GatewayAPIKey: "real-gateway-test",
		Providers: map[string]config.Provider{
			"openrouter": {
				Profile:      "anthropic-messages",
				BaseURL:      config.DefaultOpenRouterURL,
				APIKey:       strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
				Title:        "Claude Desktop Gateway Go Real Anthropic Messages Test",
				Capabilities: config.DefaultProviderCapabilities(),
			},
		},
		Routes: map[string][]config.Route{
			"claude-ring-2-6-1t-free": {
				{Provider: "openrouter", Model: config.DefaultAliasModel},
			},
		},
	}
}

func realDynamicFreeModelsConfig() config.Config {
	cfg := realAnthropicMessagesConfig()
	cfg.Routes = map[string][]config.Route{
		"claude-free-agent": {
			{
				Provider: "openrouter",
				Model:    "openrouter/free",
				DynamicFreeModels: config.DynamicFreeModels{
					Enabled:                true,
					RequiredParameters:     []string{"tools", "tool_choice"},
					MinContextLength:       32768,
					MaxModels:              4,
					CatalogCacheTTLSeconds: 900,
					Fallback: []string{
						config.DefaultAliasModel,
						"qwen/qwen3-coder:free",
						"z-ai/glm-4.5-air:free",
						"openrouter/free",
					},
				},
				Cache: config.RouteCache{Enabled: true, TTLSeconds: 300},
			},
		},
	}
	return cfg
}
