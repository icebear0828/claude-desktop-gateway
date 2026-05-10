package gui

import (
	"errors"
	"strings"

	"github.com/local/claude-desktop-gateway/internal/config"
	"github.com/local/claude-desktop-gateway/internal/localenv"
)

var allowedSecretNames = []string{
	"OPENROUTER_API_KEY",
	"CLAUDE_GATEWAY_API_KEY",
}

type EditorState struct {
	Config  config.EditableFile  `json:"config"`
	Secrets []localenv.SecretEnv `json:"secrets"`
	EnvPath string               `json:"envPath"`
}

type SecretInput struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type SecretNameInput struct {
	Name string `json:"name"`
}

func (s *Service) Editor() (EditorState, error) {
	if err := s.ensureDefaultConfig(); err != nil {
		return EditorState{}, err
	}
	editable, err := config.LoadEditableFile(s.options.ConfigPath)
	if err != nil {
		return EditorState{}, err
	}
	secrets, err := localenv.SecretStatus(s.options.EnvPath, allowedSecretNames)
	if err != nil {
		return EditorState{}, err
	}
	return EditorState{
		Config:  editable,
		Secrets: secrets,
		EnvPath: s.options.EnvPath,
	}, nil
}

func (s *Service) SaveConfig(editable config.EditableFile) (EditorState, error) {
	if _, err := config.SaveEditableFile(s.options.ConfigPath, editable); err != nil {
		return EditorState{}, err
	}
	return s.Editor()
}

func (s *Service) SaveSecret(input SecretInput) (EditorState, error) {
	name := strings.TrimSpace(input.Name)
	if !allowedSecret(name) {
		return EditorState{}, errors.New("secret name is not allowed")
	}
	if err := localenv.SaveSecret(s.options.EnvPath, name, input.Value); err != nil {
		return EditorState{}, err
	}
	return s.Editor()
}

func (s *Service) DeleteSecret(input SecretNameInput) (EditorState, error) {
	name := strings.TrimSpace(input.Name)
	if !allowedSecret(name) {
		return EditorState{}, errors.New("secret name is not allowed")
	}
	if err := localenv.DeleteSecret(s.options.EnvPath, name); err != nil {
		return EditorState{}, err
	}
	return s.Editor()
}

func (s *Service) SuggestedDesktopID(upstreamModel string) string {
	return config.DefaultDesktopModelID(upstreamModel)
}

func allowedSecret(name string) bool {
	for _, allowed := range allowedSecretNames {
		if name == allowed {
			return true
		}
	}
	return false
}
