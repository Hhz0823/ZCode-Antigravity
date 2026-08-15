package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const antigravityQuotaUserAgent = "antigravity/hub/2.8.1 darwin/arm64"

type quotaReport struct {
	FetchedAt time.Time      `json:"fetchedAt"`
	Source    string         `json:"source"`
	Stale     bool           `json:"stale"`
	Accounts  []quotaAccount `json:"accounts"`
	Warning   string         `json:"warning,omitempty"`
}

type quotaAccount struct {
	Account       string       `json:"account"`
	Plan          string       `json:"plan,omitempty"`
	Status        string       `json:"status"`
	StatusMessage string       `json:"statusMessage,omitempty"`
	Groups        []quotaGroup `json:"groups,omitempty"`
	Credits       *creditInfo  `json:"credits,omitempty"`
	Error         string       `json:"error,omitempty"`
}

type quotaGroup struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Buckets     []quotaBucket `json:"buckets"`
}

type quotaBucket struct {
	ID               string     `json:"id,omitempty"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	Window           string     `json:"window,omitempty"`
	RemainingPercent *float64   `json:"remainingPercent,omitempty"`
	RemainingAmount  *float64   `json:"remainingAmount,omitempty"`
	ResetTime        *time.Time `json:"resetTime,omitempty"`
	Disabled         bool       `json:"disabled"`
}

type creditInfo struct {
	Available     bool    `json:"available"`
	Amount        float64 `json:"amount"`
	Minimum       float64 `json:"minimum"`
	CreditType    string  `json:"creditType"`
	UpstreamLabel string  `json:"upstreamLabel"`
}

type managementAuthFiles struct {
	Files []struct {
		AuthIndex     string `json:"auth_index"`
		Provider      string `json:"provider"`
		Email         string `json:"email"`
		Label         string `json:"label"`
		ProjectID     string `json:"project_id"`
		Status        string `json:"status"`
		StatusMessage string `json:"status_message"`
		Disabled      bool   `json:"disabled"`
		Unavailable   bool   `json:"unavailable"`
	} `json:"files"`
}

type managementAPICallResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

type upstreamQuotaSummary struct {
	Buckets     []upstreamQuotaBucket `json:"buckets"`
	Groups      []upstreamQuotaGroup  `json:"groups"`
	Description string                `json:"description"`
}

type upstreamQuotaGroup struct {
	DisplayName string                `json:"displayName"`
	Description string                `json:"description"`
	Buckets     []upstreamQuotaBucket `json:"buckets"`
}

type upstreamQuotaBucket struct {
	BucketID          string   `json:"bucketId"`
	DisplayName       string   `json:"displayName"`
	Description       string   `json:"description"`
	Window            string   `json:"window"`
	RemainingFraction *float64 `json:"remainingFraction"`
	RemainingAmount   *float64 `json:"remainingAmount"`
	Disabled          bool     `json:"disabled"`
	ResetTime         string   `json:"resetTime"`
	ModelID           string   `json:"modelId"`
	TokenType         string   `json:"tokenType"`
}

func (a *app) quotaCachePath() string {
	return filepath.Join(a.paths.Data, "quota-cache.json")
}

func (a *app) printQuota() error {
	report, err := a.fetchQuotaReport()
	if err != nil {
		return err
	}
	raw, errJSON := json.MarshalIndent(report, "", "  ")
	if errJSON != nil {
		return errJSON
	}
	fmt.Println(string(raw))
	return nil
}

func (a *app) fetchQuotaReport() (quotaReport, error) {
	current, errState := a.loadState()
	if errState != nil {
		return a.cachedQuotaReport(errState)
	}
	if current.Port <= 0 {
		return a.cachedQuotaReport(errors.New("网关尚未启动"))
	}
	if errProbe := a.probeGateway(current.Port); errProbe != nil {
		return a.cachedQuotaReport(fmt.Errorf("网关未运行: %w", errProbe))
	}

	var authFiles managementAuthFiles
	if errAuth := a.managementJSON(current.Port, http.MethodGet, "/v0/management/auth-files", nil, &authFiles); errAuth != nil {
		return a.cachedQuotaReport(fmt.Errorf("额度接口尚未就绪，请在控制中心执行一次“接入 ZCode”: %w", errAuth))
	}

	report := quotaReport{
		FetchedAt: a.now().UTC(),
		Source:    "Antigravity retrieveUserQuotaSummary",
		Accounts:  make([]quotaAccount, 0, len(authFiles.Files)),
	}
	for _, authFile := range authFiles.Files {
		if !strings.EqualFold(strings.TrimSpace(authFile.Provider), "antigravity") {
			continue
		}
		account := quotaAccount{
			Account:       maskEmail(firstText(authFile.Email, authFile.Label, "Antigravity account")),
			Status:        normalizedAccountStatus(authFile.Status, authFile.Disabled, authFile.Unavailable),
			StatusMessage: strings.TrimSpace(authFile.StatusMessage),
		}
		if authFile.Disabled || authFile.Unavailable {
			account.Error = "账号当前不可用"
			report.Accounts = append(report.Accounts, account)
			continue
		}
		projectID := strings.TrimSpace(authFile.ProjectID)
		if projectID == "" || strings.TrimSpace(authFile.AuthIndex) == "" {
			account.Error = "账号缺少额度查询所需的本地索引或项目字段"
			report.Accounts = append(report.Accounts, account)
			continue
		}

		summary, errSummary := a.retrieveQuotaSummary(current.Port, authFile.AuthIndex, projectID)
		if errSummary != nil {
			account.Error = errSummary.Error()
		} else {
			account.Groups = convertQuotaGroups(summary)
		}
		plan, credits, errCredits := a.retrievePlanAndCredits(current.Port, authFile.AuthIndex)
		if errCredits == nil {
			account.Plan = plan
			account.Credits = credits
		} else if account.Error == "" {
			account.StatusMessage = "模型额度已读取；套餐与 AI Credits 暂时不可用"
		}
		report.Accounts = append(report.Accounts, account)
	}
	if len(report.Accounts) == 0 {
		return a.cachedQuotaReport(errors.New("没有可查询的 Antigravity 账号"))
	}
	sort.Slice(report.Accounts, func(i, j int) bool { return report.Accounts[i].Account < report.Accounts[j].Account })
	if errSave := a.saveQuotaCache(report); errSave != nil {
		report.Warning = "额度读取成功，但本地缓存保存失败"
	}
	return report, nil
}

func (a *app) retrieveQuotaSummary(port int, authIndex, projectID string) (upstreamQuotaSummary, error) {
	data, errJSON := json.Marshal(map[string]string{"project": projectID})
	if errJSON != nil {
		return upstreamQuotaSummary{}, errJSON
	}
	endpoints := []string{
		"https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
	}
	var lastErr error
	for _, endpoint := range endpoints {
		body, errCall := a.managementAPICall(port, authIndex, endpoint, string(data))
		if errCall != nil {
			lastErr = errCall
			continue
		}
		var summary upstreamQuotaSummary
		if errDecode := json.Unmarshal(body, &summary); errDecode != nil {
			lastErr = fmt.Errorf("额度响应不是有效 JSON")
			continue
		}
		if len(summary.Buckets) == 0 && len(summary.Groups) == 0 {
			lastErr = fmt.Errorf("Antigravity 未返回模型额度桶")
			continue
		}
		return summary, nil
	}
	if lastErr == nil {
		lastErr = errors.New("额度端点不可用")
	}
	return upstreamQuotaSummary{}, lastErr
}

func (a *app) retrievePlanAndCredits(port int, authIndex string) (string, *creditInfo, error) {
	body, errCall := a.managementAPICall(port, authIndex, "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", `{"metadata":{"ideType":"ANTIGRAVITY"}}`)
	if errCall != nil {
		return "", nil, errCall
	}
	var payload map[string]any
	if errJSON := json.Unmarshal(body, &payload); errJSON != nil {
		return "", nil, errors.New("套餐响应不是有效 JSON")
	}
	plan := extractPlanName(payload)
	paidTier, _ := payload["paidTier"].(map[string]any)
	availableCredits, _ := paidTier["availableCredits"].([]any)
	for _, rawCredit := range availableCredits {
		credit, _ := rawCredit.(map[string]any)
		creditType := stringFromAny(credit["creditType"])
		if !strings.EqualFold(creditType, "GOOGLE_ONE_AI") {
			continue
		}
		amount, amountOK := numberFromAny(credit["creditAmount"])
		minimum, minimumOK := numberFromAny(credit["minimumCreditAmountForUsage"])
		if !amountOK {
			continue
		}
		if !minimumOK {
			minimum = 0
		}
		return plan, &creditInfo{
			Available:     amount >= minimum,
			Amount:        amount,
			Minimum:       minimum,
			CreditType:    "GOOGLE_ONE_AI",
			UpstreamLabel: "Google One AI Credits",
		}, nil
	}
	return plan, nil, nil
}

func (a *app) managementAPICall(port int, authIndex, endpoint, data string) ([]byte, error) {
	payload := map[string]any{
		"auth_index": authIndex,
		"method":     http.MethodPost,
		"url":        endpoint,
		"header": map[string]string{
			"Authorization": "Bearer $TOKEN$",
			"Accept":        "*/*",
			"Content-Type":  "application/json",
			"User-Agent":    antigravityQuotaUserAgent,
		},
		"data": data,
	}
	var result managementAPICallResponse
	if err := a.managementJSON(port, http.MethodPost, "/v0/management/api-call", payload, &result); err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("上游额度接口返回 HTTP %d", result.StatusCode)
	}
	return []byte(result.Body), nil
}

func (a *app) managementJSON(port int, method, path string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		raw, errJSON := json.Marshal(payload)
		if errJSON != nil {
			return errJSON
		}
		body = bytes.NewReader(raw)
	}
	req, errRequest := http.NewRequest(method, a.gatewayURL(port)+path, body)
	if errRequest != nil {
		return errRequest
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("X-Management-Key", a.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, errDo := httpClient(75 * time.Second).Do(req)
	if errDo != nil {
		return errDo
	}
	defer resp.Body.Close()
	raw, errRead := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if errRead != nil {
		return errRead
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("本机额度代理返回 HTTP %d", resp.StatusCode)
	}
	if target == nil {
		return nil
	}
	if errJSON := json.Unmarshal(raw, target); errJSON != nil {
		return fmt.Errorf("本机额度代理响应无效: %w", errJSON)
	}
	return nil
}

func convertQuotaGroups(summary upstreamQuotaSummary) []quotaGroup {
	groups := make([]quotaGroup, 0, len(summary.Groups)+1)
	for _, upstreamGroup := range summary.Groups {
		buckets := convertQuotaBuckets(upstreamGroup.Buckets)
		if len(buckets) == 0 {
			continue
		}
		groups = append(groups, quotaGroup{
			Name:        localizedGroupName(upstreamGroup.DisplayName),
			Description: strings.TrimSpace(upstreamGroup.Description),
			Buckets:     buckets,
		})
	}
	if buckets := convertQuotaBuckets(summary.Buckets); len(buckets) > 0 {
		groups = append(groups, quotaGroup{Name: "Gemini 模型", Description: strings.TrimSpace(summary.Description), Buckets: buckets})
	}
	return groups
}

func convertQuotaBuckets(source []upstreamQuotaBucket) []quotaBucket {
	buckets := make([]quotaBucket, 0, len(source))
	seen := make(map[string]bool)
	for _, upstream := range source {
		name := localizedBucketName(upstream)
		key := strings.ToLower(strings.TrimSpace(firstText(upstream.BucketID, name, upstream.ModelID)))
		if key != "" && seen[key] {
			continue
		}
		seen[key] = true
		var percent *float64
		if upstream.RemainingFraction != nil {
			value := *upstream.RemainingFraction
			if value >= 0 && value <= 1.000001 {
				value *= 100
			}
			value = math.Max(0, math.Min(100, value))
			value = math.Round(value*10) / 10
			percent = &value
		}
		resetTime := parseQuotaTime(upstream.ResetTime)
		buckets = append(buckets, quotaBucket{
			ID:               strings.TrimSpace(upstream.BucketID),
			Name:             name,
			Description:      strings.TrimSpace(upstream.Description),
			Window:           strings.TrimSpace(upstream.Window),
			RemainingPercent: percent,
			RemainingAmount:  upstream.RemainingAmount,
			ResetTime:        resetTime,
			Disabled:         upstream.Disabled,
		})
	}
	return buckets
}

func localizedBucketName(bucket upstreamQuotaBucket) string {
	raw := firstText(bucket.DisplayName, bucket.Window, bucket.BucketID, bucket.ModelID, "模型额度")
	lower := strings.ToLower(strings.Join([]string{bucket.DisplayName, bucket.Description, bucket.Window, bucket.BucketID}, " "))
	switch {
	case strings.Contains(lower, "week") || strings.Contains(lower, "7 day") || strings.Contains(lower, "7-day"):
		return "每周剩余额度"
	case strings.Contains(lower, "five hour") || strings.Contains(lower, "five-hour") || strings.Contains(lower, "5 hour") || strings.Contains(lower, "5-hour") || strings.Contains(lower, "5h"):
		return "5 小时剩余额度"
	default:
		return strings.TrimSpace(raw)
	}
}

func localizedGroupName(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.Contains(lower, "gemini") {
		return "Gemini 模型"
	}
	if value == "" {
		return "模型额度"
	}
	return value
}

func normalizedAccountStatus(status string, disabled, unavailable bool) string {
	if disabled {
		return "disabled"
	}
	if unavailable {
		return "unavailable"
	}
	status = strings.TrimSpace(status)
	if status == "" {
		return "ready"
	}
	return status
}

func extractPlanName(payload map[string]any) string {
	for _, key := range []string{"paidTier", "currentTier"} {
		tier, _ := payload[key].(map[string]any)
		for _, field := range []string{"displayName", "name", "id"} {
			if value := strings.TrimSpace(stringFromAny(tier[field])); value != "" {
				return friendlyPlanName(value)
			}
		}
	}
	return ""
}

func friendlyPlanName(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(value))
	switch {
	case strings.Contains(upper, "G1_ULTRA"):
		return "Google AI Ultra"
	case strings.Contains(upper, "G1_PRO") || strings.Contains(upper, "G1_PLUS"):
		return "Google AI Pro"
	case strings.Contains(upper, "FREE"):
		return "Free"
	}
	return strings.TrimSpace(value)
}

func parseQuotaTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func maskEmail(value string) string {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return value
	}
	name := []rune(parts[0])
	if len(name) <= 2 {
		return string(name[:1]) + "***@" + parts[1]
	}
	return string(name[:1]) + strings.Repeat("*", minInt(5, len(name)-2)) + string(name[len(name)-1:]) + "@" + parts[1]
}

func firstText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func numberFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *app) saveQuotaCache(report quotaReport) error {
	raw, errJSON := json.MarshalIndent(report, "", "  ")
	if errJSON != nil {
		return errJSON
	}
	raw = append(raw, '\n')
	return writeAtomic(a.quotaCachePath(), raw, 0o600)
}

func (a *app) cachedQuotaReport(cause error) (quotaReport, error) {
	raw, errRead := os.ReadFile(a.quotaCachePath())
	if errRead != nil {
		return quotaReport{}, cause
	}
	var cached quotaReport
	if errJSON := json.Unmarshal(raw, &cached); errJSON != nil {
		return quotaReport{}, cause
	}
	cached.Stale = true
	cached.Warning = "实时刷新失败，正在显示上次成功缓存: " + cause.Error()
	return cached, nil
}
