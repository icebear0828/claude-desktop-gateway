package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/config"
)

func TestLoadFromEnvDefaultsOpenRouterRoutesToRingFree(t *testing.T) {
	cfg, err := config.LoadFromEnv(map[string]string{
		"OPENROUTER_API_KEY":     "or-test-key",
		"CLAUDE_GATEWAY_API_KEY": "client-test-key",
	})
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 8787 {
		t.Fatalf("Port = %d, want 8787", cfg.Port)
	}
	provider := cfg.Providers[config.DefaultOpenRouterName]
	if provider.Profile != "anthropic-messages" {
		t.Fatalf("provider.Profile = %q, want anthropic-messages", provider.Profile)
	}
	if !provider.Capabilities.Streaming || !provider.Capabilities.Tools {
		t.Fatalf("default capabilities = %#v", provider.Capabilities)
	}

	for _, alias := range []string{"claude-ring-2-6-1t-free", "claude-opus-4-7", "claude-opus-4.7", "claude-sonnet-4-6", "claude-sonnet-4.6", "claude-haiku-4-5", "claude-haiku-4.5"} {
		route, ok := cfg.ResolveRoute(alias)
		if !ok {
			t.Fatalf("ResolveRoute(%q) returned false", alias)
		}
		if route.Provider != "openrouter" {
			t.Fatalf("route.Provider = %q, want openrouter", route.Provider)
		}
		if route.Model != config.DefaultAliasModel {
			t.Fatalf("route.Model = %q, want %q", route.Model, config.DefaultAliasModel)
		}
	}
}

