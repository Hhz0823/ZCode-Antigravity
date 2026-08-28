package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeUpstreamOAuthReplacesCredentialsWithoutPersistingValues(t *testing.T) {
	root := t.TempDir()
	for _, relative := range upstreamOAuthFiles {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		clientID := "123456789-test.apps." + "googleusercontent.com"
		clientSecret := "GOC" + "SPX-" + "abcdefghijklmnopqrstuvwxyz"
		fixture := "clientID := \"" + clientID + "\"\nclientSecret := \"" + clientSecret + "\"\n"
		if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	count, err := sanitizeUpstreamOAuth(root)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(upstreamOAuthFiles)*2 {
		t.Fatalf("replacement count = %d", count)
	}
	for _, relative := range upstreamOAuthFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if strings.Contains(text, "apps.googleusercontent.com") || strings.Contains(text, "GOCSPX-") {
			t.Fatalf("credential pattern remained in %s", relative)
		}
		if !strings.Contains(text, "ZCODE_REDACTED_GOOGLE_OAUTH_CLIENT_ID") || !strings.Contains(text, "ZCODE_REDACTED_GOOGLE_OAUTH_CLIENT_SECRET") {
			t.Fatalf("redaction placeholders missing in %s", relative)
		}
	}
}
