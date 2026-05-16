package gui_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/codexapp"
	"github.com/local/claude-desktop-gateway/internal/gui"
)

func TestDashboardCombinesConfigGatewayAndClaudeDesktopState(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	body := `{
		"host": "127.0.0.1",
		"port": 8787,
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
			"providers": {
				"openrouter": {
					"profile": "openai-chat",
					"baseUrl": "https://openrouter.ai/api/v1",
					"apiKeyEnv": "OPENROUTER_API_KEY"
				},
				"openrouter-responses": {
					"profile": "responses",
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
				],
				"gpt-5.5": [
					{
						"provider": "openrouter-responses",
						"model": "openrouter/auto",
						"displayName": "Codex Auto"
					}
				]
			}
		}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stateDir := filepath.Join(repoRoot, ".local-gateway")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "gateway.pid"), []byte("12345\n"), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	_, err := claudedesktop.ApplyLocal(claudedesktop.ApplyOptions{
		Paths:         paths,
		BaseURL:       "http://127.0.0.1:8787",
		GatewayAPIKey: "client-test-key",
		ModelIDs:      []string{"claude-inclusionai/ring-2.6-1t:free"},
	})
	if err != nil {
		t.Fatalf("ApplyLocal returned error: %v", err)
	}
	codexPaths := codexapp.PathsForHome(home, "darwin")
	_, err = codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         codexPaths,
		BaseURL:       "http://127.0.0.1:8787/v1",
		GatewayAPIKey: "client-test-key",
		Model:         "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("Codex ApplyLocal returned error: %v", err)
	}

	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		StateDir:        stateDir,
		HealthURL:       "http://gateway.test/health",
		ExpectedBaseURL: "http://127.0.0.1:8787",
		DesktopPaths:    paths,
		CodexPaths:      codexPaths,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				Header:     make(http.Header),
				Request:    request,
			}, nil
		})},
	})
	dashboard := service.Dashboard(context.Background())

	if dashboard.ConfigError != "" {
		t.Fatalf("ConfigError = %q", dashboard.ConfigError)
	}
	if dashboard.ListenURL != "http://127.0.0.1:8787" {
		t.Fatalf("ListenURL = %q", dashboard.ListenURL)
	}
	if dashboard.Gateway.State != "running" {
		t.Fatalf("gateway state = %q", dashboard.Gateway.State)
	}
	if !dashboard.Gateway.Managed || dashboard.Gateway.PID != "12345" {
		t.Fatalf("gateway managed/pid = %v/%q", dashboard.Gateway.Managed, dashboard.Gateway.PID)
	}
	if len(dashboard.Providers) != 2 || dashboard.Providers[0].Name != "openrouter" || dashboard.Providers[1].Name != "openrouter-responses" {
		t.Fatalf("providers = %#v", dashboard.Providers)
	}
	if len(dashboard.Routes) != 2 {
		t.Fatalf("routes = %#v", dashboard.Routes)
	}
	if dashboard.Routes[0].DesktopID != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("DesktopID = %q", dashboard.Routes[0].DesktopID)
	}
	if dashboard.Routes[0].DisplayName != "OpenRouter Ring 2.6 1T Free" {
		t.Fatalf("DisplayName = %q", dashboard.Routes[0].DisplayName)
	}
	if dashboard.ClaudeDesktop.State != "ok" {
		t.Fatalf("ClaudeDesktop state = %q issues=%#v", dashboard.ClaudeDesktop.State, dashboard.ClaudeDesktop.Issues)
	}
	if dashboard.ClaudeDesktop.AppliedID != claudedesktop.DefaultProfileID {
		t.Fatalf("AppliedID = %q", dashboard.ClaudeDesktop.AppliedID)
	}
	if dashboard.CodexApp.State != "ok" {
		t.Fatalf("CodexApp state = %q issues=%#v", dashboard.CodexApp.State, dashboard.CodexApp.Issues)
	}
	if dashboard.CodexApp.ActiveProvider != codexapp.DefaultProviderName {
		t.Fatalf("Codex active provider = %q", dashboard.CodexApp.ActiveProvider)
	}
}

func TestDashboardReportsCodexRouteWithNonResponsesProvider(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	body := `{
		"host": "127.0.0.1",
		"port": 8787,
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"gpt-5.5": [
				{
					"provider": "openrouter",
					"model": "openrouter/auto",
					"displayName": "Codex Auto"
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	home := t.TempDir()
	codexPaths := codexapp.PathsForHome(home, "darwin")
	_, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         codexPaths,
		BaseURL:       "http://127.0.0.1:8787/v1",
		GatewayAPIKey: "client-test-key",
		Model:         "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("Codex ApplyLocal returned error: %v", err)
	}

	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		StateDir:        filepath.Join(repoRoot, ".local-gateway"),
		HealthURL:       "http://gateway.test/health",
		ExpectedBaseURL: "http://127.0.0.1:8787",
		CodexPaths:      codexPaths,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				Header:     make(http.Header),
				Request:    request,
			}, nil
		})},
	})
	dashboard := service.Dashboard(context.Background())

	if dashboard.CodexApp.State != "error" {
		t.Fatalf("CodexApp state = %q issues=%#v", dashboard.CodexApp.State, dashboard.CodexApp.Issues)
	}
	if !hasDesktopIssue(dashboard.CodexApp.Issues, "gateway_codex_route_invalid") {
		t.Fatalf("CodexApp issues = %#v", dashboard.CodexApp.Issues)
	}
}

func TestDashboardUsesConfiguredListenURLForDefaultHealthCheck(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	body := `{
		"host": "127.0.0.1",
		"port": 9898,
		"providers": {
			"openrouter": {
				"profile": "openai-chat",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"claude-test-model": [
				{
					"provider": "openrouter",
					"model": "test/model",
					"displayName": "Test Model"
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
		StateDir:   filepath.Join(repoRoot, ".local-gateway"),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://127.0.0.1:9898/health" {
				t.Fatalf("health URL = %q", request.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				Header:     make(http.Header),
				Request:    request,
			}, nil
		})},
	})
	dashboard := service.Dashboard(context.Background())

	if dashboard.ListenURL != "http://127.0.0.1:9898" {
		t.Fatalf("ListenURL = %q", dashboard.ListenURL)
	}
	if dashboard.Gateway.HealthURL != "http://127.0.0.1:9898/health" {
		t.Fatalf("HealthURL = %q", dashboard.Gateway.HealthURL)
	}
	if dashboard.Gateway.State != "running" {
		t.Fatalf("gateway state = %q", dashboard.Gateway.State)
	}
}

func TestDashboardReturnsActionableErrorsWithoutFailingWholePage(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	if err := os.WriteFile(configPath, []byte("{bad json"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		StateDir:        filepath.Join(repoRoot, ".local-gateway"),
		HealthURL:       "http://gateway.test/health",
		ExpectedBaseURL: "http://127.0.0.1:8787",
		DesktopPaths:    paths,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("offline")
		})},
	})

	dashboard := service.Dashboard(context.Background())

	if dashboard.ConfigError == "" {
		t.Fatal("ConfigError is empty")
	}
	if dashboard.Gateway.State != "stopped" {
		t.Fatalf("gateway state = %q", dashboard.Gateway.State)
	}
	if dashboard.ClaudeDesktop.State != "error" {
		t.Fatalf("ClaudeDesktop state = %q", dashboard.ClaudeDesktop.State)
	}
	if len(dashboard.ClaudeDesktop.Issues) == 0 {
		t.Fatal("ClaudeDesktop issues is empty")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func hasDesktopIssue(issues []gui.DesktopIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
