package claudedesktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	configFile       = "claude_desktop_config.json"
	configLibraryDir = "configLibrary"
)

type Paths struct {
	NormalConfigPath  string
	ThreePConfigPath  string
	ConfigLibraryPath string
	MetaPath          string
}

type DiagnosticOptions struct {
	Paths           Paths
	ExpectedBaseURL string
}

type Report struct {
	Paths             Paths
	AppliedID         string
	ActiveProfilePath string
	ProfileSummaries  []ProfileSummary
	Issues            []Issue
}

type ProfileSummary struct {
	ID               string
	Path             string
	BaseURL          string
	HasGatewayFields bool
	HasEnterprise    bool
	IsActive         bool
}

type Issue struct {
	Severity string
	Code     string
	Path     string
	Message  string
}

type jsonObject map[string]json.RawMessage

func CurrentPlatformPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("find home dir: %w", err)
	}
	return PathsForHome(home, runtime.GOOS), nil
}

func PathsForHome(home string, goos string) Paths {
	switch goos {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if strings.TrimSpace(localAppData) == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return pathsFromDirs(
			filepath.Join(localAppData, "Claude"),
			filepath.Join(localAppData, "Claude-3p"),
		)
	default:
		appSupport := filepath.Join(home, "Library", "Application Support")
		return pathsFromDirs(
			filepath.Join(appSupport, "Claude"),
			filepath.Join(appSupport, "Claude-3p"),
		)
	}
}

func Diagnose(options DiagnosticOptions) (Report, error) {
	paths := options.Paths
	if paths.ConfigLibraryPath == "" {
		var err error
		paths, err = CurrentPlatformPaths()
		if err != nil {
			return Report{}, err
		}
	}

	report := Report{Paths: paths}
	checkDeploymentMode(&report, paths.NormalConfigPath)
	checkDeploymentMode(&report, paths.ThreePConfigPath)

	meta, exists, err := readJSONObject(paths.MetaPath)
	if err != nil {
		report.add("error", "meta_invalid_json", paths.MetaPath, err.Error())
		return report, nil
	}
	if !exists {
		report.add("error", "meta_missing", paths.MetaPath, "Claude Desktop 3P _meta.json is missing")
		scanProfiles(&report, options.ExpectedBaseURL)
		return report, nil
	}

	report.AppliedID = stringField(meta, "appliedId")
	if report.AppliedID == "" {
		report.add("error", "meta_missing_applied_id", paths.MetaPath, "_meta.json does not contain appliedId")
	} else {
		report.ActiveProfilePath = filepath.Join(paths.ConfigLibraryPath, report.AppliedID+".json")
		checkActiveProfile(&report, report.ActiveProfilePath, options.ExpectedBaseURL)
	}

	scanProfiles(&report, options.ExpectedBaseURL)
	return report, nil
}

