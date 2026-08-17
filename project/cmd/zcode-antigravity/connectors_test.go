package main

import "testing"

func TestPreferredConnectorModelHonorsConfiguredBackgroundModel(t *testing.T) {
	models := []modelInfo{{ID: "gemini-3.7-flash"}, {ID: "gemini-3.6-flash"}}
	if got := preferredConnectorModel("antigravity", models, "gemini-3.6-flash"); got != "gemini-3.6-flash" {
		t.Fatalf("preferred model = %q", got)
	}
	if got := preferredConnectorModel("xai", []modelInfo{{ID: "grok-build-0.1"}}, "gemini-3.6-flash"); got != "grok-build-0.1" {
		t.Fatalf("xai preferred model = %q", got)
	}
}
