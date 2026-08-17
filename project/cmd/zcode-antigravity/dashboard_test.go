package main

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardAPIRequiresSessionTokenAndSetsSecurityHeaders(t *testing.T) {
	runtime := &guiRuntime{app: testApp(t), token: "test-dashboard-session-token"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/heartbeat", runtime.authorized(runtime.serveHeartbeat))
	handler := securityHeaders(mux)

	unauthorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/heartbeat", nil)
	handler.ServeHTTP(unauthorized, request)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("without token status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	authorized := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/heartbeat", nil)
	request.Header.Set("X-ZCAB-Session", runtime.token)
	handler.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("with token status = %d, want %d", authorized.Code, http.StatusNoContent)
	}
	if value := authorized.Header().Get("Cache-Control"); value != "no-store" {
		t.Fatalf("Cache-Control = %q", value)
	}
	if value := authorized.Header().Get("X-Frame-Options"); value != "DENY" {
		t.Fatalf("X-Frame-Options = %q", value)
	}
	if value := authorized.Header().Get("Content-Security-Policy"); !strings.Contains(value, "default-src 'none'") {
		t.Fatalf("Content-Security-Policy = %q", value)
	}
}

func TestDashboardTokenComparisonIsExact(t *testing.T) {
	for _, value := range []string{"", "secret", "secret-extra", "Secret"} {
		if !subtleTokenMismatch(value, "secret-token") {
			t.Fatalf("value %q unexpectedly matched", value)
		}
	}
	if subtleTokenMismatch("secret-token", "secret-token") {
		t.Fatal("exact token did not match")
	}
}

func TestGatewayNeedsRecoveryOnlyForUnexpectedBridgeExit(t *testing.T) {
	recorded := state{Port: 18080, PID: 3210}
	account := providerAccountCounts{Antigravity: 1}
	for _, test := range []struct {
		name            string
		current         state
		gatewayHealthy  bool
		processAlive    bool
		accounts        providerAccountCounts
		zcodeConfigured bool
		wantRecovery    bool
	}{
		{name: "crashed bridge", current: recorded, accounts: account, zcodeConfigured: true, wantRecovery: true},
		{name: "healthy bridge", current: recorded, gatewayHealthy: true, accounts: account, zcodeConfigured: true},
		{name: "still starting", current: recorded, processAlive: true, accounts: account, zcodeConfigured: true},
		{name: "intentional stop", current: state{Port: 18080}, accounts: account, zcodeConfigured: true},
		{name: "no account", current: recorded, zcodeConfigured: true},
		{name: "provider not installed", current: recorded, accounts: account},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := gatewayNeedsRecovery(test.current, test.gatewayHealthy, test.processAlive, test.accounts, test.zcodeConfigured); got != test.wantRecovery {
				t.Fatalf("gatewayNeedsRecovery() = %t, want %t", got, test.wantRecovery)
			}
		})
	}
}

func TestDashboardThemeIncludesCrispResponsiveMotion(t *testing.T) {
	for _, expected := range []string{
		"--accent: #0b63f6",
		"Segoe UI Variable Text",
		"SF Pro Text",
		"font-synthesis: none",
		"@media (max-width: 520px)",
		"@media (min-width: 1800px)",
		"@keyframes buttonSheen",
		"@keyframes ambientDrift",
		"drawPath(w, base+5",
		"Math.min(Math.max(devicePixelRatio || 1, 1), 4)",
		"prefers-reduced-motion: reduce",
		"data-provider=\"xai\"",
		"Agent Connectors",
		"data-action=\"login-grok\"",
		"animation: none !important",
	} {
		if !strings.Contains(dashboardHTML, expected) {
			t.Fatalf("dashboard is missing %q", expected)
		}
	}
	for _, forbidden := range []string{
		"font-weight: 730",
		"font-weight: 620",
		"letter-spacing: -.01em",
	} {
		if strings.Contains(dashboardHTML, forbidden) {
			t.Fatalf("dashboard still contains jagged-text trigger %q", forbidden)
		}
	}
}

func TestTraySnapshotAndIconRepresentSelectedProvider(t *testing.T) {
	weekly := 72.0
	fiveHour := 41.0
	lowerWeekly := 63.0
	report := quotaReport{
		Provider: "antigravity",
		Accounts: []quotaAccount{{
			Plan: "Pro",
			Groups: []quotaGroup{{Buckets: []quotaBucket{
				{Name: "每周剩余额度", RemainingPercent: &weekly},
				{Name: "5 小时剩余额度", RemainingPercent: &fiveHour},
			}}},
		}, {
			Plan:   "Pro",
			Groups: []quotaGroup{{Buckets: []quotaBucket{{Name: "每周剩余额度", RemainingPercent: &lowerWeekly}}}},
		}},
	}
	snapshot := traySnapshotFromReport("antigravity", report)
	if snapshot.Summary != "Antigravity · 周 63% · 5小时 41%" || snapshot.Remaining == nil || *snapshot.Remaining != 41 || !strings.Contains(snapshot.Detail, "2 个账号") {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	icon := quotaTrayIcon(snapshot.Remaining, "antigravity", false)
	decoded, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 44 || decoded.Bounds().Dy() != 44 {
		t.Fatalf("icon bounds = %v", decoded.Bounds())
	}
}
