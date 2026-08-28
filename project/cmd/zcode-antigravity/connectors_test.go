package main

import (
	"strings"
	"testing"
)

func TestPreferredConnectorModelHonorsConfiguredBackgroundModel(t *testing.T) {
	models := []modelInfo{{ID: "gemini-3.7-flash"}, {ID: "gemini-3.6-flash"}}
	if got := preferredConnectorModel("antigravity", models, "gemini-3.6-flash"); got != "gemini-3.6-flash" {
		t.Fatalf("preferred model = %q", got)
	}
	if got := preferredConnectorModel("xai", []modelInfo{{ID: "grok-build-0.1"}}, "gemini-3.6-flash"); got != "grok-build-0.1" {
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
	for _, expected := range []string{"zcode-bridge", "openai-completions", "http://127.0.0.1:18080/v1", "input: [text, image]"} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("DeepSeek Harness snippet missing %q: %s", expected, settings)
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
