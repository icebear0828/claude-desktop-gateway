package claudedesktop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
)

func TestDiagnoseFindsActiveProfileFromMeta(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.MetaPath, `{
		"appliedId": "active-profile",
		"entries": [{"id": "active-profile", "name": "Local Gateway"}]
	}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "active-profile.json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
		"inferenceGatewayApiKey": "secret-client-key",
		"inferenceGatewayAuthScheme": "bearer",
		"inferenceModels": "[\"claude-sonnet-4.6\"]"
	}`)

	report := diagnose(t, paths)

	if report.AppliedID != "active-profile" {
		t.Fatalf("AppliedID = %q", report.AppliedID)
	}
	if report.ActiveProfilePath != filepath.Join(paths.ConfigLibraryPath, "active-profile.json") {
		t.Fatalf("ActiveProfilePath = %q", report.ActiveProfilePath)
	}
	if got := issueCodes(report); got != "" {
		t.Fatalf("issueCodes = %q", got)
	}
	output := claudedesktop.FormatReport(report)
	if strings.Contains(output, "secret-client-key") {
		t.Fatalf("report leaked gateway key: %s", output)
	}
}

func TestDiagnoseRejectsNestedEnterpriseConfigProfile(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.MetaPath, `{"appliedId": "active-profile"}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "active-profile.json"), `{
		"enterpriseConfig": {
			"inferenceProvider": "gateway",
			"inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
			"inferenceGatewayApiKey": "secret-client-key",
			"inferenceGatewayAuthScheme": "bearer"
		}
	}`)

	report := diagnose(t, paths)

	if !hasIssue(report, "profile_nested_enterprise_config") {
		t.Fatalf("expected profile_nested_enterprise_config, got %q", issueCodes(report))
	}
	if !hasIssue(report, "profile_missing_provider") {
		t.Fatalf("expected profile_missing_provider, got %q", issueCodes(report))
	}
}

func TestDiagnoseFlagsBaseURLMismatchAndLANHTTP(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.MetaPath, `{"appliedId": "active-profile"}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "active-profile.json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "http://192.168.10.6:8787",
		"inferenceGatewayApiKey": "secret-client-key",
		"inferenceGatewayAuthScheme": "bearer"
	}`)

	report := diagnose(t, paths)

	if !hasIssue(report, "profile_base_url_mismatch") {
		t.Fatalf("expected profile_base_url_mismatch, got %q", issueCodes(report))
	}
	if !hasIssue(report, "profile_lan_http_not_allowed") {
		t.Fatalf("expected profile_lan_http_not_allowed, got %q", issueCodes(report))
	}
}

func TestDiagnoseFindsGatewayProfileThatIsNotApplied(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.MetaPath, `{
		"appliedId": "old-profile",
		"entries": [
			{"id": "old-profile", "name": "Old"},
			{"id": "local-gateway", "name": "Local Gateway"}
		]
	}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "old-profile.json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "https://wrong.example.test",
		"inferenceGatewayApiKey": "old-key",
		"inferenceGatewayAuthScheme": "bearer"
	}`)
	writeJSON(t, filepath.Join(paths.ConfigLibraryPath, "local-gateway.json"), `{
		"inferenceProvider": "gateway",
		"inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
		"inferenceGatewayApiKey": "secret-client-key",
		"inferenceGatewayAuthScheme": "bearer"
	}`)

	report := diagnose(t, paths)

	if !hasIssue(report, "profile_base_url_mismatch") {
		t.Fatalf("expected profile_base_url_mismatch, got %q", issueCodes(report))
	}
	if !hasIssue(report, "expected_profile_not_applied") {
		t.Fatalf("expected expected_profile_not_applied, got %q", issueCodes(report))
	}
}

func TestDiagnoseDoesNotWarnForDuplicateExpectedProfileWhenActiveMatches(t *testing.T) {
	home := t.TempDir()
	paths := claudedesktop.PathsForHome(home, "darwin")
	writeJSON(t, paths.NormalConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.ThreePConfigPath, `{"deploymentMode":"3p"}`)
	writeJSON(t, paths.MetaPath, `{
		"appliedId": "active-profile",
		"entries": [
			{"id": "active-profile", "name": "Active"},
			{"id": "old-profile", "name": "Old"}
		]
	}`)
	for _, profileID := range []string{"active-profile", "old-profile"} {
		writeJSON(t, filepath.Join(paths.ConfigLibraryPath, profileID+".json"), `{
			"inferenceProvider": "gateway",
			"inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
			"inferenceGatewayApiKey": "secret-client-key",
			"inferenceGatewayAuthScheme": "bearer"
		}`)
	}

	report := diagnose(t, paths)

	if hasIssue(report, "expected_profile_not_applied") {
		t.Fatalf("did not expect expected_profile_not_applied, got %q", issueCodes(report))
	}
}

func diagnose(t *testing.T, paths claudedesktop.Paths) claudedesktop.Report {
	t.Helper()

	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: "http://127.0.0.1:8787",
	})
	if err != nil {
		t.Fatalf("Diagnose returned error: %v", err)
	}
	return report
}

func writeJSON(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func issueCodes(report claudedesktop.Report) string {
	codes := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		codes = append(codes, issue.Code)
	}
	return strings.Join(codes, ",")
}

func hasIssue(report claudedesktop.Report, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
