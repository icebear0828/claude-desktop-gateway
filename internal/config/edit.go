package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type EditableFile struct {
	Path             string             `json:"path"`
	Host             string             `json:"host"`
	Port             int                `json:"port"`
	GatewayAPIKeyEnv string             `json:"gatewayApiKeyEnv"`
	TLSCertFile      string             `json:"tlsCertFile"`
	TLSKeyFile       string             `json:"tlsKeyFile"`
	Providers        []EditableProvider `json:"providers"`
	Routes           []EditableRoute    `json:"routes"`
}

type EditableProvider struct {
	Name         string               `json:"name"`
	Profile      string               `json:"profile"`
	BaseURL      string               `json:"baseUrl"`
	APIKeyEnv    string               `json:"apiKeyEnv"`
	Referrer     string               `json:"referrer"`
	Title        string               `json:"title"`
	Capabilities ProviderCapabilities `json:"capabilities"`
}

type EditableRoute struct {
	DesktopID         string            `json:"desktopID"`
	Provider          string            `json:"provider"`
	UpstreamModel     string            `json:"upstreamModel"`
	DisplayName       string            `json:"displayName"`
	DynamicFreeModels DynamicFreeModels `json:"dynamicFreeModels"`
	Cache             RouteCache        `json:"cache"`
}

func NewEditableRoute(provider string, upstreamModel string, displayName string) EditableRoute {
	desktopID := DefaultDesktopModelID(upstreamModel)
	trimmedDisplayName := strings.TrimSpace(displayName)
	if trimmedDisplayName == "" {
		trimmedDisplayName = desktopID
	}
	return EditableRoute{
		DesktopID:     desktopID,
		Provider:      strings.TrimSpace(provider),
		UpstreamModel: strings.TrimSpace(upstreamModel),
		DisplayName:   trimmedDisplayName,
	}
}

func LoadEditableFile(path string) (EditableFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EditableFile{}, fmt.Errorf("read config file: %w", err)
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return EditableFile{}, fmt.Errorf("parse config file: %w", err)
	}
	editable, err := editableFromRaw(path, raw)
	if err != nil {
		return EditableFile{}, err
	}
	return editable, nil
}

func SaveEditableFile(path string, editable EditableFile) (FileSummary, error) {
	if strings.TrimSpace(path) == "" {
		path = editable.Path
	}
	if strings.TrimSpace(path) == "" {
		return FileSummary{}, errors.New("config path is required")
	}

	raw, err := editableToRaw(editable)
	if err != nil {
		return FileSummary{}, err
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return FileSummary{}, fmt.Errorf("encode config file: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return FileSummary{}, fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return FileSummary{}, fmt.Errorf("write config file: %w", err)
	}
	return InspectFile(path)
}

func editableFromRaw(path string, raw fileConfig) (EditableFile, error) {
	if strings.TrimSpace(raw.GatewayAPIKey) != "" {
		return EditableFile{}, errors.New("gatewayApiKey is not allowed in config files; use gatewayApiKeyEnv")
	}

	editable := EditableFile{
		Path:             path,
		Host:             valueOrDefault(raw.Host, DefaultHost),
		Port:             raw.Port,
		GatewayAPIKeyEnv: strings.TrimSpace(raw.GatewayAPIKeyEnv),
		TLSCertFile:      strings.TrimSpace(raw.TLSCertFile),
		TLSKeyFile:       strings.TrimSpace(raw.TLSKeyFile),
	}
	if editable.Port <= 0 || editable.Port > 65535 {
		editable.Port = DefaultPort
	}

	providerNames := make([]string, 0, len(raw.Providers))
	for name := range raw.Providers {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			providerNames = append(providerNames, trimmed)
		}
	}
	sort.Strings(providerNames)
	for _, name := range providerNames {
		provider := raw.Providers[name]
		if strings.TrimSpace(provider.APIKey) != "" {
			return EditableFile{}, fmt.Errorf("provider %q apiKey is not allowed in config files; use apiKeyEnv", name)
		}
		editable.Providers = append(editable.Providers, EditableProvider{
			Name:         name,
			Profile:      valueOrDefault(provider.Profile, "openai-chat"),
			BaseURL:      strings.TrimRight(valueOrDefault(provider.BaseURL, DefaultOpenRouterURL), "/"),
			APIKeyEnv:    strings.TrimSpace(provider.APIKeyEnv),
			Referrer:     strings.TrimSpace(provider.Referrer),
			Title:        valueOrDefault(provider.Title, DefaultTitle),
			Capabilities: capabilitiesFromFile(provider.Capabilities),
		})
	}
	if len(editable.Providers) == 0 {
		return EditableFile{}, errors.New("at least one provider is required")
	}

	routes := raw.Routes
	if len(routes) == 0 {
		defaultModel := strings.TrimSpace(raw.DefaultModel)
		if defaultModel == "" {
			defaultModel = DefaultAliasModel
		}
		routes = fileRoutesFromRoutes(routesFromAliases(parseAliasEnv("", defaultModel)))
	}
	aliases := make([]string, 0, len(routes))
	for alias := range routes {
		if trimmed := strings.TrimSpace(alias); trimmed != "" {
			aliases = append(aliases, trimmed)
		}
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		for _, route := range routes[alias] {
			upstreamModel := strings.TrimSpace(route.Model)
			provider := strings.TrimSpace(route.Provider)
			if provider == "" || upstreamModel == "" {
				continue
			}
			desktopID := alias
			if desktopID == upstreamModel {
				desktopID = DefaultDesktopModelID(upstreamModel)
			}
			displayName := strings.TrimSpace(route.DisplayName)
			if displayName == "" {
				displayName = desktopID
			}
			editable.Routes = append(editable.Routes, EditableRoute{
				DesktopID:         desktopID,
				Provider:          provider,
				UpstreamModel:     upstreamModel,
				DisplayName:       displayName,
				DynamicFreeModels: dynamicFreeModelsFromFile(route.DynamicFreeModels),
				Cache:             routeCacheFromFile(route.Cache),
			})
		}
	}
	if len(editable.Routes) == 0 {
		return EditableFile{}, errors.New("at least one route is required")
	}
	if _, err := editableToRaw(editable); err != nil {
		return EditableFile{}, err
	}
	return editable, nil
}

