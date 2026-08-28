package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchXAIQuotaWithoutAccountReturnsEmptyLoginState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "CLI Proxy API Server"})
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	a := testApp(t)
	port := server.Listener.Addr().(*net.TCPAddr).Port
	if err := a.saveState(state{Port: port}); err != nil {
		t.Fatal(err)
	}
	report, err := a.fetchXAIQuotaReport()
	if err != nil {
		t.Fatalf("empty Grok state must not be an HTTP 503 cause: %v", err)
	}
	if report.Provider != "xai" || len(report.Accounts) != 0 || report.Warning == "" {
		t.Fatalf("unexpected empty Grok report: %#v", report)
	}
}

func TestRetrieveXAIBillingUsesOfficialGrokHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/management/api-call" || r.Header.Get("X-Management-Key") == "" {
			t.Fatalf("unexpected management request: %s %#v", r.URL.Path, r.Header)
		}
		var payload struct {
			AuthIndex string            `json:"auth_index"`
			Method    string            `json:"method"`
			URL       string            `json:"url"`
			Header    map[string]string `json:"header"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.AuthIndex != "auth-index" || payload.Method != http.MethodGet || payload.URL != xaiBillingEndpoint || payload.Header["Authorization"] != "Bearer $TOKEN$" || payload.Header["X-XAI-Token-Auth"] != "xai-grok-cli" || payload.Header["x-userid"] != "user-123" {
			t.Fatalf("unexpected proxied Grok billing request: %#v", payload)
		}
		body := `{"config":{"creditUsagePercent":37.5,"currentPeriod":{"type":"USAGE_PERIOD_TYPE_WEEKLY","end":"2026-08-24T00:00:00Z"},"prepaidBalance":{"val":1250}},"subscriptionTier":"SuperGrok"}`
		_ = json.NewEncoder(w).Encode(managementAPICallResponse{StatusCode: http.StatusOK, Body: body})
	}))
	defer server.Close()
	a := testApp(t)
	port := server.Listener.Addr().(*net.TCPAddr).Port
	billing, err := a.retrieveXAIBilling(port, "auth-index", "user-123")
	if err != nil {
		t.Fatal(err)
	}
	if billing.Config == nil || billing.Config.CreditUsagePercent == nil || *billing.Config.CreditUsagePercent != 37.5 || billing.SubscriptionTier != "SuperGrok" {
		t.Fatalf("billing = %#v", billing)
	}
}

func TestBuildAgentConnectorsCoversMajorAgents(t *testing.T) {
	connectors := buildAgentConnectors("http://127.0.0.1:18080", "local-key", "grok-4.5", "xai")
	seen := map[string]bool{}
	for _, connector := range connectors {
		seen[connector.ID] = true
	}
	for _, id := range []string{"grok-build", "codex", "claude-code", "opencode", "generic"} {
		if !seen[id] {
			t.Fatalf("missing connector %q", id)
		}
	}
}
