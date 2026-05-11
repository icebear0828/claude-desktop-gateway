package gui_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/gui"
	"github.com/local/claude-desktop-gateway/internal/localenv"
)

func TestManagedGatewayStartsWithLocalEnvConfig(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("CLAUDE_GATEWAY_API_KEY", "")

	repoRoot := t.TempDir()
	port := freeLocalPort(t)
	configPath := writeManagedGatewayConfig(t, repoRoot, port)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export OPENROUTER_API_KEY='or-test-key'\nexport CLAUDE_GATEWAY_API_KEY='client-test-key'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		EnvPath:         envPath,
		StateDir:        filepath.Join(repoRoot, ".local-gateway"),
		HealthURL:       fmt.Sprintf("http://127.0.0.1:%d/health", port),
		ExpectedBaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		ManageGateway:   true,
	})
	if err := service.StartGateway(context.Background()); err != nil {
		t.Fatalf("StartGateway returned error: %v", err)
	}
	defer service.StopGateway(context.Background())

	assertEventuallyHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), "", http.StatusOK)

	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/v1/models", port), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer client-test-key")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET /v1/models: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != "claude-test-model" {
		t.Fatalf("models = %#v", body.Data)
	}

	dashboard := service.Dashboard(context.Background())
	if dashboard.Gateway.State != "running" {
		t.Fatalf("gateway state = %q detail=%q", dashboard.Gateway.State, dashboard.Gateway.Detail)
	}
	if !dashboard.Gateway.Managed || dashboard.Gateway.PID == "" {
		t.Fatalf("gateway managed/pid = %v/%q", dashboard.Gateway.Managed, dashboard.Gateway.PID)
	}
}

func TestSavingOpenRouterKeyStartsManagedGateway(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("CLAUDE_GATEWAY_API_KEY", "")

	repoRoot := t.TempDir()
	port := freeLocalPort(t)
	configPath := writeManagedGatewayConfig(t, repoRoot, port)
	envPath := filepath.Join(repoRoot, ".env.local")

	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		EnvPath:         envPath,
		StateDir:        filepath.Join(repoRoot, ".local-gateway"),
		HealthURL:       fmt.Sprintf("http://127.0.0.1:%d/health", port),
		ExpectedBaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		ManageGateway:   true,
	})
	if err := service.StartGateway(context.Background()); err == nil {
		t.Fatal("StartGateway returned nil error without OPENROUTER_API_KEY")
	}

	if _, err := service.SaveSecret(gui.SecretInput{Name: "OPENROUTER_API_KEY", Value: "or-test-key"}); err != nil {
		t.Fatalf("SaveSecret returned error: %v", err)
	}
	defer service.StopGateway(context.Background())

	assertEventuallyHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/health", port), "", http.StatusOK)
	dashboard := service.Dashboard(context.Background())
	if dashboard.Gateway.State != "running" {
		t.Fatalf("gateway state = %q detail=%q", dashboard.Gateway.State, dashboard.Gateway.Detail)
	}
}

func TestRepairRestartsManagedGatewayWithGeneratedClientKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("CLAUDE_GATEWAY_API_KEY", "")

	repoRoot := t.TempDir()
	port := freeLocalPort(t)
	configPath := writeManagedGatewayConfig(t, repoRoot, port)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export OPENROUTER_API_KEY='or-test-key'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	paths := claudedesktop.PathsForHome(t.TempDir(), "windows")
	service := gui.NewService(gui.Options{
		RepoRoot:        repoRoot,
		ConfigPath:      configPath,
		EnvPath:         envPath,
		StateDir:        filepath.Join(repoRoot, ".local-gateway"),
		HealthURL:       fmt.Sprintf("http://127.0.0.1:%d/health", port),
		ExpectedBaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		DesktopPaths:    paths,
		ManageGateway:   true,
	})
	if err := service.StartGateway(context.Background()); err != nil {
		t.Fatalf("StartGateway returned error: %v", err)
	}
	defer service.StopGateway(context.Background())

	assertEventuallyHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/v1/models", port), "", http.StatusOK)

	if _, err := service.ApplyClaudeDesktopConfig(); err != nil {
		t.Fatalf("ApplyClaudeDesktopConfig returned error: %v", err)
	}
	generatedKey, err := localenv.SecretValue(envPath, "CLAUDE_GATEWAY_API_KEY")
	if err != nil {
		t.Fatalf("read generated gateway key: %v", err)
	}
	if !strings.HasPrefix(generatedKey, "cdg_") {
		t.Fatalf("generated key = %q", generatedKey)
	}

	assertEventuallyHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/v1/models", port), "", http.StatusUnauthorized)
	assertEventuallyHTTPStatus(t, fmt.Sprintf("http://127.0.0.1:%d/v1/models", port), generatedKey, http.StatusOK)
}

func writeManagedGatewayConfig(t *testing.T, repoRoot string, port int) string {
	t.Helper()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	body := fmt.Sprintf(`{
		"host": "127.0.0.1",
		"port": %d,
		"gatewayApiKeyEnv": "CLAUDE_GATEWAY_API_KEY",
		"providers": {
			"openrouter": {
				"profile": "anthropic-messages",
				"baseUrl": "https://openrouter.ai/api/v1",
				"apiKeyEnv": "OPENROUTER_API_KEY"
			}
		},
		"routes": {
			"claude-test-model": [
				{"provider": "openrouter", "model": "test/model", "displayName": "Test Model"}
			]
		}
	}`, port)
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func freeLocalPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func assertEventuallyHTTPStatus(t *testing.T, url string, bearerToken string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("create request: %v", err)
		}
		if bearerToken != "" {
			request.Header.Set("Authorization", "Bearer "+bearerToken)
		}
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == want {
				return
			}
			lastErr = fmt.Errorf("status = %d", response.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("GET %s did not return %d: %v", url, want, lastErr)
}