func editableToRaw(editable EditableFile) (fileConfig, error) {
	host := valueOrDefault(editable.Host, DefaultHost)
	port := editable.Port
	if port <= 0 || port > 65535 {
		port = DefaultPort
	}

	raw := fileConfig{
		Host:             host,
		Port:             port,
		GatewayAPIKeyEnv: strings.TrimSpace(editable.GatewayAPIKeyEnv),
		TLSCertFile:      strings.TrimSpace(editable.TLSCertFile),
		TLSKeyFile:       strings.TrimSpace(editable.TLSKeyFile),
		Providers:        map[string]fileProvider{},
		Routes:           map[string][]fileRoute{},
	}

	knownProviders := map[string]bool{}
	for _, provider := range editable.Providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			return fileConfig{}, errors.New("provider name is required")
		}
		if knownProviders[name] {
			return fileConfig{}, fmt.Errorf("provider %q is duplicated", name)
		}
		baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
		if err := validateHTTPURL(baseURL, fmt.Sprintf("provider %q baseUrl", name)); err != nil {
			return fileConfig{}, err
		}
		apiKeyEnv := strings.TrimSpace(provider.APIKeyEnv)
		if apiKeyEnv == "" {
			return fileConfig{}, fmt.Errorf("provider %q apiKeyEnv is required", name)
		}
		capabilities := provider.Capabilities
		if capabilities == (ProviderCapabilities{}) {
			capabilities = DefaultProviderCapabilities()
		}
		raw.Providers[name] = fileProvider{
			Profile:      valueOrDefault(provider.Profile, "openai-chat"),
			BaseURL:      baseURL,
			APIKeyEnv:    apiKeyEnv,
			Referrer:     strings.TrimSpace(provider.Referrer),
			Title:        valueOrDefault(provider.Title, DefaultTitle),
			Capabilities: fileCapabilitiesFromProvider(capabilities),
		}
		knownProviders[name] = true
	}
	if len(raw.Providers) == 0 {
		return fileConfig{}, errors.New("at least one provider is required")
	}

	knownDesktopIDs := map[string]bool{}
	for _, route := range editable.Routes {
		upstreamModel := strings.TrimSpace(route.UpstreamModel)
		if upstreamModel == "" {
			return fileConfig{}, errors.New("route upstream model is required")
		}
		desktopID := strings.TrimSpace(route.DesktopID)
		if desktopID == "" {
			desktopID = DefaultDesktopModelID(upstreamModel)
		}
		if knownDesktopIDs[desktopID] {
			return fileConfig{}, fmt.Errorf("route %q is duplicated", desktopID)
		}
		provider := strings.TrimSpace(route.Provider)
		if provider == "" {
			provider = DefaultOpenRouterName
		}
		if !knownProviders[provider] {
			return fileConfig{}, fmt.Errorf("route %q references unknown provider %q", desktopID, provider)
		}
		raw.Routes[desktopID] = []fileRoute{{
			Provider:          provider,
			Model:             upstreamModel,
			DisplayName:       strings.TrimSpace(route.DisplayName),
			DynamicFreeModels: fileDynamicFreeModelsFromRoute(route.DynamicFreeModels),
			Cache:             fileRouteCacheFromRoute(route.Cache),
		}}
		knownDesktopIDs[desktopID] = true
	}
	if len(raw.Routes) == 0 {
		return fileConfig{}, errors.New("at least one route is required")
	}
	return raw, nil
}

func validateHTTPURL(value string, field string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute URL", field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", field)
	}
	return nil
}

func fileCapabilitiesFromProvider(capabilities ProviderCapabilities) *fileCapabilities {
	return &fileCapabilities{
		Streaming: boolPointer(capabilities.Streaming),
		Tools:     boolPointer(capabilities.Tools),
		JSONMode:  boolPointer(capabilities.JSONMode),
	}
}

func boolPointer(value bool) *bool {
	return &value
}
