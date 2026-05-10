package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRepoRootWalksUpFromPackagedAppPath(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "wails.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write wails.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "gateway.local.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write gateway.local.json: %v", err)
	}
	appDir := filepath.Join(repoRoot, "build", "bin", "claude-desktop-gateway.app", "Contents", "MacOS")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("create app dir: %v", err)
	}

	if got := discoverRepoRoot([]string{appDir}); got != repoRoot {
		t.Fatalf("discoverRepoRoot = %q, want %q", got, repoRoot)
	}
}

func TestDefaultRepoRootFallsBackToUserConfigDirWithoutRepoMarkers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Chdir(t.TempDir())

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir returned error: %v", err)
	}

	if got, want := defaultRepoRoot(), filepath.Join(configDir, "claude-desktop-gateway"); got != want {
		t.Fatalf("defaultRepoRoot = %q, want %q", got, want)
	}
}
