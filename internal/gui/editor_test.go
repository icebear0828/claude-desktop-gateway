package gui_test

import (
	"os"
	"path/filepath"
	"testing"

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

func writeEditorConfig(t *testing.T, repoRoot string) string {
	t.Helper()
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
			"claude-inclusionai/ring-2.6-1t:free": [
				{
					"provider": "openrouter",
					"model": "inclusionai/ring-2.6-1t:free",
					"displayName": "OpenRouter Ring 2.6 1T Free"
				}
			]
		}
	}`
	if err := os.WriteFile(configPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}
