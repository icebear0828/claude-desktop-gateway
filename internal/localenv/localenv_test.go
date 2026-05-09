package localenv_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/claude-desktop-gateway/internal/localenv"
)

func TestSaveSecretCreatesOrUpdatesQuotedExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	if err := os.WriteFile(path, []byte("export OTHER_KEY='keep'\n"), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := localenv.SaveSecret(path, "OPENROUTER_API_KEY", "sk-or-test'with-dollar$"); err != nil {
		t.Fatalf("SaveSecret returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "export OTHER_KEY='keep'") {
		t.Fatalf("existing env line was not preserved:\n%s", body)
	}
	if !strings.Contains(body, "export OPENROUTER_API_KEY='sk-or-test'\\''with-dollar$'") {
		t.Fatalf("secret was not written with safe shell quoting:\n%s", body)
	}

	status, err := localenv.SecretStatus(path, []string{"OPENROUTER_API_KEY", "CLAUDE_GATEWAY_API_KEY"})
	if err != nil {
		t.Fatalf("SecretStatus returned error: %v", err)
	}
	if !status[0].Present || status[0].Value != "" {
		t.Fatalf("OPENROUTER status = %#v", status[0])
	}
	if status[1].Present {
		t.Fatalf("gateway key should be absent: %#v", status[1])
	}
}

func TestDeleteSecretRemovesOnlyMatchingAssignment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	body := "export OPENROUTER_API_KEY='secret'\nCLAUDE_GATEWAY_API_KEY=client\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := localenv.DeleteSecret(path, "OPENROUTER_API_KEY"); err != nil {
		t.Fatalf("DeleteSecret returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "OPENROUTER_API_KEY") {
		t.Fatalf("deleted key still present:\n%s", got)
	}
	if !strings.Contains(got, "CLAUDE_GATEWAY_API_KEY=client") {
		t.Fatalf("unrelated key was removed:\n%s", got)
	}
}
