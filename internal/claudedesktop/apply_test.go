package claudedesktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
)

func TestApplyLocalWritesFixedActiveProfileAndMeta(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"1p","normal":true}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"1p","threep":true}`)
	writeJSON(t, paths.MetaPath, `{
		"appliedId": "old-profile",
		"entries": [{"id": "old-profile", "name": "Old"}]
	}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "old-profile.json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "http://192.168.10.6:8787",
		"inferenceGatewayApiKey": "old-key",
		"inferenceGatewayAuthScheme": "bearer",
		"deploymentOrganizationUuid": "088BFB70-3E7B-4E9C-BAFB-AEC3DC0DA89A",
		"disableEssentialTelemetry": "true"
	}`)

	result, err := claudedesktop.ApplyLocal(claudedesktop.ApplyOptions{
		Paths:         paths,
		BaseURL:       "http://127.0.0.1:8787",
		GatewayAPIKey: "secret-client-key",
		ModelIDs:      []string{"claude-sonnet-4.6", "claude-haiku-4.5"},
	})
	if err != nil {
		t.Fatalf("ApplyLocal returned error: %v", err)
	}

	if result.ProfileID != claudedesktop.DefaultProfileID {
		t.Fatalf("ProfileID = %q", result.ProfileID)
	}

	normal := readJSONObject(t, paths.NormalConfigPath)
	threep := readJSONObject(t, paths.ThreePConfigPath)
	meta := readJSONObject(t, paths.MetaPath)
	profile := readJSONObject(t, filepath.Join(paths.ConfigLibraryPath, claudedesktop.DefaultProfileID+".json"))

	assertJSONField(t, normal, "deploymentMode", "3p")
	assertJSONField(t, threep, "deploymentMode", "3p")
	assertJSONField(t, profile, "inferenceProvider", "gateway")
	assertJSONField(t, profile, "inferenceGatewayBaseUrl", "http://127.0.0.1:8787")
	assertJSONField(t, profile, "inferenceGatewayApiKey", "secret-client-key")
	assertJSONField(t, profile, "inferenceGatewayAuthScheme", "bearer")
	assertJSONField(t, profile, "deploymentOrganizationUuid", "088BFB70-3E7B-4E9C-BAFB-AEC3DC0DA89A")
	assertJSONField(t, profile, "disableEssentialTelemetry", "true")
	assertJSONField(t, meta, "appliedId", claudedesktop.DefaultProfileID)

	var modelsString string
	if err := json.Unmarshal(profile["inferenceModels"], &modelsString); err != nil {
		t.Fatalf("inferenceModels is not a JSON string: %v", err)
	}
	if modelsString != `["claude-sonnet-4.6","claude-haiku-4.5"]` {
		t.Fatalf("inferenceModels = %q", modelsString)
	}

	output := claudedesktop.FormatApplyResult(result)
	if strings.Contains(output, "secret-client-key") {
		t.Fatalf("apply output leaked gateway key: %s", output)
	}
}

func TestApplyLocalRejectsLANHTTP(t *testing.T) {
	_, err := claudedesktop.ApplyLocal(claudedesktop.ApplyOptions{
		Paths:         claudedesktop.PathsForHome(t.TempDir(), "darwin"),
		BaseURL:       "http://192.168.10.6:8787",
		GatewayAPIKey: "secret-client-key",
	})
	if err == nil {
		t.Fatal("ApplyLocal returned nil error for LAN HTTP")
	}
	if !strings.Contains(err.Error(), "http only for loopback") {
		t.Fatalf("error = %v", err)
	}
}

func readJSONObject(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return obj
}

func assertJSONField(t *testing.T, obj map[string]json.RawMessage, key string, want string) {
	t.Helper()

	var got string
	if err := json.Unmarshal(obj[key], &got); err != nil {
		t.Fatalf("%s is not a string: %v", key, err)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