func TestLoadFromFileParsesProviderCapabilities(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY",
				"capabilities": {
					"streaming": false,
					"tools": false,
					"jsonMode": true
				}
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFromFile(path, map[string]string{
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}

	capabilities := cfg.Providers["openrouter"].Capabilities
	if capabilities.Streaming {
		t.Fatalf("Streaming = true, want false")
	}
	if capabilities.Tools {
		t.Fatalf("Tools = true, want false")
	}
	if !capabilities.JSONMode {
		t.Fatalf("JSONMode = false, want true")
	}
}

func TestLoadFromEnvParsesAliasOverrides(t *testing.T) {
	cfg, err := config.LoadFromEnv(map[string]string{
		"OPENROUTER_API_KEY":   "or-test-key",
		"CLAUDE_MODEL_ALIASES": "claude-haiku-4-5=provider/free-model,claude-sonnet-4-6=provider/sonnet-free",
	})
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	haiku, ok := cfg.ResolveRoute("claude-haiku-4-5")
	if !ok {
		t.Fatal("ResolveRoute returned false")
	}
	if haiku.Model != "provider/free-model" {
		t.Fatalf("haiku.Model = %q", haiku.Model)
	}

	direct, ok := cfg.ResolveRoute("openai/gpt-4o-mini")
	if !ok {
		t.Fatal("ResolveRoute for direct model returned false")
	}
	if direct.Model != "openai/gpt-4o-mini" {
		t.Fatalf("direct.Model = %q", direct.Model)
	}
}

func TestLoadFromEnvRequiresOpenRouterKey(t *testing.T) {
	_, err := config.LoadFromEnv(map[string]string{})
	if err == nil {
		t.Fatal("LoadFromEnv returned nil error without OPENROUTER_API_KEY")
	}
}

func TestLoadFromFileRejectsInlineAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"host": "127.0.0.1",
		"port": 9797,
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKey": "or-test-key",
				"referrer": "https://example.test",
				"title": "Claude Gateway Test"
			}
		},
		"routes": {
			"claude-haiku-4-5": [
				{"provider": "openrouter", "model": "inclusionai/ring-2.6-1t:free"}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadFromFile(path, map[string]string{})
	if err == nil {
		t.Fatal("LoadFromFile returned nil error for inline apiKey")
	}
	if !strings.Contains(err.Error(), "apiKey is not allowed") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromFileRejectsInlineGatewayAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKey": "local-client-key",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadFromFile(path, map[string]string{"OPENROUTER_API_KEY": "or-env-key"})
	if err == nil {
		t.Fatal("LoadFromFile returned nil error for inline gatewayApiKey")
	}
	if !strings.Contains(err.Error(), "gatewayApiKey is not allowed") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromFileSupportsAPIKeyEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFromFile(path, map[string]string{
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}
	if cfg.GatewayAPIKey != "client-env-key" {
		t.Fatalf("GatewayAPIKey = %q", cfg.GatewayAPIKey)
	}

	route, ok := cfg.ResolveRoute("claude-opus-4-7")
	if !ok {
		t.Fatal("ResolveRoute returned false")
	}
	provider, ok := cfg.ProviderFor(route)
	if !ok {
		t.Fatal("ProviderFor returned false")
	}
	if provider.APIKey != "or-env-key" {
		t.Fatalf("provider.APIKey = %q", provider.APIKey)
	}
	if provider.APIKeyEnv != "OPENROUTER_API_KEY" {
		t.Fatalf("provider.APIKeyEnv = %q", provider.APIKeyEnv)
	}
}

func TestLoadFromFileBuildsDesktopModelsWithDisplayNames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"claude-inclusionai/ring-2.6-1t:free": [
				{
					"provider": "openrouter",
					"model": "inclusionai/ring-2.6-1t:free",
					"displayName": "OpenRouter Ring 2.6 1T Free"
				}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFromFile(path, map[string]string{
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}

	models := cfg.DesktopModels()
	if len(models) != 1 {
		t.Fatalf("DesktopModels length = %d, want 1: %#v", len(models), models)
	}
	model := models[0]
	if model.ID != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("model.ID = %q", model.ID)
	}
	if model.DisplayName != "OpenRouter Ring 2.6 1T Free" {
		t.Fatalf("model.DisplayName = %q", model.DisplayName)
	}
	if model.Provider != "openrouter" {
		t.Fatalf("model.Provider = %q", model.Provider)
	}
	if model.UpstreamModel != "inclusionai/ring-2.6-1t:free" {
		t.Fatalf("model.UpstreamModel = %q", model.UpstreamModel)
	}
	if strings.Join(cfg.DesktopModelIDs(), ",") != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("DesktopModelIDs = %#v", cfg.DesktopModelIDs())
	}
}

func TestLoadFromFileParsesDynamicFreeModelRouteAndCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "anthropic-messages",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"claude-free-agent": [
				{
					"provider": "openrouter",
					"model": "openrouter/free",
					"displayName": "OpenRouter Free Agent Auto",
					"dynamicFreeModels": {
						"enabled": true,
						"requiredParameters": ["tools", "tool_choice"],
						"minContextLength": 32768,
						"maxModels": 4,
						"catalogCacheTTLSeconds": 900,
						"fallback": [
							"inclusionai/ring-2.6-1t:free",
							"qwen/qwen3-coder:free",
							"openrouter/free"
						]
					},
					"cache": {
						"enabled": true,
						"ttlSeconds": 300
					}
				}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFromFile(path, map[string]string{
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}

	routes, ok := cfg.ResolveRoutes("claude-free-agent")
	if !ok || len(routes) != 1 {
		t.Fatalf("ResolveRoutes returned %v, %#v", ok, routes)
	}
	route := routes[0]
	if !route.DynamicFreeModels.Enabled {
		t.Fatalf("DynamicFreeModels.Enabled = false")
	}
	if strings.Join(route.DynamicFreeModels.RequiredParameters, ",") != "tools,tool_choice" {
		t.Fatalf("RequiredParameters = %#v", route.DynamicFreeModels.RequiredParameters)
	}
	if route.DynamicFreeModels.MinContextLength != 32768 {
		t.Fatalf("MinContextLength = %d", route.DynamicFreeModels.MinContextLength)
	}
	if route.DynamicFreeModels.MaxModels != 4 {
		t.Fatalf("MaxModels = %d", route.DynamicFreeModels.MaxModels)
	}
	if route.DynamicFreeModels.CatalogCacheTTLSeconds != 900 {
		t.Fatalf("CatalogCacheTTLSeconds = %d", route.DynamicFreeModels.CatalogCacheTTLSeconds)
	}
	if strings.Join(route.DynamicFreeModels.Fallback, ",") != "inclusionai/ring-2.6-1t:free,qwen/qwen3-coder:free,openrouter/free" {
		t.Fatalf("Fallback = %#v", route.DynamicFreeModels.Fallback)
	}
	if !route.Cache.Enabled || route.Cache.TTLSeconds != 300 {
		t.Fatalf("Cache = %#v", route.Cache)
	}
}

func TestInspectFileBuildsRedactedSortedSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"host": "127.0.0.1",
		"port": 8787,
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"zeta": {
				"profile": "openai-chat",
				"baseUrl": "https://zeta.example/v1/",
				"apiKeyEnv": "ZETA_API_KEY",
				"capabilities": {
					"streaming": false,
					"tools": true,
					"jsonMode": true
				}
			},
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"claude-zeta/model:free": [
				{
					"provider": "zeta",
					"model": "zeta/model:free",
					"displayName": "Zeta Free"
				}
			],
			"openrouter/model": [
				{
					"provider": "openrouter",
					"model": "openrouter/model"
				}
			]
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	summary, err := config.InspectFile(path)
	if err != nil {
		t.Fatalf("InspectFile returned error: %v", err)
	}

	if summary.Path != path {
		t.Fatalf("summary.Path = %q, want %q", summary.Path, path)
	}
	if summary.Host != "127.0.0.1" || summary.Port != 8787 {
		t.Fatalf("summary address = %s:%d", summary.Host, summary.Port)
	}
	if summary.GatewayAPIKeyEnv != "CLAUDE_GATEWAY_API_KEY" {
		t.Fatalf("GatewayAPIKeyEnv = %q", summary.GatewayAPIKeyEnv)
	}
	if len(summary.Providers) != 2 {
		t.Fatalf("providers length = %d: %#v", len(summary.Providers), summary.Providers)
	}
	if summary.Providers[0].Name != "openrouter" || summary.Providers[1].Name != "zeta" {
		t.Fatalf("providers not sorted by name: %#v", summary.Providers)
	}
	if summary.Providers[1].BaseURL != "https://zeta.example/v1" {
		t.Fatalf("zeta BaseURL = %q", summary.Providers[1].BaseURL)
	}
	if summary.Providers[1].APIKeyEnv != "ZETA_API_KEY" {
		t.Fatalf("zeta APIKeyEnv = %q", summary.Providers[1].APIKeyEnv)
	}
	if summary.Providers[1].Capabilities.Streaming {
		t.Fatalf("zeta Streaming = true, want false")
	}
	if !summary.Providers[1].Capabilities.Tools || !summary.Providers[1].Capabilities.JSONMode {
		t.Fatalf("zeta capabilities = %#v", summary.Providers[1].Capabilities)
	}
	if len(summary.Routes) != 2 {
		t.Fatalf("routes length = %d: %#v", len(summary.Routes), summary.Routes)
	}
	if summary.Routes[0].DesktopID != "claude-openrouter/model" {
		t.Fatalf("route 0 DesktopID = %q", summary.Routes[0].DesktopID)
	}
	if summary.Routes[0].DisplayName != "claude-openrouter/model" {
		t.Fatalf("route 0 DisplayName = %q", summary.Routes[0].DisplayName)
	}
	if summary.Routes[1].DisplayName != "Zeta Free" {
		t.Fatalf("route 1 DisplayName = %q", summary.Routes[1].DisplayName)
	}
}

func TestInspectFileRejectsInlineSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
		"gatewayApiKey": "client-secret",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.InspectFile(path)
	if err == nil {
		t.Fatal("InspectFile returned nil error for inline gatewayApiKey")
	}
	if !strings.Contains(err.Error(), "gatewayApiKey is not allowed") {
		t.Fatalf("error = %v", err)
	}
}

