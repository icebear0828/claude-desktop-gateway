package localenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SecretEnv struct {
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Value   string `json:"value"`
}

func SecretStatus(path string, names []string) ([]SecretEnv, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}

	statuses := make([]SecretEnv, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		present := os.Getenv(trimmed) != ""
		for _, line := range lines {
			if assignsKey(line, trimmed) {
				present = true
				break
			}
		}
		statuses = append(statuses, SecretEnv{Name: trimmed, Present: present})
	}
	return statuses, nil
}

func SaveSecret(path string, name string, value string) error {
	key := strings.TrimSpace(name)
	if !validKey(key) {
		return fmt.Errorf("invalid env var name %q", name)
	}
	if value == "" {
		return errors.New("secret value is required")
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	replacement := "export " + key + "=" + shellSingleQuote(value)
	found := false
	next := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if assignsKey(line, key) {
			if !found {
				next = append(next, replacement)
				found = true
			}
			continue
		}
		next = append(next, line)
	}
	if !found {
		next = append(next, replacement)
	}
	return writeLines(path, next)
}

func DeleteSecret(path string, name string) error {
	key := strings.TrimSpace(name)
	if !validKey(key) {
		return fmt.Errorf("invalid env var name %q", name)
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	next := make([]string, 0, len(lines))
	for _, line := range lines {
		if assignsKey(line, key) {
			continue
		}
		next = append(next, line)
	}
	return writeLines(path, next)
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read env file: %w", err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

func writeLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create env directory: %w", err)
	}
	data := strings.Join(lines, "\n")
	if data != "" {
		data += "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	return nil
}

func assignsKey(line string, key string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return false
	}
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	}
	left, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return false
	}
	return strings.TrimSpace(left) == key
}

func validKey(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' && index > 0 {
			continue
		}
		if char == '_' {
			continue
		}
		return false
	}
	return true
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
