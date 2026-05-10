package gui

import (
	"os"
	"path/filepath"
)

func defaultRepoRoot() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	if discovered := discoverRepoRoot(candidates); discovered != "" {
		return discovered
	}
	if configDir := defaultConfigRoot(); configDir != "" {
		return configDir
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "."
}

func defaultConfigRoot() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "claude-desktop-gateway")
}

func discoverRepoRoot(candidates []string) string {
	for _, candidate := range candidates {
		current, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(current)
		if err == nil && !info.IsDir() {
			current = filepath.Dir(current)
		}
		for {
			if hasRepoMarkers(current) {
				return current
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return ""
}

func hasRepoMarkers(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "wails.json")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "gateway.local.json")); err != nil {
		return false
	}
	return true
}
