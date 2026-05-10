package claudedesktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultProfileID   = "00000000-0000-4000-8000-000000087870"
	DefaultProfileName = "Claude Desktop Gateway"
)

type ApplyOptions struct {
	Paths         Paths
	ProfileID     string
	ProfileName   string
	BaseURL       string
	GatewayAPIKey string
	ModelIDs      []string
}

type ApplyResult struct {
	Paths       Paths
	ProfileID   string
	ProfilePath string
	BaseURL     string
	ModelIDs    []string
}

type fileSnapshot struct {
	path    string
	content []byte
	exists  bool
}

type jsonObjectWriter func(path string, obj jsonObject) error

func DefaultModelIDs() []string {
	return []string{
		"claude-sonnet-4.6",
		"claude-opus-4.7",
		"claude-haiku-4.5",
		"claude-sonnet-4-6",
		"claude-opus-4-7",
		"claude-haiku-4-5",
	}
}

func ApplyLocal(options ApplyOptions) (ApplyResult, error) {
	return applyLocalWithWriter(options, writeJSONObject)
}

func applyLocalWithWriter(options ApplyOptions, writeJSON jsonObjectWriter) (ApplyResult, error) {
	paths := options.Paths
	if paths.ConfigLibraryPath == "" {
		var err error
		paths, err = CurrentPlatformPaths()
		if err != nil {
			return ApplyResult{}, err
		}
	}

	profileID := valueOrDefault(options.ProfileID, DefaultProfileID)
	profileName := valueOrDefault(options.ProfileName, DefaultProfileName)
	baseURL := valueOrDefault(options.BaseURL, "http://127.0.0.1:8787")
	if err := validateGatewayBaseURL(baseURL); err != nil {
		return ApplyResult{}, err
	}
	gatewayAPIKey := strings.TrimSpace(options.GatewayAPIKey)
	if gatewayAPIKey == "" {
		return ApplyResult{}, errors.New("gateway API key is required")
	}
	modelIDs := options.ModelIDs
	if len(modelIDs) == 0 {
		modelIDs = DefaultModelIDs()
	}

	targetProfilePath := filepath.Join(paths.ConfigLibraryPath, profileID+".json")
	result := ApplyResult{
		Paths:       paths,
		ProfileID:   profileID,
		ProfilePath: targetProfilePath,
		BaseURL:     baseURL,
		ModelIDs:    append([]string(nil), modelIDs...),
	}

	snapshots, err := snapshotFiles([]string{
		paths.NormalConfigPath,
		paths.ThreePConfigPath,
		paths.MetaPath,
		targetProfilePath,
	})
	if err != nil {
		return ApplyResult{}, err
	}

	if err := applyLocalInner(paths, profileID, profileName, baseURL, gatewayAPIKey, modelIDs, targetProfilePath, writeJSON); err != nil {
		if rollbackErr := restoreFileSnapshots(snapshots); rollbackErr != nil {
			return ApplyResult{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return ApplyResult{}, err
	}

	return result, nil
}

func FormatApplyResult(result ApplyResult) string {
	var b strings.Builder
	b.WriteString("Claude Desktop local gateway profile\n")
	writeLine(&b, "profile id", result.ProfileID)
	writeLine(&b, "profile path", result.ProfilePath)
	writeLine(&b, "base url", result.BaseURL)
	if len(result.ModelIDs) > 0 {
		fmt.Fprintf(&b, "models: %s\n", strings.Join(result.ModelIDs, ", "))
	}
	b.WriteString("gateway api key: <redacted>\n")
	return b.String()
}

func applyLocalInner(paths Paths, profileID string, profileName string, baseURL string, gatewayAPIKey string, modelIDs []string, targetProfilePath string, writeJSON jsonObjectWriter) error {
	sourceProfile := readActiveProfile(paths)
	profile, err := buildLocalProfile(sourceProfile, baseURL, gatewayAPIKey, modelIDs)
	if err != nil {
		return err
	}
	meta := buildMeta(paths.MetaPath, profileID, profileName)

	if err := writeDeploymentMode(paths.NormalConfigPath, "3p", writeJSON); err != nil {
		return err
	}
	if err := writeDeploymentMode(paths.ThreePConfigPath, "3p", writeJSON); err != nil {
		return err
	}
	if err := writeJSON(targetProfilePath, profile); err != nil {
		return err
	}
	if err := writeJSON(paths.MetaPath, meta); err != nil {
		return err
	}
	return nil
}

func buildLocalProfile(source jsonObject, baseURL string, gatewayAPIKey string, modelIDs []string) (jsonObject, error) {
	profile := jsonObject{}
	for key, value := range source {
		if isGatewayProfileField(key) || key == "enterpriseConfig" {
			continue
		}
		profile[key] = append(json.RawMessage(nil), value...)
	}

	modelBytes, err := json.Marshal(modelIDs)
	if err != nil {
		return nil, fmt.Errorf("encode model IDs: %w", err)
	}

	setString(profile, "inferenceProvider", "gateway")
	setString(profile, "inferenceGatewayBaseUrl", baseURL)
	setString(profile, "inferenceGatewayApiKey", gatewayAPIKey)
	setString(profile, "inferenceGatewayAuthScheme", "bearer")
	setString(profile, "inferenceModels", string(modelBytes))
	if stringField(profile, "disableDeploymentModeChooser") == "" {
		setString(profile, "disableDeploymentModeChooser", "true")
	}
	return profile, nil
}

func buildMeta(path string, profileID string, profileName string) jsonObject {
	meta, exists, err := readJSONObject(path)
	if err != nil || !exists {
		meta = jsonObject{}
	}

	var entries []jsonObject
	if raw, ok := meta["entries"]; ok {
		_ = json.Unmarshal(raw, &entries)
	}

	filtered := make([]jsonObject, 0, len(entries)+1)
	for _, entry := range entries {
		if stringField(entry, "id") == profileID {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, jsonObject{
		"id":   mustStringJSON(profileID),
		"name": mustStringJSON(profileName),
	})

	entriesRaw, _ := json.Marshal(filtered)
	meta["entries"] = entriesRaw
	setString(meta, "appliedId", profileID)
	return meta
}

func readActiveProfile(paths Paths) jsonObject {
	meta, exists, err := readJSONObject(paths.MetaPath)
	if err != nil || !exists {
		return jsonObject{}
	}
	appliedID := stringField(meta, "appliedId")
	if appliedID == "" {
		return jsonObject{}
	}
	profile, exists, err := readJSONObject(filepath.Join(paths.ConfigLibraryPath, appliedID+".json"))
	if err != nil || !exists {
		return jsonObject{}
	}
	return profile
}

func writeDeploymentMode(path string, mode string, writeJSON jsonObjectWriter) error {
	obj, exists, err := readJSONObject(path)
	if err != nil || !exists {
		obj = jsonObject{}
	}
	setString(obj, "deploymentMode", mode)
	return writeJSON(path, obj)
}

func writeJSONObject(path string, obj jsonObject) error {
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON %s: %w", path, err)
	}
	data = append(data, '\n')
	return atomicWriteFile(path, data)
}

func atomicWriteFile(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory %s: %w", parent, err)
	}
	tmp, err := os.CreateTemp(parent, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func snapshotFiles(paths []string) ([]fileSnapshot, error) {
	snapshots := make([]fileSnapshot, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				snapshots = append(snapshots, fileSnapshot{path: path})
				continue
			}
			return nil, fmt.Errorf("snapshot %s: %w", path, err)
		}
		snapshots = append(snapshots, fileSnapshot{
			path:    path,
			content: data,
			exists:  true,
		})
	}
	return snapshots, nil
}

func restoreFileSnapshots(snapshots []fileSnapshot) error {
	for _, snapshot := range snapshots {
		if !snapshot.exists {
			if err := os.Remove(snapshot.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove %s: %w", snapshot.path, err)
			}
			continue
		}
		if err := atomicWriteFile(snapshot.path, snapshot.content); err != nil {
			return err
		}
	}
	return nil
}

func validateGatewayBaseURL(raw string) error {
	var report Report
	checkBaseURL(&report, "", raw, "")
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			return errors.New(issue.Message)
		}
	}
	return nil
}

func isGatewayProfileField(key string) bool {
	switch key {
	case "inferenceProvider", "inferenceGatewayBaseUrl", "inferenceGatewayApiKey", "inferenceGatewayAuthScheme", "inferenceModels":
		return true
	default:
		return false
	}
}

func setString(obj jsonObject, key string, value string) {
	obj[key] = mustStringJSON(value)
}

func mustStringJSON(value string) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func valueOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
