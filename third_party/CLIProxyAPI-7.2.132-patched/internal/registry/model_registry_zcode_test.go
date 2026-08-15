package registry

import "testing"

func TestClaudeCatalogIncludesZCodeCapabilities(t *testing.T) {
	const clientID = "zcode-capability-test-client"
	const modelID = "zcode-capability-test-model"
	registry := GetGlobalRegistry()
	registry.RegisterClient(clientID, "antigravity", []*ModelInfo{{
		ID:                        modelID,
		DisplayName:               "ZCode Capability Test",
		ContextLength:             1048576,
		MaxCompletionTokens:       65536,
		Thinking:                  &ThinkingSupport{Min: 1, Max: 65535, DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high"}},
		SupportedInputModalities:  []string{"text", "image", "audio", "video"},
		SupportedOutputModalities: []string{"text"},
	}})
	t.Cleanup(func() { registry.UnregisterClient(clientID) })

	for _, model := range registry.GetAvailableModels("claude") {
		if model["id"] != modelID {
			continue
		}
		if model["max_input_tokens"] != 1048576 || model["max_tokens"] != 65536 {
			t.Fatalf("unexpected token limits: %+v", model)
		}
		inputs, ok := model["supportedInputModalities"].([]string)
		if !ok || len(inputs) != 4 {
			t.Fatalf("unexpected input modalities: %#v", model["supportedInputModalities"])
		}
		thinking, ok := model["thinking"].(map[string]any)
		if !ok {
			t.Fatalf("unexpected thinking metadata: %#v", model["thinking"])
		}
		levels, ok := thinking["levels"].([]string)
		if !ok || len(levels) != 4 || levels[1] != "low" || levels[3] != "high" {
			t.Fatalf("unexpected thinking levels: %#v", thinking["levels"])
		}
		if thinking["min"] != 1 || thinking["max"] != 65535 || thinking["dynamic_allowed"] != true {
			t.Fatalf("unexpected thinking range: %#v", thinking)
		}
		return
	}
	t.Fatalf("model %q not found", modelID)
}
