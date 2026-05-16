package codexapp

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultProviderName  = "local_gateway"
	DefaultProviderLabel = "Local Gateway"
	DefaultBaseURL       = "http://127.0.0.1:8787/v1"
	DefaultModel         = "gpt-5.5"
	DefaultWireAPI       = "responses"
)

type Paths struct {
	ConfigPath string
	AuthPath   string
}

type DiagnosticOptions struct {
	Paths           Paths
	ExpectedBaseURL string
	ProviderName    string
}

type Report struct {
	Paths          Paths
	ProviderName   string
	ActiveProvider string
	Model          string
	BaseURL        string
	WireAPI        string
	Issues         []Issue
}

type Issue struct {
	Severity string
	Code     string
	Path     string
	Message  string
}

type parsedConfig struct {
	top            map[string]string
	provider       map[string]string
	headers        map[string]string
	providerExists bool
	headersExists  bool
}

func CurrentPlatformPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("find home dir: %w", err)
	}
	return PathsForHome(home, runtime.GOOS), nil
}

func PathsForHome(home string, goos string) Paths {
	codexDir := filepath.Join(home, ".codex")
	return Paths{
		ConfigPath: filepath.Join(codexDir, "config.toml"),
		AuthPath:   filepath.Join(codexDir, "auth.json"),
	}
}

func Diagnose(options DiagnosticOptions) (Report, error) {
	paths := options.Paths
	if strings.TrimSpace(paths.ConfigPath) == "" {
		var err error
		paths, err = CurrentPlatformPaths()
		if err != nil {
			return Report{}, err
		}
	}
	providerName := valueOrDefault(options.ProviderName, DefaultProviderName)
	expectedBaseURL := strings.TrimRight(valueOrDefault(options.ExpectedBaseURL, DefaultBaseURL), "/")
	report := Report{Paths: paths, ProviderName: providerName}

	data, err := os.ReadFile(paths.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			report.add("error", "config_missing", paths.ConfigPath, "Codex config.toml is missing")
			checkAuthJSON(&report)
			return report, nil
		}
		return Report{}, fmt.Errorf("read Codex config: %w", err)
	}

	parsed := parseConfig(string(data), providerName)
	report.ActiveProvider = parsed.top["model_provider"]
	report.Model = parsed.top["model"]
	report.BaseURL = parsed.provider["base_url"]
	report.WireAPI = parsed.provider["wire_api"]

	if report.ActiveProvider != providerName {
		report.add("error", "model_provider_mismatch", paths.ConfigPath, fmt.Sprintf("model_provider is %q, expected %q", report.ActiveProvider, providerName))
	}
	if !parsed.providerExists {
		report.add("error", "provider_missing", paths.ConfigPath, fmt.Sprintf("model provider %q is missing", providerName))
		checkAuthJSON(&report)
		return report, nil
	}
	if strings.TrimRight(report.BaseURL, "/") != expectedBaseURL {
		report.add("error", "base_url_mismatch", paths.ConfigPath, fmt.Sprintf("base_url is %q, expected %q", report.BaseURL, expectedBaseURL))
	}
	if err := validateGatewayBaseURL(strings.TrimRight(report.BaseURL, "/")); err != nil {
		code := "base_url_invalid"
		if strings.Contains(err.Error(), "http only for loopback") {
			code = "base_url_insecure"
		}
		report.add("error", code, paths.ConfigPath, err.Error())
	}
	if report.WireAPI != DefaultWireAPI {
		report.add("error", "wire_api_not_responses", paths.ConfigPath, fmt.Sprintf("wire_api is %q, expected %q", report.WireAPI, DefaultWireAPI))
	}
	authorization := strings.TrimSpace(parsed.headers["Authorization"])
	if !parsed.headersExists || authorization == "" || !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		report.add("error", "authorization_missing", paths.ConfigPath, "provider Authorization bearer header is missing")
	}
	checkAuthJSON(&report)
	return report, nil
}

func FormatReport(report Report) string {
	var b strings.Builder
	b.WriteString("Codex App config doctor\n")
	writeLine(&b, "config", report.Paths.ConfigPath)
	writeLine(&b, "auth", report.Paths.AuthPath)
	writeLine(&b, "provider", report.ProviderName)
	if report.ActiveProvider != "" {
		writeLine(&b, "active provider", report.ActiveProvider)
	}
	if report.Model != "" {
		writeLine(&b, "model", report.Model)
	}
	if report.BaseURL != "" {
		writeLine(&b, "base url", report.BaseURL)
	}
	if report.WireAPI != "" {
		writeLine(&b, "wire api", report.WireAPI)
	}
	if len(report.Issues) == 0 {
		b.WriteString("[ok] no config issues found\n")
		return b.String()
	}
	for _, issue := range report.Issues {
		path := ""
		if issue.Path != "" {
			path = " (" + issue.Path + ")"
		}
		fmt.Fprintf(&b, "[%s] %s: %s%s\n", issue.Severity, issue.Code, issue.Message, path)
	}
	return b.String()
}

func (r *Report) add(severity string, code string, path string, message string) {
	r.Issues = append(r.Issues, Issue{
		Severity: severity,
		Code:     code,
		Path:     path,
		Message:  message,
	})
}

func checkAuthJSON(report *Report) {
	if strings.TrimSpace(report.Paths.AuthPath) == "" {
		return
	}
	if _, err := os.Stat(report.Paths.AuthPath); err == nil {
		report.add("warning", "auth_json_present", report.Paths.AuthPath, "Codex auth.json exists and may override provider Authorization headers")
	}
}

func parseConfig(body string, providerName string) parsedConfig {
	parsed := parsedConfig{
		top:      map[string]string{},
		provider: map[string]string{},
		headers:  map[string]string{},
	}
	providerSection := "model_providers." + providerName
	headersSection := providerSection + ".http_headers"
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if normalizedSection, ok := tableSection(line); ok {
			section = normalizedSection
			if section == providerSection {
				parsed.providerExists = true
			}
			if section == headersSection {
				parsed.headersExists = true
			}
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			continue
		}
		switch section {
		case "":
			parsed.top[key] = value
		case providerSection:
			parsed.provider[key] = value
		case headersSection:
			parsed.headers[key] = value
		}
	}
	return parsed
}

func parseAssignment(line string) (string, string, bool) {
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}
	return key, parseStringValue(value), true
}

func parseStringValue(value string) string {
	value = strings.TrimSpace(stripInlineComment(value))
	if value == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	return strings.Trim(value, `"`)
}

func stripInlineComment(value string) string {
	inString := false
	escaped := false
	for index, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if char == '#' && !inString {
			return value[:index]
		}
	}
	return value
}

func validateGatewayBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("base URL is not a valid absolute URL: %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("base URL uses unsupported scheme %q", parsed.Scheme)
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("Codex App allows http only for loopback; LAN/VPS gateway URLs must use trusted https")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func valueOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func writeLine(b *strings.Builder, key string, value string) {
	if value != "" {
		fmt.Fprintf(b, "%s: %s\n", key, value)
	}
}
