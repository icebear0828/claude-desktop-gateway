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

func SecretValue(path string, name string) (string, error) {
	key := strings.TrimSpace(name)
	if !validKey(key) {
		return "", fmt.Errorf("invalid env var name %q", name)
	}
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value, nil
	}

	lines, err := readLines(path)
	if err != nil {
		return "", err
	}
	value := ""
	found := false
	for _, line := range lines {
		raw, ok := assignmentValue(line, key)
		if !ok {
			continue
		}
		parsed, err := parseEnvValue(raw)
		if err != nil {
			return "", fmt.Errorf("parse %s: %w", key, err)
		}
		value = parsed
		found = true
	}
	if !found {
		return "", nil
	}
	return value, nil
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
	_, ok := assignmentValue(line, key)
	return ok
}

func assignmentValue(line string, key string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	}
	left, _, ok := strings.Cut(trimmed, "=")
	if !ok {
		return "", false
	}
	if strings.TrimSpace(left) != key {
		return "", false
	}
	_, right, _ := strings.Cut(trimmed, "=")
	return right, true
}

func parseEnvValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "'") {
		parsed, ok := parseSingleQuotedValue(trimmed)
		if !ok {
			return "", errors.New("invalid single-quoted value")
		}
		return parsed, nil
	}
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) && len(trimmed) >= 2 {
		inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, `"`), `"`)
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner, nil
	}
	if before, _, ok := strings.Cut(trimmed, " #"); ok {
		return strings.TrimSpace(before), nil
	}
	return trimmed, nil
}

func parseSingleQuotedValue(value string) (string, bool) {
	var b strings.Builder
	index := 0
	for index < len(value) {
		if value[index] != '\'' {
			if strings.TrimSpace(value[index:]) == "" {
				return b.String(), true
			}
			return "", false
		}
		index++
		for index < len(value) && value[index] != '\'' {
			b.WriteByte(value[index])
			index++
		}
		if index >= len(value) {
			return "", false
		}
		index++
		if index >= len(value) {
			return b.String(), true
		}
		if index+1 < len(value) && value[index] == '\\' && value[index+1] == '\'' {
			b.WriteByte('\'')
			index += 2
		}
	}
	return b.String(), true
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
