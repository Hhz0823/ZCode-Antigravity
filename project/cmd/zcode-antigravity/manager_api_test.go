package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestManagerSettingsUpdatePersistsValidatedRouting(t *testing.T) {
	a := testApp(t)
	runtime := &guiRuntime{app: a, usage: newUsageTracker(a.paths.UsageMetrics)}
	body := bytes.NewBufferString(`{"routingStrategy":"fill-first","sessionAffinity":false,"autoRefreshMinutes":10,"quotaWarningPercent":30,"enableGrokModels":true,"enableOtherModels":true}`)
	request := httptest.NewRequest(http.MethodPost, "/api/manager/settings", body)
	recorder := httptest.NewRecorder()
	runtime.serveManagerSettings(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	current := a.currentSettings()
	if current.RoutingStrategy != "fill-first" || current.SessionAffinity || current.AutoRefreshMinutes != 10 || current.QuotaWarningPercent != 30 || !current.EnableGrokModels || !current.EnableOtherModels {
		t.Fatalf("settings not applied: %#v", current)
	}
	raw, err := os.ReadFile(a.paths.UserSettings)
	if err != nil {
		t.Fatal(err)
	}
	var persisted settings
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.RoutingStrategy != "fill-first" || persisted.AutoRefreshMinutes != 10 || !persisted.EnableGrokModels || !persisted.EnableOtherModels {
		t.Fatalf("settings not persisted: %#v", persisted)
	}
}

func TestReadManagerAccountsRedactsCredentialMetadata(t *testing.T) {
	a := testApp(t)
	fixture := map[string]any{
		"type":          "antigravity",
		"email":         "developer@example.com",
		"plan":          "Google AI Pro",
		"access_token":  "must-never-leave-backend",
		"refresh_token": "must-never-leave-backend-either",
		"project_id":    "local-project",
	}
	raw, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(a.paths.AuthDir, "antigravity-test.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	accounts, err := readManagerAccounts(a.paths.AuthDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Label != "d******@example.com" || accounts[0].Plan != "Google AI Pro" {
		t.Fatalf("unexpected account: %#v", accounts)
	}
	if bytes.Contains([]byte(accounts[0].ID), []byte("test")) || bytes.Contains([]byte(accounts[0].ID), []byte("developer")) {
		t.Fatalf("account id was not anonymized: %q", accounts[0].ID)
	}
	encoded, _ := json.Marshal(accounts)
	if bytes.Contains(encoded, []byte("must-never")) {
		t.Fatal("manager response exposed a credential")
	}
}

func TestManagerSettingsRejectsUnknownFields(t *testing.T) {
	a := testApp(t)
	runtime := &guiRuntime{app: a}
	request := httptest.NewRequest(http.MethodPost, "/api/manager/settings", bytes.NewBufferString(`{"remoteAccess":true}`))
	recorder := httptest.NewRecorder()
	runtime.serveManagerSettings(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
