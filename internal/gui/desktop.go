package gui

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/claudedesktop"
	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/localenv"
)

type ClaudeDesktopApplyResult struct {
	ProfileID       string   `json:"profileId"`
	ProfilePath     string   `json:"profilePath"`
	BaseURL         string   `json:"baseUrl"`
	ModelIDs        []string `json:"modelIds"`
	RestartRequired bool     `json:"restartRequired"`
}

func (s *Service) ApplyClaudeDesktopConfig() (ClaudeDesktopApplyResult, error) {
	if err := s.ensureDefaultConfig(); err != nil {
		return ClaudeDesktopApplyResult{}, err
	}
	gatewayAPIKey, err := s.gatewayAPIKeyForRepair()
	if err != nil {
		return ClaudeDesktopApplyResult{}, err
	}

	modelIDs, err := s.desktopModelIDs()
	if err != nil {
		return ClaudeDesktopApplyResult{}, err
	}
	baseURL := strings.TrimRight(s.options.ExpectedBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8787"
	}

	result, err := claudedesktop.ApplyLocal(claudedesktop.ApplyOptions{
		Paths:         s.options.DesktopPaths,
		BaseURL:       baseURL,
		GatewayAPIKey: gatewayAPIKey,
		ModelIDs:      modelIDs,
	})
	if err != nil {
		return ClaudeDesktopApplyResult{}, err
	}
	return ClaudeDesktopApplyResult{
		ProfileID:       result.ProfileID,
		ProfilePath:     result.ProfilePath,
		BaseURL:         result.BaseURL,
		ModelIDs:        append([]string(nil), result.ModelIDs...),
		RestartRequired: true,
	}, nil
}

func (s *Service) gatewayAPIKeyForRepair() (string, error) {
	gatewayAPIKey, err := localenv.SecretValue(s.options.EnvPath, "CLAUDE_GATEWAY_API_KEY")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(gatewayAPIKey) != "" {
		return gatewayAPIKey, nil
	}

	generated, err := generateGatewayAPIKey()
	if err != nil {
		return "", err
	}
	if err := localenv.SaveSecret(s.options.EnvPath, "CLAUDE_GATEWAY_API_KEY", generated); err != nil {
		return "", err
	}
	return generated, nil
}

func generateGatewayAPIKey() (string, error) {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "cdg_" + base64.RawURLEncoding.EncodeToString(random[:]), nil
}

func (s *Service) desktopModelIDs() ([]string, error) {
	summary, err := config.InspectFile(s.options.ConfigPath)
	if err != nil {
		return nil, err
	}
	modelIDs := make([]string, 0, len(summary.Routes))
	for _, route := range summary.Routes {
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID != "" {
			modelIDs = append(modelIDs, desktopID)
		}
	}
	if len(modelIDs) == 0 {
		return claudedesktop.DefaultModelIDs(), nil
	}
	return modelIDs, nil
}
