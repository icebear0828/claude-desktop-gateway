package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/config"
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
	baseURL := flags.String("base-url", "http://127.0.0.1:8787", "Claude Desktop gateway base URL")
	apiKeyEnv := flags.String("api-key-env", "CLAUDE_GATEWAY_API_KEY", "environment variable containing the Claude Desktop gateway API key")
	models := flags.String("models", "", "comma-separated Claude Desktop model IDs")
	profileID := flags.String("profile-id", claudedesktop.DefaultProfileID, "Claude Desktop 3P profile ID to write and apply")
	profileName := flags.String("profile-name", claudedesktop.DefaultProfileName, "Claude Desktop 3P profile display name")
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
	modelIDs, err := modelIDsForApply(*models, env)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *dryRun {
		result := claudedesktop.ApplyResult{
			Paths:       paths,
			ProfileID:   *profileID,
			ProfilePath: paths.ConfigLibraryPath + string(os.PathSeparator) + *profileID + ".json",
			BaseURL:     *baseURL,
			ModelIDs:    modelIDs,
		}
		fmt.Print(claudedesktop.FormatApplyResult(result))
		fmt.Println("dry run: no files changed")
		return
	}

	apiKey := strings.TrimSpace(env[*apiKeyEnv])
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "%s is required\n", *apiKeyEnv)
		os.Exit(1)
	}

	result, err := claudedesktop.ApplyLocal(claudedesktop.ApplyOptions{
		Paths:         paths,
		ProfileID:     *profileID,
		ProfileName:   *profileName,
		BaseURL:       *baseURL,
		GatewayAPIKey: apiKey,
		ModelIDs:      modelIDs,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(claudedesktop.FormatApplyResult(result))
	fmt.Println("restart Claude Desktop with Cmd+Q, then reopen it")
}

func runDoctor(args []string) {
	flags := flag.NewFlagSet("doctor", flag.ExitOnError)
	expectedBaseURL := flags.String("expected-base-url", "http://127.0.0.1:8787", "expected Claude Desktop gateway base URL")
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

	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           paths,
		ExpectedBaseURL: *expectedBaseURL,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Print(claudedesktop.FormatReport(report))
	if hasError(report) {
		os.Exit(1)
	}
}

func pathsFromFlags(home string, goos string) (claudedesktop.Paths, error) {
	if home == "" {
		return claudedesktop.CurrentPlatformPaths()
	}
	layout := goos
	if layout == "" {
		layout = "darwin"
	}
	return claudedesktop.PathsForHome(home, layout), nil
}

func modelIDsFromCSV(value string) []string {
	parts := strings.Split(value, ",")
	modelIDs := make([]string, 0, len(parts))
	for _, part := range parts {
		modelID := strings.TrimSpace(part)
		if modelID != "" {
			modelIDs = append(modelIDs, modelID)
		}
	}
	return modelIDs
}

func modelIDsForApply(value string, env map[string]string) ([]string, error) {
	explicitModelIDs := modelIDsFromCSV(value)
	if len(explicitModelIDs) > 0 {
		return explicitModelIDs, nil
	}

	cfgPath := strings.TrimSpace(env["CLAUDE_GATEWAY_CONFIG"])
	cfg, err := loadGatewayConfigForApply(cfgPath, env)
	if err != nil {
		if cfgPath != "" {
			return nil, fmt.Errorf("load gateway config: %w", err)
		}
		return claudedesktop.DefaultModelIDs(), nil
	}
	modelIDs := cfg.DesktopModelIDs()
	if len(modelIDs) == 0 {
		return claudedesktop.DefaultModelIDs(), nil
	}
	return modelIDs, nil
}

func loadGatewayConfigForApply(path string, env map[string]string) (config.Config, error) {
	if path != "" {
		return config.LoadFromFile(path, env)
	}
	return config.LoadFromEnv(env)
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

func hasError(report claudedesktop.Report) bool {
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: claude-desktop-config <doctor|apply-local> [flags]")
}
