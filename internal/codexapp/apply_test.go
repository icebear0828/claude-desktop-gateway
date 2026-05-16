package codexapp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/codexapp"
)

func TestApplyLocalWritesGatewayProviderAndPreservesUnrelatedConfig(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.4"
model_provider = "openai"

[projects."/Users/c/claude-desktop-gateway"]
trust_level = "trusted"

[model_providers.openai]
name = "OpenAI"
base_url = "https://api.openai.com/v1"
wire_api = "responses"
`)

	result, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         paths,
		BaseURL:       "http://127.0.0.1:8787/v1",
		GatewayAPIKey: "test-key",
		Model:         "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("ApplyLocal returned error: %v", err)
	}

	body := readText(t, paths.ConfigPath)
	for _, want := range []string{
		`model = "gpt-5.5"`,
		`model_provider = "local_gateway"`,
		`[projects."/Users/c/claude-desktop-gateway"]`,
		`trust_level = "trusted"`,
		`[model_providers.openai]`,
		`[model_providers.local_gateway]`,
		`name = "Local Gateway"`,
		`base_url = "http://127.0.0.1:8787/v1"`,
		`wire_api = "responses"`,
		`[model_providers.local_gateway.http_headers]`,
		`Authorization = "Bearer test-key"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("config missing %q:\n%s", want, body)
		}
	}
	if result.ProviderName != codexapp.DefaultProviderName {
		t.Fatalf("ProviderName = %q", result.ProviderName)
	}
	output := codexapp.FormatApplyResult(result)
	if strings.Contains(output, "test-key") {
		t.Fatalf("apply output leaked gateway key: %s", output)
	}
}

func TestApplyLocalReplacesExistingGatewayProviderBlock(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "old-model"
model_provider = "local_gateway"

[model_providers.local_gateway]
name = "Old"
base_url = "http://127.0.0.1:9999/v1"
wire_api = "chat"

[model_providers.local_gateway.http_headers]
Authorization = "Bearer old-key"

[features]
memories = true
`)

	_, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         paths,
		BaseURL:       "http://127.0.0.1:8787/v1",
		GatewayAPIKey: "new-test-key",
		Model:         "gpt-5.5",
	})
	if err != nil {
		t.Fatalf("ApplyLocal returned error: %v", err)
	}

	body := readText(t, paths.ConfigPath)
	if strings.Contains(body, "old-key") || strings.Contains(body, "http://127.0.0.1:9999/v1") || strings.Contains(body, `wire_api = "chat"`) {
		t.Fatalf("stale provider block remained:\n%s", body)
	}
	if !strings.Contains(body, `[features]`) || !strings.Contains(body, `memories = true`) {
		t.Fatalf("unrelated feature config was not preserved:\n%s", body)
	}
}

func TestApplyLocalRejectsLANHTTP(t *testing.T) {
	_, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         codexapp.PathsForHome(t.TempDir(), "darwin"),
		BaseURL:       "http://192.168.10.6:8787/v1",
		GatewayAPIKey: "test-key",
	})
	if err == nil {
		t.Fatal("ApplyLocal returned nil error for LAN HTTP")
	}
	if !strings.Contains(err.Error(), "http only for loopback") {
		t.Fatalf("error = %v", err)
	}
}

func writeText(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
