package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
)

func TestModelIDsForApplyUsesGatewayConfigRoutes(t *testing.T) {
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

	modelIDs, err := modelIDsForApply("", map[string]string{
		"CLAUDE_GATEWAY_CONFIG":  path,
		"OPENROUTER_API_KEY":     "or-env-key",
		"CLAUDE_GATEWAY_API_KEY": "client-env-key",
	})
	if err != nil {
		t.Fatalf("modelIDsForApply returned error: %v", err)
	}

	want := []string{"claude-inclusionai/ring-2.6-1t:free"}
	if !reflect.DeepEqual(modelIDs, want) {
		t.Fatalf("modelIDs = %#v, want %#v", modelIDs, want)
	}
}

func TestModelIDsForApplyKeepsExplicitModels(t *testing.T) {
	modelIDs, err := modelIDsForApply("claude-a, claude-b", map[string]string{
		"CLAUDE_GATEWAY_CONFIG": "/does/not/exist.json",
	})
	if err != nil {
		t.Fatalf("modelIDsForApply returned error: %v", err)
	}

	want := []string{"claude-a", "claude-b"}
	if !reflect.DeepEqual(modelIDs, want) {
		t.Fatalf("modelIDs = %#v, want %#v", modelIDs, want)
	}
}

func TestModelIDsForApplyFailsForInvalidExplicitGatewayConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.local.json")
	body := `{
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

	_, err := modelIDsForApply("", map[string]string{"CLAUDE_GATEWAY_CONFIG": path})
	if err == nil {
		t.Fatal("modelIDsForApply returned nil error")
	}
	if !strings.Contains(err.Error(), "load gateway config") {
		t.Fatalf("error = %v", err)
	}
}

func TestModelIDsForApplyFallsBackToDefaultModelsWithoutConfig(t *testing.T) {
	modelIDs, err := modelIDsForApply("", map[string]string{})
	if err != nil {
		t.Fatalf("modelIDsForApply returned error: %v", err)
	}

	if !reflect.DeepEqual(modelIDs, claudedesktop.DefaultModelIDs()) {
		t.Fatalf("modelIDs = %#v, want defaults", modelIDs)
	}
}
