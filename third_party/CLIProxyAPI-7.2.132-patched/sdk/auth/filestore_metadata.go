package auth

import (
	"encoding/json"
	"fmt"
	"strings"
)

var antigravitySensitiveFields = []string{"access_token", "refresh_token"}

func marshalAuthMetadataForStorage(metadata map[string]any) ([]byte, error) {
	stored := cloneMetadataMap(metadata)
	provider, _ := stored["type"].(string)
	if strings.EqualFold(strings.TrimSpace(provider), "antigravity") {
		if err := transformSensitiveFields(stored, protectCredentialString); err != nil {
			return nil, err
		}
	}
	return json.Marshal(stored)
}

func unmarshalAuthMetadataFromStorage(data []byte) (map[string]any, error) {
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	provider, _ := metadata["type"].(string)
	if strings.EqualFold(strings.TrimSpace(provider), "antigravity") {
		if err := transformSensitiveFields(metadata, unprotectCredentialString); err != nil {
			return nil, err
		}
	}
	return metadata, nil
}

// UnmarshalStoredMetadata decodes auth metadata exactly as FileTokenStore persists it.
// On Windows and macOS this also unprotects platform-backed Antigravity credentials
// for the current user so runtime consumers never mistake ciphertext for an access token.
func UnmarshalStoredMetadata(data []byte) (map[string]any, error) {
	return unmarshalAuthMetadataFromStorage(data)
}

func authMetadataStorageNeedsMigration(data []byte) (bool, error) {
	stored := make(map[string]any)
	if err := json.Unmarshal(data, &stored); err != nil {
		return false, err
	}
	provider, _ := stored["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(provider), "antigravity") {
		return false, nil
	}
	for _, key := range antigravitySensitiveFields {
		value, _ := stored[key].(string)
		if credentialStringNeedsProtection(value) {
			return true, nil
		}
	}
	return false, nil
}

func transformSensitiveFields(metadata map[string]any, transform func(string) (string, error)) error {
	for _, key := range antigravitySensitiveFields {
		value, ok := metadata[key].(string)
		if !ok || value == "" {
			continue
		}
		updated, err := transform(value)
		if err != nil {
			return fmt.Errorf("auth filestore: transform %s: %w", key, err)
		}
		metadata[key] = updated
	}
	return nil
}

func cloneMetadataMap(metadata map[string]any) map[string]any {
	copyMap := make(map[string]any, len(metadata))
	for key, value := range metadata {
		copyMap[key] = cloneMetadataValue(value)
	}
	return copyMap
}

func cloneMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMetadataMap(typed)
	case []any:
		copySlice := make([]any, len(typed))
		for i, entry := range typed {
			copySlice[i] = cloneMetadataValue(entry)
		}
		return copySlice
	default:
		return value
	}
}
