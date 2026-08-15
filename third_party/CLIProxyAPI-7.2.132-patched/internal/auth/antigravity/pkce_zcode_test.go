package antigravity

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestAntigravityOAuthUsesPKCE(t *testing.T) {
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_ID", "development-client-id")
	t.Setenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET", "development-client-secret")
	pkce := &PKCECodes{CodeVerifier: "verifier-value", CodeChallenge: "challenge-value"}
	auth := NewAntigravityAuth(nil, &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, errRead := io.ReadAll(req.Body)
		if errRead != nil {
			t.Fatalf("read token request: %v", errRead)
		}
		values, errParse := url.ParseQuery(string(body))
		if errParse != nil {
			t.Fatalf("parse token request: %v", errParse)
		}
		if got := values.Get("code_verifier"); got != pkce.CodeVerifier {
			t.Fatalf("code_verifier = %q, want %q", got, pkce.CodeVerifier)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`)),
			Header:     make(http.Header),
		}, nil
	})})

	authURL, errURL := auth.BuildAuthURL("state-value", "http://localhost:51121/oauth-callback", pkce)
	if errURL != nil {
		t.Fatalf("BuildAuthURL: %v", errURL)
	}
	parsed, errParse := url.Parse(authURL)
	if errParse != nil {
		t.Fatalf("parse auth URL: %v", errParse)
	}
	if got := parsed.Query().Get("code_challenge"); got != pkce.CodeChallenge {
		t.Fatalf("code_challenge = %q, want %q", got, pkce.CodeChallenge)
	}
	if got := parsed.Query().Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", got)
	}
	if _, errExchange := auth.ExchangeCodeForTokens(context.Background(), "code", "http://localhost:51121/oauth-callback", pkce); errExchange != nil {
		t.Fatalf("ExchangeCodeForTokens: %v", errExchange)
	}
}

func TestAntigravityOAuthRejectsMissingPKCE(t *testing.T) {
	auth := NewAntigravityAuth(nil, &http.Client{})
	if _, err := auth.BuildAuthURL("state", "", nil); err == nil {
		t.Fatal("BuildAuthURL accepted missing PKCE")
	}
	if _, err := auth.ExchangeCodeForTokens(context.Background(), "code", "redirect", nil); err == nil {
		t.Fatal("ExchangeCodeForTokens accepted missing PKCE")
	}
}
