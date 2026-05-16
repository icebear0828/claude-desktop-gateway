package gui

import (
	"context"
	"fmt"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/codexapp"
	"github.com/local/claude-desktop-gateway/internal/config"
)

type CodexAppApplyResult struct {
	ProviderName    string `json:"providerName"`
	ConfigPath      string `json:"configPath"`
	BaseURL         string `json:"baseUrl"`
	Model           string `json:"model"`
	RestartRequired bool   `json:"restartRequired"`
}

func (s *Service) ApplyCodexAppConfig() (CodexAppApplyResult, error) {
	if err := s.ensureDefaultConfig(); err != nil {
		return CodexAppApplyResult{}, err
	}
	gatewayAPIKey, err := s.gatewayAPIKeyForRepair()
	if err != nil {
		return CodexAppApplyResult{}, err
	}
	model, err := s.codexModelID()
	if err != nil {
		return CodexAppApplyResult{}, err
	}
	baseURL := responsesBaseURL(s.desktopBaseURL())
	result, err := codexapp.ApplyLocal(codexapp.ApplyOptions{
		Paths:         s.options.CodexPaths,
		BaseURL:       baseURL,
		GatewayAPIKey: gatewayAPIKey,
		Model:         model,
	})
	if err != nil {
		return CodexAppApplyResult{}, err
	}
	if s.options.ManageGateway {
		if err := s.RestartGateway(context.Background()); err != nil {
			return CodexAppApplyResult{}, err
		}
	}
	return CodexAppApplyResult{
		ProviderName:    result.ProviderName,
		ConfigPath:      result.Paths.ConfigPath,
		BaseURL:         result.BaseURL,
		Model:           result.Model,
		RestartRequired: true,
	}, nil
}

func (s *Service) codexModelID() (string, error) {
	summary, err := config.InspectFile(s.options.ConfigPath)
	if err != nil {
		return "", err
	}
	return codexResponsesModelID(summary, "")
}

func codexResponsesModelID(summary config.FileSummary, preferredModel string) (string, error) {
	providerProfiles := map[string]string{}
	for _, provider := range summary.Providers {
		providerProfiles[strings.TrimSpace(provider.Name)] = strings.TrimSpace(provider.Profile)
	}

	preferredModel = strings.TrimSpace(preferredModel)
	if preferredModel != "" {
		for _, route := range summary.Routes {
			if strings.TrimSpace(route.DesktopID) == preferredModel {
				return validateCodexResponsesRoute(route, providerProfiles)
			}
		}
		return "", fmt.Errorf("Codex model %q is not configured in gateway routes", preferredModel)
	}

	for _, route := range summary.Routes {
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID == codexapp.DefaultModel {
			return validateCodexResponsesRoute(route, providerProfiles)
		}
	}
	var firstCodexErr error
	for _, route := range summary.Routes {
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID != "" && !isClaudeDesktopModelID(desktopID) {
			modelID, err := validateCodexResponsesRoute(route, providerProfiles)
			if err == nil {
				return modelID, nil
			}
			if firstCodexErr == nil {
				firstCodexErr = err
			}
		}
	}
	if firstCodexErr != nil {
		return "", firstCodexErr
	}
	return "", fmt.Errorf("gateway config has no Codex route using provider profile %q", "responses")
}

func validateCodexResponsesRoute(route config.RouteSummary, providerProfiles map[string]string) (string, error) {
	desktopID := strings.TrimSpace(route.DesktopID)
	providerName := strings.TrimSpace(route.Provider)
	profile := strings.TrimSpace(providerProfiles[providerName])
	if profile != "responses" {
		return "", fmt.Errorf("Codex model %q uses provider %q with profile %q; expected provider profile %q", desktopID, providerName, profile, "responses")
	}
	return desktopID, nil
}
