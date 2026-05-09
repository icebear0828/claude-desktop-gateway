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
