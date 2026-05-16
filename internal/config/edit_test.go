package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/config"
)

func TestSaveEditableFileWritesEnvReferencesAndRoutes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")

	summary, err := config.SaveEditableFile(path, config.EditableFile{
		Host:             "127.0.0.1",
		Port:             8787,
		GatewayAPIKeyEnv: "CLAUDE_GATEWAY_API_KEY",
		Providers: []config.EditableProvider{{
			Name:      "openrouter",
			Profile:   "openai-chat",
			BaseURL:   "https://openrouter.ai/api/v1/",
			APIKeyEnv: "OPENROUTER_API_KEY",
			Title:     "Claude Gateway",
		}},
		Routes: []config.EditableRoute{{
			DesktopID:     "claude-inclusionai/ring-2.6-1t:free",
			Provider:      "openrouter",
			UpstreamModel: "inclusionai/ring-2.6-1t:free",
			DisplayName:   "OpenRouter Ring 2.6 1T Free",
		}},
	})
	if err != nil {
		t.Fatalf("SaveEditableFile returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "apiKey\"") || strings.Contains(body, "gatewayApiKey\"") {
		t.Fatalf("saved config contains inline secret fields:\n%s", body)
	}
	if !strings.Contains(body, `"apiKeyEnv": "OPENROUTER_API_KEY"`) {
		t.Fatalf("saved config missing apiKeyEnv:\n%s", body)
	}
	if summary.Providers[0].BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("summary provider base URL = %q", summary.Providers[0].BaseURL)
	}
	if summary.Routes[0].DisplayName != "OpenRouter Ring 2.6 1T Free" {
		t.Fatalf("summary route display name = %q", summary.Routes[0].DisplayName)
	}
}

func TestSaveEditableFileRejectsUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")

	_, err := config.SaveEditableFile(path, config.EditableFile{
		Providers: []config.EditableProvider{{
			Name:      "openrouter",
			BaseURL:   "https://openrouter.ai/api/v1",
			APIKeyEnv: "OPENROUTER_API_KEY",
		}},
		Routes: []config.EditableRoute{{
			DesktopID:     "claude-example/model",
			Provider:      "missing",
			UpstreamModel: "example/model",
		}},
	})
	if err == nil {
		t.Fatal("SaveEditableFile returned nil error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadEditableFileRejectsInlineSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"providers": {
			"openrouter": {
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKey": "secret"
			}
		},
		"routes": {
			"claude-example/model": [
				{"provider": "openrouter", "model": "example/model"}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadEditableFile(path)
	if err == nil {
		t.Fatal("LoadEditableFile returned nil error for inline apiKey")
	}
	if !strings.Contains(err.Error(), "apiKey is not allowed") {
		t.Fatalf("error = %v", err)
	}
}

func TestNewEditableRouteDefaultsDesktopID(t *testing.T) {
	route := config.NewEditableRoute("openrouter", "inclusionai/ring-2.6-1t:free", "")
	if route.DesktopID != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("DesktopID = %q", route.DesktopID)
	}
	if route.DisplayName != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("DisplayName = %q", route.DisplayName)
	}
}

func TestDefaultEditableFileIncludesCodexResponsesRoute(t *testing.T) {
	editable := config.DefaultEditableFile("gateway.local.json")

	providers := map[string]config.EditableProvider{}
	for _, provider := range editable.Providers {
		providers[provider.Name] = provider
	}
	responsesProvider, ok := providers["openrouter-responses"]
	if !ok {
		t.Fatalf("openrouter-responses provider missing: %#v", editable.Providers)
	}
	if responsesProvider.Profile != "responses" {
		t.Fatalf("responses provider profile = %q", responsesProvider.Profile)
	}
	if responsesProvider.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("responses provider apiKeyEnv = %q", responsesProvider.APIKeyEnv)
	}

	foundRoute := false
	for _, route := range editable.Routes {
		if route.DesktopID == "gpt-5.5" {
			foundRoute = true
			if route.Provider != "openrouter-responses" {
				t.Fatalf("gpt-5.5 provider = %q", route.Provider)
			}
			if route.UpstreamModel != "openrouter/auto" {
				t.Fatalf("gpt-5.5 upstream = %q", route.UpstreamModel)
			}
		}
	}
	if !foundRoute {
		t.Fatalf("gpt-5.5 route missing: %#v", editable.Routes)
	}
}
