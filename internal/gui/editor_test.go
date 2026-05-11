package gui_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gui"
)

func TestEditorLoadsEditableConfigAndSecretStatus(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export OPENROUTER_API_KEY='set'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
		EnvPath:    envPath,
	})
	state, err := service.Editor()
	if err != nil {
		t.Fatalf("Editor returned error: %v", err)
	}

	if state.Config.Path != configPath {
		t.Fatalf("config path = %q", state.Config.Path)
	}
	if len(state.Config.Routes) != 1 || state.Config.Routes[0].DesktopID != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("routes = %#v", state.Config.Routes)
	}
	if len(state.Secrets) != 2 {
		t.Fatalf("secrets = %#v", state.Secrets)
	}
	if state.Secrets[0].Name != "OPENROUTER_API_KEY" || !state.Secrets[0].Present || state.Secrets[0].Value != "" {
		t.Fatalf("OPENROUTER secret status = %#v", state.Secrets[0])
	}
}

func TestEditorCreatesDefaultConfigWhenMissing(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	envPath := filepath.Join(repoRoot, ".env.local")
	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
		EnvPath:    envPath,
	})

	state, err := service.Editor()
	if err != nil {
		t.Fatalf("Editor returned error: %v", err)
	}
	if state.Config.Path != configPath {
		t.Fatalf("config path = %q, want %q", state.Config.Path, configPath)
	}
	if len(state.Config.Providers) != 1 || state.Config.Providers[0].Name != "openrouter" {
		t.Fatalf("providers = %#v", state.Config.Providers)
	}
	if len(state.Config.Routes) == 0 || state.Config.Routes[0].DesktopID != "claude-free-agent" {
		t.Fatalf("routes = %#v", state.Config.Routes)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("default config was not written: %v", err)
	}
}

func TestSaveConfigUsesServiceConfigPathAndRejectsUnknownProvider(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
	})
	untrustedPath := filepath.Join(repoRoot, "should-not-be-used.json")

	_, err := service.SaveConfig(config.EditableFile{
		Path: untrustedPath,
		Providers: []config.EditableProvider{{
			Name:      "openrouter",
			BaseURL:   "https://openrouter.ai/api/v1",
			APIKeyEnv: "OPENROUTER_API_KEY",
		}},
		Routes: []config.EditableRoute{{
			DesktopID:     "claude-bad/model",
			Provider:      "missing",
			UpstreamModel: "bad/model",
		}},
	})
	if err == nil {
		t.Fatal("SaveConfig returned nil error for unknown provider")
	}

	_, statErr := os.Stat(untrustedPath)
	if !os.IsNotExist(statErr) {
		t.Fatalf("SaveConfig used untrusted input path, stat err = %v", statErr)
	}
}

func TestSaveSecretAllowsOnlyKnownSecretNames(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	envPath := filepath.Join(repoRoot, ".env.local")
	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
		EnvPath:    envPath,
	})

	_, err := service.SaveSecret(gui.SecretInput{Name: "MALICIOUS_KEY", Value: "secret"})
	if err == nil {
		t.Fatal("SaveSecret returned nil error for disallowed key")
	}

	state, err := service.SaveSecret(gui.SecretInput{Name: "OPENROUTER_API_KEY", Value: "secret"})
	if err != nil {
		t.Fatalf("SaveSecret returned error: %v", err)
	}
	if !state.Secrets[0].Present {
		t.Fatalf("OPENROUTER secret was not marked present: %#v", state.Secrets)
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if string(data) == "" || state.Secrets[0].Value != "" {
		t.Fatalf("unexpected secret state/data: %#v %q", state.Secrets[0], string(data))
	}
}

func TestDeleteSecretUpdatesStatus(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export OPENROUTER_API_KEY='secret'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	service := gui.NewService(gui.Options{
		RepoRoot:   repoRoot,
		ConfigPath: configPath,
		EnvPath:    envPath,
	})

	state, err := service.DeleteSecret(gui.SecretNameInput{Name: "OPENROUTER_API_KEY"})
	if err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}
	if state.Secrets[0].Present {
		t.Fatalf("OPENROUTER secret still present: %#v", state.Secrets[0])
	}
}

