package claudedesktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyLocalRollsBackWhenProfileWriteFails(t *testing.T) {
	home := t.TempDir()
	paths := PathsForHome(home, "darwin")
	writeJSONForRollbackTest(t, paths.NormalConfigPath, `{"deploymentMode":"1p","normal":true}`)
	writeJSONForRollbackTest(t, paths.ThreePConfigPath, `{"deploymentMode":"1p","threep":true}`)

	targetProfilePath := filepath.Join(paths.ConfigLibraryPath, DefaultProfileID+".json")
	writeJSON := func(path string, obj jsonObject) error {
		if path == targetProfilePath {
			return errors.New("injected profile write failure")
		}
		return writeJSONObject(path, obj)
	}

	_, err := applyLocalWithWriter(ApplyOptions{
		Paths:         paths,
		BaseURL:       "http://127.0.0.1:8787",
		GatewayAPIKey: "secret-client-key",
	}, writeJSON)
	if err == nil {
		t.Fatal("ApplyLocal returned nil error")
	}

	normal := readJSONObjectForRollbackTest(t, paths.NormalConfigPath)
	threep := readJSONObjectForRollbackTest(t, paths.ThreePConfigPath)
	assertJSONStringFieldForRollbackTest(t, normal, "deploymentMode", "1p")
	assertJSONStringFieldForRollbackTest(t, threep, "deploymentMode", "1p")
}

func writeJSONForRollbackTest(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONObjectForRollbackTest(t *testing.T, path string) map[string]json.RawMessage {
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

func assertJSONStringFieldForRollbackTest(t *testing.T, obj map[string]json.RawMessage, key string, want string) {
	t.Helper()

	var got string
	if err := json.Unmarshal(obj[key], &got); err != nil {
		t.Fatalf("%s is not a string: %v", key, err)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
