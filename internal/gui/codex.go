package gui

import (
	"context"
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
	for _, route := range summary.Routes {
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID == codexapp.DefaultModel {
			return desktopID, nil
		}
	}
	for _, route := range summary.Routes {
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID != "" && !strings.HasPrefix(desktopID, "claude-") {
			return desktopID, nil
		}
	}
	return codexapp.DefaultModel, nil
}
