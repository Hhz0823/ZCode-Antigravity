package main

import (
	"strings"
	"testing"
)

func TestPreferredConnectorModelHonorsConfiguredBackgroundModel(t *testing.T) {
	models := []modelInfo{{ID: "gemini-3.8-flash"}, {ID: "gemini-3.7-flash"}, {ID: "gemini-3.6-flash"}}
	if got := preferredConnectorModel("antigravity", models, "gemini-3.6-flash"); got != "gemini-3.6-flash" {
		t.Fatalf("preferred model = %q", got)
	}
	if got := preferredConnectorModel("antigravity", models, ""); got != "gemini-3.8-flash" {
		t.Fatalf("default preferred model = %q, want gemini-3.8-flash", got)
	}
	if got := preferredConnectorModel("antigravity", models[1:], ""); got != "gemini-3.7-flash" {
		t.Fatalf("fallback preferred model = %q, want gemini-3.7-flash", got)
	}
	if got := preferredConnectorModel("xai", []modelInfo{{ID: "grok-build-0.1"}}, "gemini-3.8-flash"); got != "grok-build-0.1" {
		t.Fatalf("xai preferred model = %q", got)
	}
}

func TestAgentConnectorsIncludeOneClickDeepSeekHarness(t *testing.T) {
	connectors := buildAgentConnectors("http://127.0.0.1:18080", "local-key", "gemini-3.7-flash", "antigravity")
	if len(connectors) == 0 || connectors[0].ID != "deepseek-harness" || connectors[0].Action != connectorAction("deepseek-harness") {
		t.Fatalf("first connector = %#v", connectors)
	}
	for _, connector := range connectors[:len(connectors)-1] {
		if connector.Action == "" {
			t.Fatalf("%s is missing one-click action", connector.ID)
		}
	}
	settings := connectors[0].Snippets["$DSH_HOME/settings.yaml"]
	for _, expected := range []string{"zcode-bridge", "displayName: Google", "openai-completions", "http://127.0.0.1:18080/v1", "input: [text, image]"} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("DeepSeek Harness snippet missing %q: %s", expected, settings)
		}
	}
	configSnippet := map[string]string{"codex": "~/.codex/config.toml", "opencode": "~/.config/opencode/opencode.json"}
	for id, path := range configSnippet {
		for _, connector := range connectors {
			if connector.ID == id {
				snippet := connector.Snippets[path]
				if !strings.Contains(snippet, "Google") || strings.Contains(snippet, "ZCode Local Bridge") {
					t.Fatalf("%s provider display name is not Google: %s", id, snippet)
				}
			}
		}
	}
	grok := buildAgentConnectors("http://127.0.0.1:18080", "local-key", "grok-4.6", "xai")[0]
	if strings.Contains(grok.Snippets["$DSH_HOME/settings.yaml"], "input: [text, image]") {
		t.Fatal("Grok connector must not claim stable image input")
	}
	for _, connector := range buildAgentConnectors("http://127.0.0.1:18080", "local-key", "grok-4.6", "xai") {
		if connector.ID == "gemini-cli" && connector.Action != "" {
			t.Fatal("Gemini CLI native connector must not be enabled for Grok")
		}
	}
}
