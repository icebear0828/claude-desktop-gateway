package gui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/config"
)

type Options struct {
	RepoRoot        string
	ConfigPath      string
	EnvPath         string
	StateDir        string
	HealthURL       string
	ExpectedBaseURL string
	DesktopPaths    claudedesktop.Paths
	HTTPClient      *http.Client
}

type Service struct {
	options Options
	client  *http.Client
}

type Dashboard struct {
	ProjectRoot    string                   `json:"projectRoot"`
	ConfigPath     string                   `json:"configPath"`
	ConfigError    string                   `json:"configError"`
	ListenURL      string                   `json:"listenUrl"`
	Gateway        GatewayStatus            `json:"gateway"`
	Providers      []config.ProviderSummary `json:"providers"`
	Routes         []config.RouteSummary    `json:"routes"`
	ClaudeDesktop  ClaudeDesktopStatus      `json:"claudeDesktop"`
	GeneratedAtISO string                   `json:"generatedAtIso"`
}

type GatewayStatus struct {
	State     string `json:"state"`
	Managed   bool   `json:"managed"`
	PID       string `json:"pid"`
	HealthURL string `json:"healthUrl"`
	LogPath   string `json:"logPath"`
	Detail    string `json:"detail"`
}

type ClaudeDesktopStatus struct {
	State             string         `json:"state"`
	AppliedID         string         `json:"appliedId"`
	ActiveProfilePath string         `json:"activeProfilePath"`
	Issues            []DesktopIssue `json:"issues"`
}

type DesktopIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

func NewService(options Options) *Service {
	options = normalizeOptions(options)
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 800 * time.Millisecond}
	}
	return &Service{options: options, client: client}
}

func (s *Service) Dashboard(ctx context.Context) Dashboard {
	dashboard := Dashboard{
		ProjectRoot:    s.options.RepoRoot,
		ConfigPath:     s.options.ConfigPath,
		ListenURL:      s.options.ExpectedBaseURL,
		GeneratedAtISO: time.Now().UTC().Format(time.RFC3339),
	}

	summary, err := config.InspectFile(s.options.ConfigPath)
	if err != nil {
		dashboard.ConfigError = err.Error()
	} else {
		dashboard.ListenURL = listenURL(summary)
		dashboard.Providers = summary.Providers
		dashboard.Routes = summary.Routes
	}

	dashboard.Gateway = s.gatewayStatus(ctx)
	dashboard.ClaudeDesktop = s.claudeDesktopStatus()
	return dashboard
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.RepoRoot) == "" {
		options.RepoRoot = defaultRepoRoot()
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		if envPath := strings.TrimSpace(os.Getenv("CLAUDE_GATEWAY_CONFIG")); envPath != "" {
			options.ConfigPath = envPath
		} else {
			options.ConfigPath = filepath.Join(options.RepoRoot, "gateway.local.json")
		}
	}
	if strings.TrimSpace(options.StateDir) == "" {
		options.StateDir = filepath.Join(options.RepoRoot, ".local-gateway")
	}
	if strings.TrimSpace(options.EnvPath) == "" {
		options.EnvPath = filepath.Join(options.RepoRoot, ".env.local")
	}
	if strings.TrimSpace(options.HealthURL) == "" {
		options.HealthURL = "http://127.0.0.1:8787/health"
	}
	if strings.TrimSpace(options.ExpectedBaseURL) == "" {
		options.ExpectedBaseURL = "http://127.0.0.1:8787"
	}
	return options
}

func listenURL(summary config.FileSummary) string {
	scheme := "http"
	if summary.TLSCertFile != "" || summary.TLSKeyFile != "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(summary.Host, strconv.Itoa(summary.Port)))
}

func (s *Service) gatewayStatus(ctx context.Context) GatewayStatus {
	status := GatewayStatus{
		State:     "stopped",
		HealthURL: s.options.HealthURL,
		LogPath:   filepath.Join(s.options.StateDir, "gateway.log"),
	}
	if pid := readPID(filepath.Join(s.options.StateDir, "gateway.pid")); pid != "" {
		status.Managed = true
		status.PID = pid
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.options.HealthURL, nil)
	if err != nil {
		status.State = "error"
		status.Detail = err.Error()
		return status
	}
	response, err := s.client.Do(request)
	if err != nil {
		status.Detail = "health check failed"
		return status
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		status.State = "running"
		status.Detail = "health check OK"
		return status
	}
	status.State = "error"
	status.Detail = fmt.Sprintf("health returned HTTP %d", response.StatusCode)
	return status
}

func readPID(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return ""
	}
	for _, char := range pid {
		if char < '0' || char > '9' {
			return ""
		}
	}
	return pid
}

func (s *Service) claudeDesktopStatus() ClaudeDesktopStatus {
	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           s.options.DesktopPaths,
		ExpectedBaseURL: s.options.ExpectedBaseURL,
	})
	if err != nil {
		return ClaudeDesktopStatus{
			State: "error",
			Issues: []DesktopIssue{{
				Severity: "error",
				Code:     "doctor_failed",
				Message:  err.Error(),
			}},
		}
	}

	status := ClaudeDesktopStatus{
		State:             "ok",
		AppliedID:         report.AppliedID,
		ActiveProfilePath: report.ActiveProfilePath,
		Issues:            make([]DesktopIssue, 0, len(report.Issues)),
	}
	hasError := false
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			hasError = true
		}
		status.Issues = append(status.Issues, DesktopIssue{
			Severity: issue.Severity,
			Code:     issue.Code,
			Path:     issue.Path,
			Message:  issue.Message,
		})
	}
	if hasError {
		status.State = "error"
	} else if len(status.Issues) > 0 {
		status.State = "warning"
	}
	return status
}