func TestDefaultDesktopModelIDPrefixesUpstreamModelWithClaude(t *testing.T) {
	tests := map[string]string{
		"inclusionai/ring-2.6-1t:free": "claude-inclusionai/ring-2.6-1t:free",
		"claude-sonnet-4.6":            "claude-sonnet-4.6",
		"anthropic/claude-opus-4-7":    "anthropic/claude-opus-4-7",
	}
	for upstream, want := range tests {
		if got := config.DefaultDesktopModelID(upstream); got != want {
			t.Fatalf("DefaultDesktopModelID(%q) = %q, want %q", upstream, got, want)
		}
	}
}

func TestLoadFromEnvAllowsLoopbackWithoutGatewayKey(t *testing.T) {
	_, err := config.LoadFromEnv(map[string]string{
		"OPENROUTER_API_KEY": "or-test-key",
		"HOST":               "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
}

func TestLoadFromEnvRequiresGatewayKeyForLANBind(t *testing.T) {
	_, err := config.LoadFromEnv(map[string]string{
		"OPENROUTER_API_KEY": "or-test-key",
		"HOST":               "0.0.0.0",
	})
	if err == nil {
		t.Fatal("LoadFromEnv returned nil error for LAN bind without gateway key")
	}
	if !strings.Contains(err.Error(), "gateway API key is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromEnvParsesTLSFiles(t *testing.T) {
	cfg, err := config.LoadFromEnv(map[string]string{
		"OPENROUTER_API_KEY":     "or-test-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
		"HOST":                   "0.0.0.0",
		"TLS_CERT_FILE":          "certs/gateway.crt",
		"TLS_KEY_FILE":           "certs/gateway.key",
	})
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}
	if !cfg.TLSEnabled() {
		t.Fatal("TLSEnabled returned false")
	}
	if cfg.Scheme() != "https" {
		t.Fatalf("Scheme = %q", cfg.Scheme())
	}
}

func TestLoadFromFileRequiresGatewayKeyForLANBind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.lan.json")
	body := `{
		"host": "0.0.0.0",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadFromFile(path, map[string]string{"OPENROUTER_API_KEY": "or-env-key"})
	if err == nil {
		t.Fatal("LoadFromFile returned nil error for LAN bind without gateway key")
	}
	if !strings.Contains(err.Error(), "gateway API key is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadFromFileParsesTLSFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.lan.json")
	body := `{
		"host": "0.0.0.0",
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"tlsCertFile": "certs/gateway.crt",
		"tlsKeyFile": "certs/gateway.key",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		}
	}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFromFile(path, map[string]string{
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}
	if cfg.TLSCertFile != "certs/gateway.crt" || cfg.TLSKeyFile != "certs/gateway.key" {
		t.Fatalf("TLS files = %q / %q", cfg.TLSCertFile, cfg.TLSKeyFile)
	}
	if cfg.Scheme() != "https" {
		t.Fatalf("Scheme = %q", cfg.Scheme())
	}
}
