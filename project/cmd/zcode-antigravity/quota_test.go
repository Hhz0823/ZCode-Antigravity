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
	"time"
)

func TestFetchQuotaReportReadsSummaryAndKeepsSecretsOutOfCache(t *testing.T) {
	a := testApp(t)
	projectID := "private-project-id"
	email := "quota-user@example.com"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "CLI Proxy API Server"})
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/v0/management/auth-files":
			if r.Header.Get("X-Management-Key") != a.apiKey {
				t.Fatalf("missing management key")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]any{{
				"auth_index": "auth-index-1",
				"provider":   "antigravity",
				"email":      email,
				"project_id": projectID,
				"status":     "ready",
			}}})
		case "/v0/management/api-call":
			var request struct {
				URL  string `json:"url"`
				Data string `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(request.URL, "retrieveUserQuotaSummary") {
				if !strings.Contains(request.Data, projectID) {
					t.Fatalf("quota request missing project ID")
				}
				writeAPICallTestResponse(t, w, map[string]any{
					"groups": []map[string]any{{
						"displayName": "Gemini Models",
						"buckets": []map[string]any{{
							"bucketId":          "weekly",
							"displayName":       "Weekly Limit Remaining",
							"remainingFraction": 0.99,
							"resetTime":         "2026-08-21T06:00:00Z",
						}, {
							"bucketId":          "five-hour",
							"displayName":       "Five Hour Limit Remaining",
							"remainingFraction": 1.0,
						}},
					}},
				})
				return
			}
			if strings.Contains(request.URL, "loadCodeAssist") {
				writeAPICallTestResponse(t, w, map[string]any{
					"paidTier": map[string]any{
						"id": "G1_PRO_TIER",
						"availableCredits": []map[string]string{{
							"creditType":                  "GOOGLE_ONE_AI",
							"creditAmount":                "25000",
							"minimumCreditAmountForUsage": "50",
						}},
					},
				})
				return
			}
			http.Error(w, "unexpected api-call", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	parsed, errURL := url.Parse(server.URL)
	if errURL != nil {
		t.Fatal(errURL)
	}
	port := parsed.Port()
	if port == "" {
		t.Fatal("test server port missing")
	}
	portNumber, errPort := strconv.Atoi(port)
	if errPort != nil {
		t.Fatal(errPort)
	}
	if err := a.saveState(state{Port: portNumber}); err != nil {
		t.Fatal(err)
	}

	report, errReport := a.fetchQuotaReport()
	if errReport != nil {
		t.Fatal(errReport)
	}
	if len(report.Accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(report.Accounts))
	}
	account := report.Accounts[0]
	if account.Account != "q*****r@example.com" {
		t.Fatalf("masked account = %q", account.Account)
	}
	if account.Plan != "Google AI Pro" {
		t.Fatalf("plan = %q", account.Plan)
	}
	if account.Credits == nil || account.Credits.Amount != 25000 || !account.Credits.Available {
		t.Fatalf("credits = %+v", account.Credits)
	}
	if len(account.Groups) != 1 || len(account.Groups[0].Buckets) != 2 {
		t.Fatalf("quota groups = %+v", account.Groups)
	}
	if got := *account.Groups[0].Buckets[0].RemainingPercent; got != 99 {
		t.Fatalf("weekly percent = %v", got)
	}
	if account.Groups[0].Buckets[0].Name != "每周剩余额度" || account.Groups[0].Buckets[1].Name != "5 小时剩余额度" {
		t.Fatalf("localized buckets = %+v", account.Groups[0].Buckets)
	}

	cache, errRead := os.ReadFile(a.quotaCachePath())
	if errRead != nil {
		t.Fatal(errRead)
	}
	if strings.Contains(string(cache), projectID) || strings.Contains(string(cache), email) {
		t.Fatal("quota cache leaked project ID or full email")
	}
}

func writeAPICallTestResponse(t *testing.T, w http.ResponseWriter, upstream any) {
	t.Helper()
	raw, errJSON := json.Marshal(upstream)
	if errJSON != nil {
		t.Fatal(errJSON)
	}
	_ = json.NewEncoder(w).Encode(managementAPICallResponse{StatusCode: http.StatusOK, Body: string(raw)})
}

func TestCachedQuotaReportMarksDataStale(t *testing.T) {
	a := testApp(t)
	report := quotaReport{FetchedAt: time.Now().UTC(), Source: "test", Accounts: []quotaAccount{{Account: "q***@example.com"}}}
	if err := a.saveQuotaCache(report); err != nil {
		t.Fatal(err)
	}
	cached, err := a.cachedQuotaReport(assertionError("network down"))
	if err != nil {
		t.Fatal(err)
	}
	if !cached.Stale || !strings.Contains(cached.Warning, "network down") {
		t.Fatalf("cached report = %+v", cached)
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
