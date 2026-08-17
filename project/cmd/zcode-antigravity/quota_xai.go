package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const xaiBillingEndpoint = "https://cli-chat-proxy.grok.com/v1/billing?format=credits"

type xaiBillingResponse struct {
	Config *struct {
		CreditUsagePercent *float64 `json:"creditUsagePercent"`
		CurrentPeriod      *struct {
			Type  string `json:"type"`
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"currentPeriod"`
		MonthlyLimit   *xaiCent `json:"monthlyLimit"`
		Used           *xaiCent `json:"used"`
		OnDemandCap    *xaiCent `json:"onDemandCap"`
		OnDemandUsed   *xaiCent `json:"onDemandUsed"`
		PrepaidBalance *xaiCent `json:"prepaidBalance"`
	} `json:"config"`
	SubscriptionTier string `json:"subscriptionTier"`
}

type xaiCent struct {
	Val int64 `json:"val"`
}

type xaiCredential struct {
	FileName string `json:"-"`
	Subject  string `json:"sub"`
	Email    string `json:"email"`
}

func (a *app) fetchProviderQuotaReport(provider string) (quotaReport, error) {
	switch normalizeProvider(provider) {
	case "xai":
		return a.fetchXAIQuotaReport()
	default:
		return a.fetchQuotaReport()
	}
}

func normalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "grok" || provider == "xai" {
		return "xai"
	}
	return "antigravity"
}

func (a *app) fetchXAIQuotaReport() (quotaReport, error) {
	current, errState := a.loadState()
	if errState != nil || current.Port <= 0 {
		return quotaReport{}, errors.New("网关尚未启动")
	}
	if errProbe := a.probeGateway(current.Port); errProbe != nil {
		return quotaReport{}, fmt.Errorf("网关未运行: %w", errProbe)
	}
	credentials, err := loadXAICredentials(a.paths.AuthDir)
	if err != nil {
		return quotaReport{}, err
	}
	var authFiles managementAuthFiles
	if errAuth := a.managementJSON(current.Port, http.MethodGet, "/v0/management/auth-files", nil, &authFiles); errAuth != nil {
		return quotaReport{}, fmt.Errorf("Grok 额度接口尚未就绪: %w", errAuth)
	}
	xaiFiles := make([]struct {
		AuthIndex string
		Email     string
		Label     string
	}, 0)
	for _, authFile := range authFiles.Files {
		if strings.EqualFold(strings.TrimSpace(authFile.Provider), "xai") && strings.TrimSpace(authFile.AuthIndex) != "" {
			xaiFiles = append(xaiFiles, struct{ AuthIndex, Email, Label string }{authFile.AuthIndex, authFile.Email, authFile.Label})
		}
	}
	if len(credentials) == 0 || len(xaiFiles) == 0 {
		return quotaReport{}, errors.New("没有 Grok / xAI 账号，请先登录")
	}
	report := quotaReport{
		FetchedAt: a.now().UTC(),
		Provider:  "xai",
		Source:    "xAI Grok Build billing",
		Accounts:  make([]quotaAccount, 0, len(xaiFiles)),
	}
	for _, authFile := range xaiFiles {
		credential, found := matchXAICredential(credentials, authFile.Email)
		account := quotaAccount{Account: maskEmail(firstText(authFile.Email, authFile.Label, credential.Email, credential.Subject, "Grok account")), Status: "ready"}
		if !found || strings.TrimSpace(credential.Subject) == "" {
			account.Status = "error"
			account.Error = "Grok 凭据缺少额度接口所需的用户 ID"
			report.Accounts = append(report.Accounts, account)
			continue
		}
		billing, errBilling := a.retrieveXAIBilling(current.Port, authFile.AuthIndex, credential.Subject)
		if errBilling != nil {
			account.Status = "error"
			account.Error = errBilling.Error()
			report.Accounts = append(report.Accounts, account)
			continue
		}
		account.Plan = firstText(strings.TrimSpace(billing.SubscriptionTier), "Grok Build")
		if billing.Config == nil {
			account.Error = "xAI 当前账号未返回额度配置"
			report.Accounts = append(report.Accounts, account)
			continue
		}
		config := billing.Config
		usedPercent := config.CreditUsagePercent
		if usedPercent == nil && config.MonthlyLimit != nil && config.MonthlyLimit.Val > 0 && config.Used != nil {
			value := float64(config.Used.Val) / float64(config.MonthlyLimit.Val) * 100
			usedPercent = &value
		}
		var remaining *float64
		if usedPercent != nil {
			value := math.Max(0, math.Min(100, 100-*usedPercent))
			remaining = &value
		}
		bucket := quotaBucket{Name: "Grok 剩余额度", Description: "xAI 官方 Grok Build 共享额度池", RemainingPercent: remaining}
		if config.CurrentPeriod != nil {
			if strings.Contains(strings.ToUpper(config.CurrentPeriod.Type), "WEEK") {
				bucket.Name = "Grok 每周剩余额度"
				bucket.Window = "weekly"
			} else if strings.Contains(strings.ToUpper(config.CurrentPeriod.Type), "MONTH") {
				bucket.Name = "Grok 每月剩余额度"
				bucket.Window = "monthly"
			}
			if reset, errTime := time.Parse(time.RFC3339, strings.TrimSpace(config.CurrentPeriod.End)); errTime == nil {
				bucket.ResetTime = &reset
			}
		}
		account.Groups = []quotaGroup{{Name: "Grok Build", Description: "xAI 统一用量", Buckets: []quotaBucket{bucket}}}
		if config.PrepaidBalance != nil {
			account.Credits = &creditInfo{Available: config.PrepaidBalance.Val != 0, Amount: math.Abs(float64(config.PrepaidBalance.Val)) / 100, CreditType: "USD", UpstreamLabel: "Extra Usage Credits (USD)"}
		}
		if config.OnDemandCap != nil && config.OnDemandUsed != nil && config.OnDemandCap.Val != 0 {
			capValue := math.Abs(float64(config.OnDemandCap.Val)) / 100
			usedValue := math.Abs(float64(config.OnDemandUsed.Val)) / 100
			account.StatusMessage = fmt.Sprintf("按量额度：已用 $%.2f / 上限 $%.2f", usedValue, capValue)
		}
		report.Accounts = append(report.Accounts, account)
	}
	return report, nil
}

func loadXAICredentials(dir string) ([]xaiCredential, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	credentials := make([]xaiCredential, 0)
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if entry.IsDir() || !strings.HasPrefix(name, "xai-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, errRead := os.ReadFile(filepath.Join(dir, entry.Name()))
		if errRead != nil {
			return nil, errRead
		}
		var credential xaiCredential
		if errJSON := json.Unmarshal(raw, &credential); errJSON != nil {
			return nil, fmt.Errorf("读取 Grok 凭据 %s: %w", entry.Name(), errJSON)
		}
		if strings.TrimSpace(credential.Subject) == "" {
			return nil, fmt.Errorf("Grok 凭据缺少用户 ID: %s", entry.Name())
		}
		credential.FileName = entry.Name()
		credentials = append(credentials, credential)
	}
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].Email < credentials[j].Email })
	return credentials, nil
}

func matchXAICredential(credentials []xaiCredential, email string) (xaiCredential, bool) {
	email = strings.TrimSpace(email)
	for _, credential := range credentials {
		if email != "" && strings.EqualFold(strings.TrimSpace(credential.Email), email) {
			return credential, true
		}
	}
	if len(credentials) == 1 {
		return credentials[0], true
	}
	return xaiCredential{}, false
}

func (a *app) retrieveXAIBilling(port int, authIndex, subject string) (xaiBillingResponse, error) {
	body, errCall := a.managementAPICallRequest(port, authIndex, http.MethodGet, xaiBillingEndpoint, "", map[string]string{
		"Authorization":            "Bearer $TOKEN$",
		"Accept":                   "application/json",
		"X-XAI-Token-Auth":         "xai-grok-cli",
		"x-userid":                 subject,
		"x-grok-client-version":    "0.2.120",
		"x-grok-client-identifier": "grok-shell",
	})
	if errCall != nil {
		return xaiBillingResponse{}, fmt.Errorf("读取 xAI 额度: %w", errCall)
	}
	var billing xaiBillingResponse
	if errJSON := json.Unmarshal(body, &billing); errJSON != nil {
		return xaiBillingResponse{}, fmt.Errorf("xAI 额度响应不是有效 JSON: %w", errJSON)
	}
	return billing, nil
}
