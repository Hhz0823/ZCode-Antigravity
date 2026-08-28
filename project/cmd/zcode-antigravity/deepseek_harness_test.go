package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigureDeepSeekHarnessMergesBacksUpAndUpdates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DSH_HOME", home)
	settingsPath := filepath.Join(home, "settings.yaml")
	credentialsPath := filepath.Join(home, ".credentials.yaml")
	settingsBefore := "# keep this comment\nui-local:\n  theme: dark\nllm-pi-ai:\n  providers:\n    existing:\n      api: openai-completions\n      baseURL: https://example.invalid/v1\n      models:\n        - id: keep-model\nagent-default-model:\n  provider: existing\n  model: keep-model\n"
	credentialsBefore := "version: 1\nrefs:\n  KEEP_KEY: keep-me\nrecords: {}\n"
	if err := os.WriteFile(settingsPath, []byte(settingsBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsPath, []byte(credentialsBefore), 0o600); err != nil {
		t.Fatal(err)
	}

	a := testApp(t)
	result, err := a.configureDeepSeekHarness("http://127.0.0.1:18080", "gemini-3.7-flash", true)
	if err != nil {
		t.Fatal(err)
	}
	if result.SettingsBackup == "" || result.CredentialsBackup == "" {
		t.Fatalf("missing backups: %#v", result)
	}
	for _, path := range []string{result.SettingsBackup, result.CredentialsBackup} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("backup %s: %v", path, err)
		}
	}
	settingsRaw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settingsRaw), "# keep this comment") {
		t.Fatal("unrelated YAML comment was not preserved")
	}
	settings := decodeYAMLMap(t, settingsRaw)
	llm := settings["llm-pi-ai"].(map[string]any)
	providers := llm["providers"].(map[string]any)
	if _, ok := providers["existing"]; !ok {
		t.Fatal("existing provider was removed")
	}
	bridge := providers[deepSeekHarnessProviderID].(map[string]any)
	if bridge["displayName"] != "Google" || bridge["baseURL"] != "http://127.0.0.1:18080/v1" || bridge["apiKeyEnv"] != deepSeekHarnessCredentialRef {
		t.Fatalf("bridge = %#v", bridge)
	}
	models := bridge["models"].([]any)
	model := models[0].(map[string]any)
	if got := model["input"].([]any); len(got) != 2 || got[1] != "image" {
		t.Fatalf("input = %#v", got)
	}
	selection := settings["agent-default-model"].(map[string]any)
	if selection["provider"] != deepSeekHarnessProviderID || selection["model"] != "gemini-3.7-flash" {
		t.Fatalf("selection = %#v", selection)
	}
	credentialsRaw, err := os.ReadFile(credentialsPath)
	if err != nil {
		t.Fatal(err)
	}
	credentials := decodeYAMLMap(t, credentialsRaw)
	refs := credentials["refs"].(map[string]any)
	if refs["KEEP_KEY"] != "keep-me" || refs[deepSeekHarnessCredentialRef] != a.apiKey {
		t.Fatalf("refs = %#v", refs)
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{settingsPath, credentialsPath} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat %s: %v", path, err)
			}
			if info.Mode().Perm()&0o077 != 0 {
				t.Fatalf("%s mode = %v", path, info.Mode().Perm())
			}
		}
	}

	if _, err := a.configureDeepSeekHarness("http://localhost:18081/", "grok-4.6", false); err != nil {
		t.Fatal(err)
	}
	settings = decodeYAMLMap(t, mustReadFile(t, settingsPath))
	bridge = settings["llm-pi-ai"].(map[string]any)["providers"].(map[string]any)[deepSeekHarnessProviderID].(map[string]any)
	model = bridge["models"].([]any)[0].(map[string]any)
	if bridge["baseURL"] != "http://localhost:18081/v1" || model["id"] != "grok-4.6" || len(model["input"].([]any)) != 1 {
		t.Fatalf("updated bridge = %#v", bridge)
	}
}

func TestConfigureDeepSeekHarnessRefusesForeignProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DSH_HOME", home)
	path := filepath.Join(home, "settings.yaml")
	original := []byte("llm-pi-ai:\n  providers:\n    zcode-bridge:\n      displayName: Somebody Else\n      api: openai-completions\n      baseURL: https://example.com/v1\n      models:\n        - id: custom\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	if _, err := a.configureDeepSeekHarness("http://127.0.0.1:18080", "gemini-3.7-flash", true); err == nil {
		t.Fatal("expected foreign provider collision")
	}
	if after := mustReadFile(t, path); string(after) != string(original) {
		t.Fatal("foreign provider file changed after refusal")
	}
}

func decodeYAMLMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := yaml.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
