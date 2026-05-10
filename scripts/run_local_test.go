package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestRunLocalDryRunRequiresEnvFile(t *testing.T) {
	t.Parallel()

	missingEnvPath := filepath.Join(t.TempDir(), ".env.local")
	cmd := scriptCommand("run-local", "--dry-run")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_ENV_FILE="+missingEnvPath,
		"CLAUDE_GATEWAY_CONFIG="+filepath.Join(t.TempDir(), "gateway.local.json"),
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("run-local returned nil error without env file")
	}
	if !strings.Contains(string(output), "missing env file") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(string(output), "secret") {
		t.Fatalf("output leaked a secret-like value: %q", output)
	}
}

func TestRunLocalDryRunRequiresOpenRouterAPIKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.local")
	configPath := filepath.Join(dir, "gateway.local.json")
	if err := os.WriteFile(envPath, []byte(`export CLAUDE_GATEWAY_API_KEY="secret-client-key"`+"\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cmd := scriptCommand("run-local", "--dry-run")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_ENV_FILE="+envPath,
		"CLAUDE_GATEWAY_CONFIG="+configPath,
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("run-local returned nil error without OPENROUTER_API_KEY")
	}
	if !strings.Contains(string(output), "OPENROUTER_API_KEY is required") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(string(output), "secret-client-key") {
		t.Fatalf("output leaked CLAUDE_GATEWAY_API_KEY: %q", output)
	}
}

func TestRunLocalDryRunDoesNotPrintSecrets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.local")
	configPath := filepath.Join(dir, "gateway.local.json")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		`export OPENROUTER_API_KEY="secret-openrouter-key"`,
		`export CLAUDE_GATEWAY_API_KEY="secret-client-key"`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cmd := scriptCommand("run-local", "--dry-run")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_ENV_FILE="+envPath,
		"CLAUDE_GATEWAY_CONFIG="+configPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run-local returned error: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "local gateway configuration OK") {
		t.Fatalf("output = %q", output)
	}
	for _, secret := range []string{"secret-openrouter-key", "secret-client-key"} {
		if strings.Contains(text, secret) {
			t.Fatalf("output leaked %s on %s: %q", secret, runtime.GOOS, output)
		}
	}
}

func TestLocalGatewayStartDryRunDoesNotPrintSecrets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env.local")
	configPath := filepath.Join(dir, "gateway.local.json")
	stateDir := filepath.Join(dir, "state")
	if err := os.WriteFile(envPath, []byte(strings.Join([]string{
		`export OPENROUTER_API_KEY="secret-openrouter-key"`,
		`export CLAUDE_GATEWAY_API_KEY="secret-client-key"`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cmd := scriptCommand("local-gateway", "start", "--dry-run")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_ENV_FILE="+envPath,
		"CLAUDE_GATEWAY_CONFIG="+configPath,
		"CLAUDE_GATEWAY_STATE_DIR="+stateDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("local-gateway start --dry-run returned error: %v\n%s", err, output)
	}
	text := string(output)
	if !strings.Contains(text, "local gateway start dry run OK") {
		t.Fatalf("output = %q", output)
	}
	for _, secret := range []string{"secret-openrouter-key", "secret-client-key"} {
		if strings.Contains(text, secret) {
			t.Fatalf("output leaked %s on %s: %q", secret, runtime.GOOS, output)
		}
	}
	if _, err := os.Stat(filepath.Join(stateDir, "gateway.pid")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write pid file, stat err = %v", err)
	}
}

func TestLocalGatewayStatusReportsManagedPID(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	pidPath := filepath.Join(stateDir, "gateway.pid")
	pid := managedPIDForStatusTest(t)
	if err := os.WriteFile(pidPath, []byte(pid+"\n"), 0o600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	cmd := scriptCommand("local-gateway", "status")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_STATE_DIR="+stateDir,
		"CLAUDE_GATEWAY_HEALTH_URL=http://127.0.0.1:1/health",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("local-gateway status returned error: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "local gateway running: pid "+pid) {
		t.Fatalf("output = %q", output)
	}
}

func TestLocalGatewayStopWithoutManagedProcessIsClear(t *testing.T) {
	t.Parallel()

	cmd := scriptCommand("local-gateway", "stop")
	cmd.Env = append(os.Environ(),
		"CLAUDE_GATEWAY_STATE_DIR="+t.TempDir(),
		"CLAUDE_GATEWAY_HEALTH_URL=http://127.0.0.1:1/health",
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("local-gateway stop returned nil error without a managed process")
	}
	if !strings.Contains(string(output), "local gateway is not managed by this script") {
		t.Fatalf("output = %q", output)
	}
}

func scriptCommand(name string, args ...string) *exec.Cmd {
	scriptPath := "./" + name
	if runtime.GOOS == "windows" {
		return exec.Command("bash", append([]string{scriptPath}, args...)...)
	}
	return exec.Command(scriptPath, args...)
}

func managedPIDForStatusTest(t *testing.T) string {
	t.Helper()

	if runtime.GOOS != "windows" {
		return strconv.Itoa(os.Getpid())
	}

	cmd := exec.Command("bash", "-c", "sleep 30 >/dev/null 2>&1 & echo $!")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("start bash sleep process: %v", err)
	}
	pid := strings.TrimSpace(string(output))
	if _, err := strconv.Atoi(pid); err != nil {
		t.Fatalf("bash sleep pid = %q: %v", pid, err)
	}
	t.Cleanup(func() {
		_ = exec.Command("bash", "-c", "kill "+pid).Run()
	})
	return pid
}
