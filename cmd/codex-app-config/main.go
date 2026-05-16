package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/codexapp"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "apply-local":
		runApplyLocal(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runApplyLocal(args []string) {
	flags := flag.NewFlagSet("apply-local", flag.ExitOnError)
	baseURL := flags.String("base-url", codexapp.DefaultBaseURL, "Codex App gateway base URL")
	apiKeyEnv := flags.String("api-key-env", "CLAUDE_GATEWAY_API_KEY", "environment variable containing the gateway API key")
	providerName := flags.String("provider-name", codexapp.DefaultProviderName, "Codex model provider name to write")
	providerLabel := flags.String("provider-label", codexapp.DefaultProviderLabel, "Codex model provider display name")
	model := flags.String("model", "", "Codex model ID to select")
	home := flags.String("home", "", "override home directory")
	goos := flags.String("goos", "", "override GOOS path layout")
	dryRun := flags.Bool("dry-run", false, "print the planned write without changing files")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	paths, err := pathsFromFlags(*home, *goos)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env := currentEnv()
	selectedModel := modelForApply(*model, env)
	if *dryRun {
		result := codexapp.ApplyResult{
			Paths:        paths,
			ProviderName: *providerName,
			BaseURL:      strings.TrimRight(*baseURL, "/"),
			Model:        selectedModel,
		}
		fmt.Print(codexapp.FormatApplyResult(result))
		fmt.Println("dry run: no files changed")
		return
	}

	apiKey := strings.TrimSpace(env[*apiKeyEnv])
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", *apiKeyEnv)
		os.Exit(1)
	}
	result, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         paths,
		ProviderName:  *providerName,
		ProviderLabel: *providerLabel,
		BaseURL:       *baseURL,
		GatewayAPIKey: apiKey,
		Model:         selectedModel,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(codexapp.FormatApplyResult(result))
	fmt.Println("restart Codex App before using the repaired provider")
}

func runDoctor(args []string) {
	flags := flag.NewFlagSet("doctor", flag.ExitOnError)
	expectedBaseURL := flags.String("expected-base-url", codexapp.DefaultBaseURL, "expected Codex App gateway base URL")
	providerName := flags.String("provider-name", codexapp.DefaultProviderName, "expected Codex model provider name")
	home := flags.String("home", "", "override home directory for diagnostics")
	goos := flags.String("goos", "", "override GOOS path layout for diagnostics")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	paths, err := pathsFromFlags(*home, *goos)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := codexapp.Diagnose(codexapp.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: *expectedBaseURL,
		ProviderName:    *providerName,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(codexapp.FormatReport(report))
	if hasError(report) {
		os.Exit(1)
	}
}

func pathsFromFlags(home string, goos string) (codexapp.Paths, error) {
	if home == "" {
		return codexapp.CurrentPlatformPaths()
	}
	layout := goos
	if layout == "" {
		layout = "darwin"
	}
	return codexapp.PathsForHome(home, layout), nil
}

func modelForApply(value string, env map[string]string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(env["CODEX_MODEL"]); trimmed != "" {
		return trimmed
	}
	return codexapp.DefaultModel
}

func currentEnv() map[string]string {
	env := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func hasError(report codexapp.Report) bool {
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: codex-app-config <doctor|apply-local> [flags]")
}
