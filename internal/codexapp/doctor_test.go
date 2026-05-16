package codexapp_test

import (
	"os"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/codexapp"
)

func TestDiagnoseFindsConfiguredLocalGatewayProvider(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.5"
model_provider = "local_gateway"

[model_providers.local_gateway]
name = "Local Gateway"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"

[model_providers.local_gateway.http_headers]
Authorization = "Bearer test-key"
`)

	report := diagnose(t, paths)

	if report.ActiveProvider != codexapp.DefaultProviderName {
		t.Fatalf("ActiveProvider = %q", report.ActiveProvider)
	}
	if report.Model != "gpt-5.5" {
		t.Fatalf("Model = %q", report.Model)
	}
	if got := codexIssueCodes(report); got != "" {
		t.Fatalf("issueCodes = %q", got)
	}
	output := codexapp.FormatReport(report)
	if strings.Contains(output, "test-key") {
		t.Fatalf("report leaked gateway key: %s", output)
	}
}

func TestDiagnoseFindsQuotedLocalGatewayProvider(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.5"
model_provider = "local_gateway"

[model_providers."local_gateway"]
name = "Local Gateway"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"

[model_providers."local_gateway".http_headers]
Authorization = "Bearer test-key"
`)

	report := diagnose(t, paths)

	if got := codexIssueCodes(report); got != "" {
		t.Fatalf("issueCodes = %q", got)
	}
}

func TestDiagnoseReportsMissingAndMismatchedProviderFields(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.4"
model_provider = "other"

[model_providers.local_gateway]
name = "Local Gateway"
base_url = "http://127.0.0.1:9999/v1"
wire_api = "chat"
`)

	report := diagnose(t, paths)

	for _, code := range []string{
		"model_provider_mismatch",
		"base_url_mismatch",
		"wire_api_not_responses",
		"authorization_missing",
	} {
		if !hasCodexIssue(report, code) {
			t.Fatalf("expected %s, got %q", code, codexIssueCodes(report))
		}
	}
}

func TestDiagnoseRejectsLANHTTP(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.5"
model_provider = "local_gateway"

[model_providers.local_gateway]
name = "Local Gateway"
base_url = "http://192.168.10.6:8787/v1"
wire_api = "responses"

[model_providers.local_gateway.http_headers]
Authorization = "Bearer test-key"
`)

	report, err := codexapp.Diagnose(codexapp.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: "http://192.168.10.6:8787/v1",
	})
	if err != nil {
		t.Fatalf("Diagnose returned error: %v", err)
	}
	if !hasCodexIssue(report, "base_url_insecure") {
		t.Fatalf("expected base_url_insecure, got %q", codexIssueCodes(report))
	}
}

func TestDiagnoseWarnsWhenAuthJSONExists(t *testing.T) {
	home := t.TempDir()
	paths := codexapp.PathsForHome(home, "darwin")
	writeText(t, paths.ConfigPath, `model = "gpt-5.5"
model_provider = "local_gateway"

[model_providers.local_gateway]
name = "Local Gateway"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"

[model_providers.local_gateway.http_headers]
Authorization = "Bearer test-key"
`)
	writeText(t, paths.AuthPath, `{}`)

	report := diagnose(t, paths)

	if !hasCodexIssue(report, "auth_json_present") {
		t.Fatalf("expected auth_json_present, got %q", codexIssueCodes(report))
	}
}

func TestDiagnoseReportsMissingConfig(t *testing.T) {
	report := diagnose(t, codexapp.PathsForHome(t.TempDir(), "darwin"))

	if !hasCodexIssue(report, "config_missing") {
		t.Fatalf("expected config_missing, got %q", codexIssueCodes(report))
	}
}

func diagnose(t *testing.T, paths codexapp.Paths) codexapp.Report {
	t.Helper()
	report, err := codexapp.Diagnose(codexapp.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: "http://127.0.0.1:8787/v1",
	})
	if err != nil {
		t.Fatalf("Diagnose returned error: %v", err)
	}
	return report
}

func codexIssueCodes(report codexapp.Report) string {
	codes := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		codes = append(codes, issue.Code)
	}
	return strings.Join(codes, ",")
}

func hasCodexIssue(report codexapp.Report, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
