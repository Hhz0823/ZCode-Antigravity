package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPersonalProviderMigratesBothSchemas(t *testing.T) {
	a := testApp(t)
	legacy := []byte(fmt.Sprintf(`{"provider":{%q:{"x-zcode-antigravity-managed":1,"models":{"gemini-3.6-flash":{},"gemini-web-search":{}}}}}`, providerID))
	personal := []byte(fmt.Sprintf(`{"schemaVersion":1,"config":{
		"providerConfigRules":{"providerRules":[{"providerId":"keep","enabled":true},{"providerId":%q}]},
		"modelConfigRules":{"providerModelRules":[{"providerId":"keep","modelId":"custom","config":{"properties":{"contextWindow":9007199254740993}}}],"manualProviderModelRules":[{"providerId":%q,"modelId":"gemini-3.6-flash"}]},
		"defaultModelSelection":{"providerId":%q,"modelId":"gemini-3.6-flash","options":{"reasoningLevel":"disabled"}},
		"providerOrder":["keep",%q]
	}}`, providerID, providerID, providerID, providerID))
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0700); err != nil {
		t.Fatal(err)
	}
	personalPath := filepath.Join(filepath.Dir(a.paths.ZCodeConfig), "provider_config.json")
	for path, raw := range map[string][]byte{a.paths.ZCodeConfig: legacy, personalPath: personal} {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, changed, err := a.configureZCode(18080, requiredTestModels()); err != nil || !changed {
		t.Fatalf("sync changed=%v err=%v", changed, err)
	}
	_, root, err := readJSONObject(personalPath)
	if err != nil {
		t.Fatal(err)
	}
	config := root["config"].(map[string]any)
	selection := config["defaultModelSelection"].(map[string]any)
	if selection["modelId"] != "gemini-3.8-flash" || selection["options"].(map[string]any)["reasoningLevel"] != "enabled" {
		t.Fatalf("wrong default: %v", selection)
	}
	rules := config["modelConfigRules"].(map[string]any)
	models := rules["providerModelRules"].([]any)
	if len(models) != 3 || len(rules["manualProviderModelRules"].([]any)) != 0 {
		t.Fatalf("old rules remain: %v", rules)
	}
	keep := models[0].(map[string]any)
	if keep["providerId"] != "keep" || fmt.Sprint(keep["config"].(map[string]any)["properties"].(map[string]any)["contextWindow"]) != "9007199254740993" {
		t.Fatal("unrelated model changed")
	}
	for _, value := range models[1:] {
		model := value.(map[string]any)
		if !isAllowedZCodeModel(model["modelId"].(string)) {
			t.Fatalf("legacy model remains: %v", model["modelId"])
		}
		cfg := model["config"].(map[string]any)
		if fmt.Sprint(cfg["properties"].(map[string]any)["contextWindow"]) != "393216" {
			t.Fatal("384K context not applied")
		}
		option := cfg["optionSpecs"].(map[string]any)["reasoningLevel"].(map[string]any)
		if fmt.Sprint(option["values"]) != "[disabled enabled]" || option["map"] != geminiHighReasoningMap {
			t.Fatalf("on/off mapping missing: %v", option)
		}
	}
	a.zcodeRunning = func() bool { return true }
	if backup, changed, err := a.configureZCode(18080, requiredTestModels()); err != nil || changed || backup != "" {
		t.Fatalf("idempotence failed: changed=%v err=%v", changed, err)
	}
	a.zcodeRunning = func() bool { return false }
	// A user's unrelated default selection survives later sync and removal.
	config["defaultModelSelection"] = map[string]any{"providerId": "keep", "modelId": "custom"}
	raw, _ := marshalJSONObject(root)
	if err := os.WriteFile(personalPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.configureZCode(18081, requiredTestModels()); err != nil {
		t.Fatal(err)
	}
	if err := a.removeZCodeProvider(); err != nil {
		t.Fatal(err)
	}
	_, root, err = readJSONObject(personalPath)
	if err != nil {
		t.Fatal(err)
	}
	config = root["config"].(map[string]any)
	if config["defaultModelSelection"].(map[string]any)["providerId"] != "keep" || fmt.Sprint(config["providerOrder"]) != "[keep]" {
		t.Fatal("unrelated selection changed")
	}
	if len(config["providerConfigRules"].(map[string]any)["providerRules"].([]any)) != 1 || len(config["modelConfigRules"].(map[string]any)["providerModelRules"].([]any)) != 1 {
		t.Fatal("provider removal did not clean both schemas")
	}
}

func TestPersonalProviderFailureLeavesLegacyUntouched(t *testing.T) {
	for name, personal := range map[string]string{
		"invalid-json":     `{"schemaVersion":`,
		"newer-schema":     `{"schemaVersion":2,"config":{}}`,
		"invalid-rules":    `{"schemaVersion":1,"config":{"providerConfigRules":{"providerRules":{}}}}`,
		"unowned-provider": fmt.Sprintf(`{"schemaVersion":1,"config":{"providerConfigRules":{"providerRules":[{"providerId":%q}]}}}`, providerID),
	} {
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0700); err != nil {
				t.Fatal(err)
			}
			before := []byte(`{"provider":{}}`)
			personalPath := filepath.Join(filepath.Dir(a.paths.ZCodeConfig), "provider_config.json")
			if err := os.WriteFile(a.paths.ZCodeConfig, before, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(personalPath, []byte(personal), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := a.configureZCode(18080, requiredTestModels()); err == nil {
				t.Fatal("expected preflight error")
			}
			after, err := os.ReadFile(a.paths.ZCodeConfig)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("legacy file changed on preflight failure")
			}
			after, err = os.ReadFile(personalPath)
			if err != nil || string(after) != personal {
				t.Fatal("personal file changed on preflight failure")
			}
		})
	}
}

func TestPersonalProviderOnlyChangeBlocksWhileZCodeRuns(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.paths.ZCodeConfig, []byte(`{"provider":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.configureZCode(18080, requiredTestModels()); err != nil {
		t.Fatal(err)
	}
	personalPath := filepath.Join(filepath.Dir(a.paths.ZCodeConfig), "provider_config.json")
	if err := os.Remove(personalPath); err != nil {
		t.Fatal(err)
	}
	a.zcodeRunning = func() bool { return true }
	if _, _, err := a.configureZCode(18080, requiredTestModels()); err == nil {
		t.Fatal("modern schema change bypassed running client guard")
	}
	if _, err := os.Stat(personalPath); !os.IsNotExist(err) {
		t.Fatal("created file while ZCode running")
	}
}
