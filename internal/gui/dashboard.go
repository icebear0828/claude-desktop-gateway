package gui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/codexapp"
	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/gateway"
	"github.com/local/claude-desktop-gateway/internal/localenv"
)

type Options struct {
	RepoRoot        string
	ConfigPath      string
	EnvPath         string
	StateDir        string
	HealthURL       string
	ExpectedBaseURL string
	DesktopPaths    claudedesktop.Paths
	CodexPaths      codexapp.Paths
	HTTPClient      *http.Client
	ManageGateway   bool
}

type Service struct {
	options Options
	client  *http.Client

	gatewayMu        sync.Mutex
	gatewayServer    *http.Server
	gatewayAddr      string
	gatewayLastError string
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
	CodexApp       CodexAppStatus           `json:"codexApp"`
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

type CodexAppStatus struct {
	State          string         `json:"state"`
	ActiveProvider string         `json:"activeProvider"`
	Model          string         `json:"model"`
	ConfigPath     string         `json:"configPath"`
	Issues         []DesktopIssue `json:"issues"`
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
	baseURL := defaultBaseURL()
	dashboard := Dashboard{
		ProjectRoot:    s.options.RepoRoot,
		ConfigPath:     s.options.ConfigPath,
		ListenURL:      baseURL,
		GeneratedAtISO: time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.ensureDefaultConfig(); err != nil {
		dashboard.ConfigError = err.Error()
	} else {
		summary, err := config.InspectFile(s.options.ConfigPath)
		if err != nil {
			dashboard.ConfigError = err.Error()
		} else {
			baseURL = listenURL(summary)
			dashboard.ListenURL = baseURL
			dashboard.Providers = summary.Providers
			dashboard.Routes = summary.Routes
		}
	}

	dashboard.Gateway = s.gatewayStatus(ctx, healthURLForBaseURL(s.options.HealthURL, baseURL))
	dashboard.ClaudeDesktop = s.claudeDesktopStatus(expectedBaseURL(s.options.ExpectedBaseURL, baseURL))
	dashboard.CodexApp = s.codexAppStatus(responsesBaseURL(expectedBaseURL(s.options.ExpectedBaseURL, baseURL)))
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
	return options
}

func (s *Service) ensureDefaultConfig() error {
	if _, err := os.Stat(s.options.ConfigPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config file: %w", err)
	}
	if _, err := config.SaveEditableFile(s.options.ConfigPath, config.DefaultEditableFile(s.options.ConfigPath)); err != nil {
		return fmt.Errorf("create default config file: %w", err)
	}
	return nil
}

func (s *Service) StartGateway(ctx context.Context) error {
	if err := s.ensureDefaultConfig(); err != nil {
		s.setGatewayLastError(err)
		return err
	}
	cfg, err := s.loadGatewayConfig()
	if err != nil {
		s.setGatewayLastError(err)
		return err
	}

	addr := cfg.Address()
	s.gatewayMu.Lock()
	if s.gatewayServer != nil && s.gatewayAddr == addr {
		s.gatewayLastError = ""
		s.gatewayMu.Unlock()
		_ = s.writeGatewayPID()
		return nil
	}
	s.gatewayMu.Unlock()

	if err := s.StopGateway(ctx); err != nil {
		s.setGatewayLastError(err)
		return err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		err = fmt.Errorf("start local gateway on %s: %w", addr, err)
		s.setGatewayLastError(err)
		return err
	}

	server := &http.Server{
		Addr:    addr,
		Handler: gateway.New(cfg, http.DefaultClient),
	}

	s.gatewayMu.Lock()
	s.gatewayServer = server
	s.gatewayAddr = addr
	s.gatewayLastError = ""
	s.gatewayMu.Unlock()

	if err := s.writeGatewayPID(); err != nil {
		_ = listener.Close()
		_ = s.StopGateway(ctx)
		s.setGatewayLastError(err)
		return err
	}
	s.appendGatewayLog(fmt.Sprintf("local gateway listening on %s://%s", cfg.Scheme(), addr))

	go s.serveGateway(server, listener, cfg)
	return nil
}

func (s *Service) StopGateway(ctx context.Context) error {
	s.gatewayMu.Lock()
	server := s.gatewayServer
	s.gatewayServer = nil
	s.gatewayAddr = ""
	s.gatewayMu.Unlock()

	_ = os.Remove(filepath.Join(s.options.StateDir, "gateway.pid"))
	if server == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("stop local gateway: %w", err)
	}
	s.appendGatewayLog("local gateway stopped")
	return nil
}

func (s *Service) RestartGateway(ctx context.Context) error {
	if err := s.StopGateway(ctx); err != nil {
		return err
	}
	return s.StartGateway(ctx)
}

func (s *Service) TryStartGateway(ctx context.Context) {
	_ = s.StartGateway(ctx)
}

func (s *Service) loadGatewayConfig() (config.Config, error) {
	env, err := localenv.Values(s.options.EnvPath)
	if err != nil {
		return config.Config{}, err
	}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok && strings.TrimSpace(value) != "" {
			env[key] = value
		}
	}
	return config.LoadFromFile(s.options.ConfigPath, env)
}

