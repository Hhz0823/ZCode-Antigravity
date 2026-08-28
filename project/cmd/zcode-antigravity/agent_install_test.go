package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestOneClickAgentConfigsPreserveExistingSettings(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "claude"))
	t.Setenv("GEMINI_CLI_HOME", filepath.Join(root, "gemini-home"))
	t.Setenv("QWEN_CODE_HOME", filepath.Join(root, "qwen"))
	t.Setenv("KIMI_HOME", filepath.Join(root, "kimi"))
	t.Setenv("GROK_HOME", filepath.Join(root, "grok"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv("OPENCODE_CONFIG", filepath.Join(root, "opencode", "opencode.json"))

	seed := map[string]string{
		filepath.Join(root, "claude", "settings.json"):                 `{"permissions":{"allow":["Read"]}}`,
		filepath.Join(root, "gemini-home", ".gemini", "settings.json"): `{"ui":{"theme":"Default"}}`,
		filepath.Join(root, "gemini-home", ".gemini", ".env"):          "KEEP_ME=yes\nGEMINI_MODEL=old\n",
		filepath.Join(root, "qwen", "settings.json"):                   `{"theme":"dark"}`,
		filepath.Join(root, "opencode", "opencode.json"):               `{"instructions":["keep.md"]}`,
		filepath.Join(root, "codex", "config.toml"):                    "sandbox_mode = \"workspace-write\"\n",
		filepath.Join(root, "kimi", "config.toml"):                     "theme = \"dark\"\n",
		filepath.Join(root, "grok", "config.toml"):                     "[ui]\ntheme = \"dark\"\n",
	}
	for path, content := range seed {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	a := testApp(t)
	ctx := connectorInstallContext{
		BaseURL: "http://127.0.0.1:18080", OpenAIBaseURL: "http://127.0.0.1:18080/v1",
		APIKey: a.apiKey, Model: "gemini-3.7-flash", Provider: "antigravity", ContextWindow: 1048576,
	}
	for name, configure := range map[string]func(connectorInstallContext) error{
		"claude": a.configureClaudeCode, "gemini": a.configureGeminiCLI, "qwen": a.configureQwenCode,
		"opencode": a.configureOpenCode, "codex": a.configureCodex, "kimi": a.configureKimiCode,
		"grok": a.configureGrokBuild,
	} {
		if err := configure(ctx); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	assertJSONContains(t, seedPath(root, "claude/settings.json"), "permissions", "ANTHROPIC_BASE_URL")
	assertJSONContains(t, seedPath(root, "gemini-home/.gemini/settings.json"), "ui", "selectedType")
	assertJSONContains(t, seedPath(root, "qwen/settings.json"), "theme", "zcode-bridge")
	assertJSONContains(t, seedPath(root, "opencode/opencode.json"), "instructions", "@ai-sdk/openai-compatible", "Google")
	for _, path := range []string{"codex/config.toml", "kimi/config.toml", "grok/config.toml"} {
		raw := mustReadFile(t, seedPath(root, path))
		var parsed map[string]any
		if err := toml.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !strings.Contains(string(raw), "zcode") {
			t.Fatalf("%s missing managed provider", path)
		}
	}
	for _, path := range []string{"codex/config.toml", "grok/config.toml"} {
		if raw := string(mustReadFile(t, seedPath(root, path))); !strings.Contains(raw, "Google") || strings.Contains(raw, "ZCode Local Bridge") {
			t.Fatalf("%s provider display name is not Google: %s", path, raw)
		}
	}
	env := string(mustReadFile(t, seedPath(root, "gemini-home/.gemini/.env")))
	if !strings.Contains(env, "KEEP_ME=yes") || !strings.Contains(env, "GEMINI_MODEL=gemini-3.7-flash") {
		t.Fatalf("Gemini .env = %s", env)
	}
	if string(mustReadFile(t, seedPath(root, "opencode/zcode-bridge-key"))) != a.apiKey+"\n" {
		t.Fatal("OpenCode key file mismatch")
	}
}

func TestOneClickAgentConfigRefusesForeignManagedID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	path := filepath.Join(dir, "config.toml")
	original := "[model_providers.zcode_bridge]\nname = \"Somebody Else\"\nbase_url = \"https://example.com/v1\"\nwire_api = \"responses\"\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	a := testApp(t)
	err := a.configureCodex(connectorInstallContext{OpenAIBaseURL: "http://127.0.0.1:18080/v1", APIKey: a.apiKey, Model: "gemini-3.7-flash"})
	if err == nil {
		t.Fatal("expected foreign provider collision")
	}
	if got := string(mustReadFile(t, path)); got != original {
		t.Fatal("foreign Codex provider was modified")
	}
}

func assertJSONContains(t *testing.T, path string, keys ...string) {
	t.Helper()
	raw := mustReadFile(t, path)
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("%s missing %q", path, key)
		}
	}
}

func seedPath(root, relative string) string {
	return filepath.Join(root, filepath.FromSlash(relative))
}
