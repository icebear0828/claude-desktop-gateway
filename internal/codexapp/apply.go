package codexapp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ApplyOptions struct {
	Paths         Paths
	ProviderName  string
	ProviderLabel string
	BaseURL       string
	GatewayAPIKey string
	Model         string
}

type ApplyResult struct {
	Paths        Paths
	ProviderName string
	BaseURL      string
	Model        string
}

type textSnapshot struct {
	path    string
	content []byte
	exists  bool
}

type textWriter func(path string, data []byte) error

func ApplyLocal(options ApplyOptions) (ApplyResult, error) {
	return applyLocalWithWriter(options, atomicWriteFile)
}

func applyLocalWithWriter(options ApplyOptions, writeText textWriter) (ApplyResult, error) {
	paths := options.Paths
	if strings.TrimSpace(paths.ConfigPath) == "" {
		var err error
		paths, err = CurrentPlatformPaths()
		if err != nil {
			return ApplyResult{}, err
		}
	}
	providerName := valueOrDefault(options.ProviderName, DefaultProviderName)
	providerLabel := valueOrDefault(options.ProviderLabel, DefaultProviderLabel)
	baseURL := strings.TrimRight(valueOrDefault(options.BaseURL, DefaultBaseURL), "/")
	if err := validateGatewayBaseURL(baseURL); err != nil {
		return ApplyResult{}, err
	}
	gatewayAPIKey := strings.TrimSpace(options.GatewayAPIKey)
	if gatewayAPIKey == "" {
		return ApplyResult{}, errors.New("gateway API key is required")
	}
	model := valueOrDefault(options.Model, DefaultModel)

	result := ApplyResult{
		Paths:        paths,
		ProviderName: providerName,
		BaseURL:      baseURL,
		Model:        model,
	}
	snapshot, err := snapshotTextFile(paths.ConfigPath)
	if err != nil {
		return ApplyResult{}, err
	}
	existing := ""
	if snapshot.exists {
		existing = string(snapshot.content)
	}
	patched := renderPatchedConfig(existing, providerName, providerLabel, baseURL, gatewayAPIKey, model)
	if err := writeText(paths.ConfigPath, []byte(patched)); err != nil {
		if restoreErr := restoreTextSnapshot(snapshot); restoreErr != nil {
			return ApplyResult{}, fmt.Errorf("%w; rollback failed: %v", err, restoreErr)
		}
		return ApplyResult{}, err
	}
	return result, nil
}

func FormatApplyResult(result ApplyResult) string {
	var b strings.Builder
	b.WriteString("Codex App local gateway provider\n")
	writeLine(&b, "provider", result.ProviderName)
	writeLine(&b, "config", result.Paths.ConfigPath)
	writeLine(&b, "base url", result.BaseURL)
	writeLine(&b, "model", result.Model)
	b.WriteString("gateway api key: <redacted>\n")
	return b.String()
}

func renderPatchedConfig(existing string, providerName string, providerLabel string, baseURL string, gatewayAPIKey string, model string) string {
	lines := removeProviderSections(splitLines(existing), providerName)
	lines = setTopLevelString(lines, "model", model)
	lines = setTopLevelString(lines, "model_provider", providerName)
	lines = trimTrailingEmptyLines(lines)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		"[model_providers."+providerName+"]",
		"name = "+quoteString(providerLabel),
		"base_url = "+quoteString(baseURL),
		"wire_api = "+quoteString(DefaultWireAPI),
		"",
		"[model_providers."+providerName+".http_headers]",
		"Authorization = "+quoteString("Bearer "+gatewayAPIKey),
	)
	return strings.Join(lines, "\n") + "\n"
}

func splitLines(body string) []string {
	if body == "" {
		return nil
	}
	lines := strings.Split(body, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func removeProviderSections(lines []string, providerName string) []string {
	targetPrefix := "model_providers." + providerName
	filtered := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		if section, ok := tableSection(line); ok {
			skipping = section == targetPrefix || strings.HasPrefix(section, targetPrefix+".")
		}
		if skipping {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func setTopLevelString(lines []string, key string, value string) []string {
	firstTable := len(lines)
	for index, line := range lines {
		if _, ok := tableSection(line); ok {
			firstTable = index
			break
		}
	}
	replacement := key + " = " + quoteString(value)
	for index := 0; index < firstTable; index++ {
		existingKey, _, ok := parseAssignment(strings.TrimSpace(lines[index]))
		if ok && existingKey == key {
			lines[index] = replacement
			return lines
		}
	}
	next := make([]string, 0, len(lines)+1)
	next = append(next, lines[:firstTable]...)
	next = append(next, replacement)
	next = append(next, lines[firstTable:]...)
	return next
}

func tableSection(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") || !strings.Contains(trimmed, "]") {
		return "", false
	}
	raw := strings.TrimSpace(trimmed[1:strings.Index(trimmed, "]")])
	if normalized, ok := normalizeTablePath(raw); ok {
		return normalized, true
	}
	return raw, true
}

func normalizeTablePath(path string) (string, bool) {
	segments := []string{}
	index := 0
	for {
		index = skipSpaces(path, index)
		if index >= len(path) {
			break
		}

		segment := ""
		switch path[index] {
		case '"':
			value, next, ok := parseQuotedPathSegment(path, index)
			if !ok {
				return "", false
			}
			segment = value
			index = next
		case '\'':
			value, next, ok := parseLiteralPathSegment(path, index)
			if !ok {
				return "", false
			}
			segment = value
			index = next
		default:
			start := index
			for index < len(path) && path[index] != '.' {
				index++
			}
			segment = strings.TrimSpace(path[start:index])
		}

		if segment == "" {
			return "", false
		}
		segments = append(segments, segment)

		index = skipSpaces(path, index)
		if index >= len(path) {
			break
		}
		if path[index] != '.' {
			return "", false
		}
		index++
	}
	if len(segments) == 0 {
		return "", false
	}
	return strings.Join(segments, "."), true
}

func parseQuotedPathSegment(path string, start int) (string, int, bool) {
	escaped := false
	for index := start + 1; index < len(path); index++ {
		switch {
		case escaped:
			escaped = false
		case path[index] == '\\':
			escaped = true
		case path[index] == '"':
			value, err := strconv.Unquote(path[start : index+1])
			if err != nil {
				return "", 0, false
			}
			return value, index + 1, true
		}
	}
	return "", 0, false
}

func parseLiteralPathSegment(path string, start int) (string, int, bool) {
	for index := start + 1; index < len(path); index++ {
		if path[index] == '\'' {
			return path[start+1 : index], index + 1, true
		}
	}
	return "", 0, false
}

func skipSpaces(value string, start int) int {
	for start < len(value) && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	return start
}

func trimTrailingEmptyLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func quoteString(value string) string {
	return strconv.Quote(value)
}

func snapshotTextFile(path string) (textSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return textSnapshot{path: path}, nil
		}
		return textSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	return textSnapshot{path: path, content: append([]byte(nil), data...), exists: true}, nil
}

func restoreTextSnapshot(snapshot textSnapshot) error {
	if snapshot.exists {
		return atomicWriteFile(snapshot.path, snapshot.content)
	}
	if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", snapshot.path, err)
	}
	return nil
}

func atomicWriteFile(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create parent directory %s: %w", parent, err)
	}
	tmp, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