func (s *Service) serveGateway(server *http.Server, listener net.Listener, cfg config.Config) {
	var err error
	if cfg.TLSEnabled() {
		err = server.ServeTLS(listener, cfg.TLSCertFile, cfg.TLSKeyFile)
	} else {
		err = server.Serve(listener)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.setGatewayLastError(err)
		s.appendGatewayLog(fmt.Sprintf("local gateway stopped with error: %v", err))
	}

	s.gatewayMu.Lock()
	if s.gatewayServer == server {
		s.gatewayServer = nil
		s.gatewayAddr = ""
		_ = os.Remove(filepath.Join(s.options.StateDir, "gateway.pid"))
	}
	s.gatewayMu.Unlock()
}

func (s *Service) writeGatewayPID() error {
	if err := os.MkdirAll(s.options.StateDir, 0o755); err != nil {
		return fmt.Errorf("create gateway state dir: %w", err)
	}
	pid := strconv.Itoa(os.Getpid()) + "\n"
	if err := os.WriteFile(filepath.Join(s.options.StateDir, "gateway.pid"), []byte(pid), 0o600); err != nil {
		return fmt.Errorf("write gateway pid: %w", err)
	}
	return nil
}

func (s *Service) appendGatewayLog(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if err := os.MkdirAll(s.options.StateDir, 0o755); err != nil {
		return
	}
	path := filepath.Join(s.options.StateDir, "gateway.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().UTC().Format(time.RFC3339), line)
}

func (s *Service) setGatewayLastError(err error) {
	s.gatewayMu.Lock()
	defer s.gatewayMu.Unlock()
	if err == nil {
		s.gatewayLastError = ""
		return
	}
	s.gatewayLastError = err.Error()
}

func listenURL(summary config.FileSummary) string {
	scheme := "http"
	if summary.TLSCertFile != "" || summary.TLSKeyFile != "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(summary.Host, strconv.Itoa(summary.Port)))
}

func defaultBaseURL() string {
	return "http://127.0.0.1:8787"
}

func expectedBaseURL(override string, configured string) string {
	if trimmed := strings.TrimRight(strings.TrimSpace(override), "/"); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimRight(strings.TrimSpace(configured), "/"); trimmed != "" {
		return trimmed
	}
	return defaultBaseURL()
}

func healthURLForBaseURL(override string, baseURL string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return strings.TrimRight(expectedBaseURL("", baseURL), "/") + "/health"
}

func responsesBaseURL(baseURL string) string {
	return strings.TrimRight(expectedBaseURL("", baseURL), "/") + "/v1"
}

func (s *Service) gatewayStatus(ctx context.Context, healthURL string) GatewayStatus {
	status := GatewayStatus{
		State:     "stopped",
		HealthURL: healthURL,
		LogPath:   filepath.Join(s.options.StateDir, "gateway.log"),
	}

	s.gatewayMu.Lock()
	if s.gatewayServer != nil {
		status.Managed = true
		status.PID = strconv.Itoa(os.Getpid())
	}
	lastError := s.gatewayLastError
	s.gatewayMu.Unlock()

	if !status.Managed {
		if pid := readPID(filepath.Join(s.options.StateDir, "gateway.pid")); pid != "" {
			status.Managed = true
			status.PID = pid
		}
	}
	if status.Managed && status.PID == "" {
		status.PID = strconv.Itoa(os.Getpid())
	}
	if status.Managed {
		status.Managed = true
		if status.PID == "" {
			status.PID = strconv.Itoa(os.Getpid())
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		status.State = "error"
		status.Detail = err.Error()
		return status
	}
	response, err := s.client.Do(request)
	if err != nil {
		if lastError != "" {
			status.Detail = lastError
		} else {
			status.Detail = "health check failed"
		}
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

func (s *Service) claudeDesktopStatus(expectedBaseURL string) ClaudeDesktopStatus {
	report, err := claudedesktop.Diagnose(claudedesktop.DiagnosticOptions{
		Paths:           s.options.DesktopPaths,
		ExpectedBaseURL: expectedBaseURL,
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

func (s *Service) codexAppStatus(expectedBaseURL string) CodexAppStatus {
	report, err := codexapp.Diagnose(codexapp.DiagnosticOptions{
		Paths:           s.options.CodexPaths,
		ExpectedBaseURL: expectedBaseURL,
	})
	if err != nil {
		return CodexAppStatus{
			State: "error",
			Issues: []DesktopIssue{{
				Severity: "error",
				Code:     "doctor_failed",
				Message:  err.Error(),
			}},
		}
	}

	status := CodexAppStatus{
		State:          "ok",
		ActiveProvider: report.ActiveProvider,
		Model:          report.Model,
		ConfigPath:     report.Paths.ConfigPath,
		Issues:         make([]DesktopIssue, 0, len(report.Issues)),
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