func FormatReport(report Report) string {
	var b strings.Builder
	b.WriteString("Claude Desktop config doctor\n")
	writeLine(&b, "normal config", report.Paths.NormalConfigPath)
	writeLine(&b, "3p config", report.Paths.ThreePConfigPath)
	writeLine(&b, "config library", report.Paths.ConfigLibraryPath)
	if report.AppliedID != "" {
		writeLine(&b, "applied profile", report.AppliedID)
	}
	if report.ActiveProfilePath != "" {
		writeLine(&b, "active profile path", report.ActiveProfilePath)
	}

	if len(report.ProfileSummaries) > 0 {
		b.WriteString("profiles:\n")
		for _, profile := range report.ProfileSummaries {
			marker := " "
			if profile.IsActive {
				marker = "*"
			}
			baseURL := profile.BaseURL
			if baseURL == "" {
				baseURL = "<none>"
			}
			fmt.Fprintf(&b, "  %s %s baseUrl=%s path=%s\n", marker, profile.ID, baseURL, profile.Path)
		}
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

func pathsFromDirs(normalDir string, threepDir string) Paths {
	configLibraryPath := filepath.Join(threepDir, configLibraryDir)
	return Paths{
		NormalConfigPath:  filepath.Join(normalDir, configFile),
		ThreePConfigPath:  filepath.Join(threepDir, configFile),
		ConfigLibraryPath: configLibraryPath,
		MetaPath:          filepath.Join(configLibraryPath, "_meta.json"),
	}
}

func checkDeploymentMode(report *Report, path string) {
	obj, exists, err := readJSONObject(path)
	if err != nil {
		report.add("warning", "deployment_config_invalid_json", path, err.Error())
		return
	}
	if !exists {
		report.add("warning", "deployment_config_missing", path, "Claude Desktop deployment config is missing")
		return
	}
	if nestedEnterpriseHasGateway(obj) {
		report.add("warning", "root_nested_enterprise_config", path, "gateway settings are nested under enterpriseConfig; active 3P profile should use flat gateway fields")
	}
	if mode := stringField(obj, "deploymentMode"); mode != "3p" {
		report.add("warning", "deployment_mode_not_3p", path, fmt.Sprintf("deploymentMode is %q, expected \"3p\"", mode))
	}
}

func checkActiveProfile(report *Report, path string, expectedBaseURL string) {
	obj, exists, err := readJSONObject(path)
	if err != nil {
		report.add("error", "profile_invalid_json", path, err.Error())
		return
	}
	if !exists {
		report.add("error", "profile_missing", path, "active profile file does not exist")
		return
	}

	if nestedEnterpriseHasGateway(obj) {
		report.add("error", "profile_nested_enterprise_config", path, "active profile uses nested enterpriseConfig; Claude Desktop expects flat gateway fields in the active profile")
	}
	if provider := stringField(obj, "inferenceProvider"); provider != "gateway" {
		report.add("error", "profile_missing_provider", path, fmt.Sprintf("inferenceProvider is %q, expected \"gateway\"", provider))
	}
	baseURL := stringField(obj, "inferenceGatewayBaseUrl")
	if baseURL == "" {
		report.add("error", "profile_missing_base_url", path, "inferenceGatewayBaseUrl is missing")
	} else {
		checkBaseURL(report, path, baseURL, expectedBaseURL)
	}
	if scheme := stringField(obj, "inferenceGatewayAuthScheme"); !strings.EqualFold(scheme, "bearer") {
		report.add("error", "profile_auth_scheme_not_bearer", path, fmt.Sprintf("inferenceGatewayAuthScheme is %q, expected \"bearer\"", scheme))
	}
	if key := stringField(obj, "inferenceGatewayApiKey"); strings.TrimSpace(key) == "" {
		report.add("error", "profile_missing_api_key", path, "inferenceGatewayApiKey is missing or empty")
	}
	if raw, ok := obj["inferenceModels"]; ok && !validInferenceModels(raw) {
		report.add("error", "profile_invalid_models", path, "inferenceModels must be an array or a JSON string array")
	}
}

func checkBaseURL(report *Report, path string, actual string, expected string) {
	if strings.TrimSpace(expected) != "" && actual != expected {
		report.add("error", "profile_base_url_mismatch", path, fmt.Sprintf("inferenceGatewayBaseUrl is %q, expected %q", actual, expected))
	}

	parsed, err := url.Parse(actual)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		report.add("error", "profile_base_url_invalid", path, fmt.Sprintf("inferenceGatewayBaseUrl is not a valid absolute URL: %q", actual))
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		report.add("error", "profile_base_url_scheme_invalid", path, fmt.Sprintf("inferenceGatewayBaseUrl uses unsupported scheme %q", parsed.Scheme))
		return
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		report.add("error", "profile_lan_http_not_allowed", path, "Claude Desktop allows http only for loopback; LAN/VPS gateway URLs must use trusted https")
	}
}

func scanProfiles(report *Report, expectedBaseURL string) {
	entries, err := os.ReadDir(report.Paths.ConfigLibraryPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			report.add("warning", "profile_scan_failed", report.Paths.ConfigLibraryPath, err.Error())
		}
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || name == "_meta.json" {
			continue
		}
		path := filepath.Join(report.Paths.ConfigLibraryPath, name)
		obj, exists, err := readJSONObject(path)
		if err != nil || !exists {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		summary := ProfileSummary{
			ID:               id,
			Path:             path,
			BaseURL:          stringField(obj, "inferenceGatewayBaseUrl"),
			HasGatewayFields: stringField(obj, "inferenceProvider") == "gateway",
			HasEnterprise:    nestedEnterpriseHasGateway(obj),
			IsActive:         id == report.AppliedID,
		}
		report.ProfileSummaries = append(report.ProfileSummaries, summary)
	}

	sort.Slice(report.ProfileSummaries, func(i int, j int) bool {
		return report.ProfileSummaries[i].ID < report.ProfileSummaries[j].ID
	})

	if expectedBaseURL == "" {
		return
	}
	for _, profile := range report.ProfileSummaries {
		if profile.IsActive && profile.HasGatewayFields && profile.BaseURL == expectedBaseURL {
			return
		}
	}
	for _, profile := range report.ProfileSummaries {
		if profile.IsActive {
			continue
		}
		if profile.HasGatewayFields && profile.BaseURL == expectedBaseURL {
			report.add("warning", "expected_profile_not_applied", profile.Path, "a profile with the expected gateway URL exists, but _meta.json applies a different profile")
		}
	}
}

func readJSONObject(path string) (jsonObject, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read JSON file: %w", err)
	}

	var obj jsonObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, true, fmt.Errorf("parse JSON file: %w", err)
	}
	if obj == nil {
		obj = jsonObject{}
	}
	return obj, true, nil
}

func stringField(obj jsonObject, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func nestedEnterpriseHasGateway(obj jsonObject) bool {
	raw, ok := obj["enterpriseConfig"]
	if !ok {
		return false
	}
	var nested jsonObject
	if err := json.Unmarshal(raw, &nested); err != nil {
		return false
	}
	return stringField(nested, "inferenceProvider") == "gateway" ||
		stringField(nested, "inferenceGatewayBaseUrl") != ""
}

func validInferenceModels(raw json.RawMessage) bool {
	var models []string
	if err := json.Unmarshal(raw, &models); err == nil {
		return true
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return false
	}
	return json.Unmarshal([]byte(encoded), &models) == nil
}

func isLoopbackHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if trimmed == "" || strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func (report *Report) add(severity string, code string, path string, message string) {
	report.Issues = append(report.Issues, Issue{
		Severity: severity,
		Code:     code,
		Path:     path,
		Message:  message,
	})
}

func writeLine(b *strings.Builder, label string, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
}
