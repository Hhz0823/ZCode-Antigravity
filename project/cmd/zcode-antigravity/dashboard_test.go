package main

import (
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

func TestDashboardThemeIncludesBlueMotionAndReducedMotionFallback(t *testing.T) {
	for _, expected := range []string{
		"--accent: #0b63f6",
		"@keyframes quotaCardIn",
		"drawPath(w, base+5",
		"prefers-reduced-motion: reduce",
		"animation: none !important",
	} {
		if !strings.Contains(dashboardHTML, expected) {
			t.Fatalf("dashboard is missing %q", expected)
		}
	}
}
