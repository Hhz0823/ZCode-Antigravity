package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestGoogleVerificationSignals(t *testing.T) {
	for _, tt := range []struct {
		message string
		want    bool
	}{
		{`{"error":{"code":403,"details":[{"reason":"VALIDATION_REQUIRED"}]}}`, true},
		{"Please verify your account to continue", true},
		{"Please verify your age", true},
		{`{"validation_required":true}`, true},
		{`{"validation_required":false}`, false},
		{`{"error":{"code":403,"message":"Permission denied for quota summary"}}`, false},
		{"Quota exhausted; retry later", false},
		{"Connection timed out", false},
	} {
		if got := requiresGoogleVerification(tt.message); got != tt.want {
			t.Errorf("requiresGoogleVerification(%q) = %v", tt.message, got)
		}
	}
}

func TestGoogleVerificationOverridesHealthyQuotaCacheWithoutLeakingChallenge(t *testing.T) {
	a := testApp(t)
	if err := a.saveQuotaCache(quotaReport{Accounts: []quotaAccount{{Account: "old", Groups: []quotaGroup{{Buckets: []quotaBucket{{Name: "old healthy quota"}}}}}}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "CLI Proxy API Server"})
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/v0/management/auth-files":
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{{"name": "antigravity-new.json", "auth_index": "new", "provider": "antigravity", "email": "new@example.com", "project_id": "private-project", "status": "ready"}}})
		case "/v0/management/api-call":
			calls++
			_ = json.NewEncoder(w).Encode(managementAPICallResponse{StatusCode: 403, Body: `{"error":{"message":"Please verify your account","details":[{"reason":"VALIDATION_REQUIRED","metadata":{"validation_url":"https://accounts.google.com/challenge?private-secret=123"}}]}}`})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(parsed.Port())
	if err := a.saveState(state{Port: port}); err != nil {
		t.Fatal(err)
	}
	report, err := a.fetchQuotaReport()
	if err != nil {
		t.Fatal(err)
	}
	if report.Stale || len(report.Accounts) != 1 || !report.Accounts[0].VerificationRequired || calls != 1 {
		t.Fatalf("verification was hidden or retried: report=%+v calls=%d", report, calls)
	}
	if report.Accounts[0].ID != managerAccountID("antigravity", "antigravity-new.json") {
		t.Fatal("live account cannot be matched to the manager account")
	}
	raw, _ := os.ReadFile(a.quotaCachePath())
	for _, secret := range []string{"private-secret", "private-project", "new@example.com"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("cache leaked %s", secret)
		}
	}
}