func TestApplyClaudeDesktopConfigRepairsBadActiveWindowsProfile(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export CLAUDE_GATEWAY_API_KEY='client-test-key'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	t.Setenv("CLAUDE_GATEWAY_API_KEY", "")
	t.Setenv("LOCALAPPDATA", "")
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "windows")
	badProfileID := "fec4210e-00b4-481a-a754-e199a203fddb"
	writeDesktopJSON(t, paths.MetaPath, `{
		"appliedId": "fec4210e-00b4-481a-a754-e199a203fddb",
		"entries": [{"id": "fec4210e-00b4-481a-a754-e199a203fddb", "name": "Bad Manual Profile"}]
	}`)
	writeDesktopJSON(t, filepath.Join(paths.ConfigLibraryPath, badProfileID+".json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "http://127.0.0.1:8087/",
		"inferenceGatewayApiKey": "old-key",
		"inferenceGatewayAuthScheme": "",
		"inferenceModels": "claude-free-auto"
	}`)

	service := gui.NewService(gui.Options{
		RepoRoot:     repoRoot,
		ConfigPath:   configPath,
		EnvPath:      envPath,
		DesktopPaths: paths,
	})
	result, err := service.ApplyClaudeDesktopConfig()
	if err != nil {
		t.Fatalf("ApplyClaudeDesktopConfig returned error: %v", err)
	}

	if result.ProfileID != claudedesktop.DefaultProfileID {
		t.Fatalf("ProfileID = %q", result.ProfileID)
	}
	if result.BaseURL != "http://127.0.0.1:8787" {
		t.Fatalf("BaseURL = %q", result.BaseURL)
	}
	if len(result.ModelIDs) != 1 || result.ModelIDs[0] != "claude-inclusionai/ring-2.6-1t:free" {
		t.Fatalf("ModelIDs = %#v", result.ModelIDs)
	}

	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: "http://127.0.0.1:8787",
	})
	if err != nil {
		t.Fatalf("Diagnose returned error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("issues after repair = %#v", report.Issues)
	}
	if report.AppliedID != claudedesktop.DefaultProfileID {
		t.Fatalf("AppliedID = %q", report.AppliedID)
	}
}

func TestApplyClaudeDesktopConfigGeneratesMissingGatewayKey(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfig(t, repoRoot)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export OPENROUTER_API_KEY='openrouter-test-key'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}
	t.Setenv("CLAUDE_GATEWAY_API_KEY", "")
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "windows")
	service := gui.NewService(gui.Options{
		RepoRoot:     repoRoot,
		ConfigPath:   configPath,
		EnvPath:      envPath,
		DesktopPaths: paths,
	})

	result, err := service.ApplyClaudeDesktopConfig()
	if err != nil {
		t.Fatalf("ApplyClaudeDesktopConfig returned error: %v", err)
	}

	envData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(envData), "CLAUDE_GATEWAY_API_KEY=") {
		t.Fatalf("generated gateway key was not saved:\n%s", string(envData))
	}

	profile := readProfileJSON(t, result.ProfilePath)
	var profileKey string
	if err := json.Unmarshal(profile["inferenceGatewayApiKey"], &profileKey); err != nil {
		t.Fatalf("profile gateway key is not a string: %v", err)
	}
	if strings.TrimSpace(profileKey) == "" {
		t.Fatal("profile gateway key is empty")
	}

	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: "http://127.0.0.1:8787",
	})
	if err != nil {
		t.Fatalf("Diagnose returned error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("issues after repair = %#v", report.Issues)
	}
}

func TestApplyClaudeDesktopConfigUsesConfiguredListenURL(t *testing.T) {
	repoRoot := t.TempDir()
	configPath := writeEditorConfigWithPort(t, repoRoot, 9898)
	envPath := filepath.Join(repoRoot, ".env.local")
	if err := os.WriteFile(envPath, []byte("export CLAUDE_GATEWAY_API_KEY='client-test-key'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "windows")
	service := gui.NewService(gui.Options{
		RepoRoot:     repoRoot,
		ConfigPath:   configPath,
		EnvPath:      envPath,
		DesktopPaths: paths,
	})

	result, err := service.ApplyClaudeDesktopConfig()
	if err != nil {
		t.Fatalf("ApplyClaudeDesktopConfig returned error: %v", err)
	}
	if result.BaseURL != "http://127.0.0.1:9898" {
		t.Fatalf("BaseURL = %q", result.BaseURL)
	}
}

func writeEditorConfig(t *testing.T, repoRoot string) string {
	t.Helper()
	return writeEditorConfigWithPort(t, repoRoot, 8787)
}

func writeEditorConfigWithPort(t *testing.T, repoRoot string, port int) string {
	t.Helper()
	configPath := filepath.Join(repoRoot, "gateway.local.json")
	body := fmt.Sprintf(`{
		"host": "127.0.0.1",
		"port": %d,
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
	}`, port)
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func writeDesktopJSON(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readProfileJSON(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	var profile map[string]json.RawMessage
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	return profile
}
