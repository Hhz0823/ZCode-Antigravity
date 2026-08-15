package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestAntigravityFileStoreProtectsTokensAndReadsThemBack(t *testing.T) {
	dir := t.TempDir()
	store := NewFileTokenStore()
	store.SetBaseDir(dir)
	auth := &cliproxyauth.Auth{
		ID:       "antigravity-test.json",
		Provider: "antigravity",
		Metadata: map[string]any{
			"type":          "antigravity",
			"access_token":  "plain-access-secret",
			"refresh_token": "plain-refresh-secret",
			"project_id":    "test-project",
			"email":         "test@example.invalid",
		},
	}
	path, errSave := store.Save(t.Context(), auth)
	if errSave != nil {
		t.Fatalf("Save: %v", errSave)
	}
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		t.Fatalf("read saved auth: %v", errRead)
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(string(raw), "plain-access-secret") || strings.Contains(string(raw), "plain-refresh-secret") {
			t.Fatalf("saved Windows auth contains plaintext token: %s", raw)
		}
		if !strings.Contains(string(raw), protectedCredentialPrefix) {
			t.Fatalf("saved Windows auth lacks DPAPI marker: %s", raw)
		}
	}
	listed, errList := store.List(t.Context())
	if errList != nil {
		t.Fatalf("List: %v", errList)
	}
	if len(listed) != 1 {
		t.Fatalf("listed auth count=%d, want 1", len(listed))
	}
	if got := listed[0].Metadata["access_token"]; got != "plain-access-secret" {
		t.Fatalf("decrypted access token=%v", got)
	}
}

func TestFileStoreFailsOnMalformedAuthJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed auth: %v", err)
	}
	store := NewFileTokenStore()
	store.SetBaseDir(dir)
	if _, err := store.List(t.Context()); err == nil {
		t.Fatal("List silently ignored malformed auth JSON")
	}
}

func TestAntigravityFileStoreMigratesLegacyPlaintextTokens(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("DPAPI migration is Windows-specific")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.json")
	legacy := `{"type":"antigravity","access_token":"legacy-access","refresh_token":"legacy-refresh","project_id":"test-project"}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy auth: %v", err)
	}
	store := NewFileTokenStore()
	store.SetBaseDir(dir)
	listed, errList := store.List(t.Context())
	if errList != nil {
		t.Fatalf("List: %v", errList)
	}
	if len(listed) != 1 || listed[0].Metadata["access_token"] != "legacy-access" {
		t.Fatalf("legacy auth was not returned decrypted: %#v", listed)
	}
	raw, errRead := os.ReadFile(path)
	if errRead != nil {
		t.Fatalf("read migrated auth: %v", errRead)
	}
	if strings.Contains(string(raw), "legacy-access") || strings.Contains(string(raw), "legacy-refresh") {
		t.Fatalf("legacy auth remained plaintext: %s", raw)
	}
	if !strings.Contains(string(raw), protectedCredentialPrefix) {
		t.Fatalf("migrated auth lacks DPAPI marker: %s", raw)
	}
}
