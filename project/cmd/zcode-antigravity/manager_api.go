package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// managerReport is the native UI's privacy-safe management model. Credentials,
// raw prompts and response bodies are deliberately never included.
type managerReport struct {
	Version   string                `json:"version"`
	Accounts  []managerAccount      `json:"accounts"`
	Proxy     managerProxy          `json:"proxy"`
	Routing   managerRouting        `json:"routing"`
	Settings  managerPublicSettings `json:"settings"`
	Features  []managerFeature      `json:"features"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

type managerAccount struct {
	ID       string    `json:"id"`
	Provider string    `json:"provider"`
	Label    string    `json:"label"`
	Plan     string    `json:"plan,omitempty"`
	Status   string    `json:"status"`
	Updated  time.Time `json:"updatedAt"`
}

type managerProxy struct {
	Running   bool              `json:"running"`
	BaseURL   string            `json:"baseURL,omitempty"`
	Port      int               `json:"port,omitempty"`
	Protocols []managerProtocol `json:"protocols"`
}

type managerProtocol struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

type managerRouting struct {
	Strategy           string `json:"strategy"`
	SessionAffinity    bool   `json:"sessionAffinity"`
	SessionAffinityTTL string `json:"sessionAffinityTTL"`
	RequestRetry       int    `json:"requestRetry"`
	CredentialRetry    int    `json:"credentialRetry"`
	RetryInterval      int    `json:"retryInterval"`
	BackgroundModel    string `json:"backgroundModel"`
}

type managerPublicSettings struct {
	AutoRefreshMinutes  int    `json:"autoRefreshMinutes"`
	QuotaWarningPercent int    `json:"quotaWarningPercent"`
	EnableGrokModels    bool   `json:"enableGrokModels"`
	EnableOtherModels   bool   `json:"enableOtherModels"`
	ProxyURL            string `json:"proxyURL"`
	Theme               string `json:"theme"`
	LiquidGlass         bool   `json:"liquidGlass"`
	SettingsPath        string `json:"settingsPath"`
}

type managerFeature struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
}

type managerSettingsUpdate struct {
	RoutingStrategy     *string `json:"routingStrategy,omitempty"`
	SessionAffinity     *bool   `json:"sessionAffinity,omitempty"`
	SessionAffinityTTL  *string `json:"sessionAffinityTTL,omitempty"`
	RequestRetry        *int    `json:"requestRetry,omitempty"`
	MaxRetryCredentials *int    `json:"maxRetryCredentials,omitempty"`
	MaxRetryInterval    *int    `json:"maxRetryInterval,omitempty"`
	AutoRefreshMinutes  *int    `json:"autoRefreshMinutes,omitempty"`
	QuotaWarningPercent *int    `json:"quotaWarningPercent,omitempty"`
	EnableGrokModels    *bool   `json:"enableGrokModels,omitempty"`
	EnableOtherModels   *bool   `json:"enableOtherModels,omitempty"`
	BackgroundModel     *string `json:"backgroundModel,omitempty"`
	Theme               *string `json:"theme,omitempty"`
	LiquidGlass         *bool   `json:"liquidGlass,omitempty"`
	ProxyURL            *string `json:"proxyURL,omitempty"`
}

func (g *guiRuntime) serveManager(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, g.managerReport())
}

func (g *guiRuntime) managerReport() managerReport {
	cfg := g.app.currentSettings()
	accounts, _ := readManagerAccounts(g.app.paths.AuthDir)
	report := managerReport{
		Version:   version,
		Accounts:  accounts,
		UpdatedAt: time.Now().UTC(),
		Proxy: managerProxy{Protocols: []managerProtocol{
			{Name: "OpenAI", Path: "/v1/chat/completions", Description: "Chat Completions / Responses 兼容"},
			{Name: "Anthropic", Path: "/v1/messages", Description: "Claude Code 原生消息协议"},
			{Name: "Gemini", Path: "/v1beta/models", Description: "Google SDK 兼容协议"},
		}},
		Routing: managerRouting{
			Strategy: cfg.RoutingStrategy, SessionAffinity: cfg.SessionAffinity,
			SessionAffinityTTL: cfg.SessionAffinityTTL, RequestRetry: cfg.RequestRetry,
			CredentialRetry: cfg.MaxRetryCredentials, RetryInterval: cfg.MaxRetryInterval,
			BackgroundModel: cfg.BackgroundModel,
		},
		Settings: managerPublicSettings{
			AutoRefreshMinutes: cfg.AutoRefreshMinutes, QuotaWarningPercent: cfg.QuotaWarningPercent,
			EnableGrokModels: cfg.EnableGrokModels, EnableOtherModels: cfg.EnableOtherModels,
			ProxyURL: redactURLUserinfo(cfg.ProxyURL), Theme: cfg.Theme, LiquidGlass: cfg.LiquidGlass,
			SettingsPath: g.app.paths.UserSettings,
		},
		Features: []managerFeature{
			{ID: "accounts", Name: "多账号管家", Description: "OAuth 登录、账号发现和脱敏状态", Available: true},
			{ID: "google-claude", Name: "Google Claude", Description: "使用 Antigravity Google 账号调用 Claude Sonnet / Opus", Available: true},
			{ID: "auto-proxy", Name: "v2rayN 自动代理", Description: "无需 TUN，自动发现系统代理与本机端口", Available: true},
			{ID: "protocols", Name: "三协议中继", Description: "OpenAI、Anthropic、Gemini", Available: true},
			{ID: "connectors", Name: "Agent 一键接入", Description: "备份并合并 8 类 CLI 配置", Available: true},
			{ID: "routing", Name: "模型路由", Description: "轮询、加权与填满优先", Available: true},
			{ID: "retry", Name: "自动自愈", Description: "401/429 重试与凭据轮换", Available: true},
			{ID: "usage", Name: "用量统计", Description: "输出 Token、推理 Token 与 tok/s", Available: true},
			{ID: "multimodal", Name: "多模态", Description: "由本地网关透传图片输入", Available: true},
		},
	}
	if current, err := g.app.loadState(); err == nil && current.Port > 0 && g.app.probeGateway(current.Port) == nil {
		report.Proxy.Running = true
		report.Proxy.Port = current.Port
		report.Proxy.BaseURL = g.app.gatewayURL(current.Port)
	}
	return report
}

func (g *guiRuntime) serveManagerSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var update managerSettingsUpdate
	decoder := json.NewDecoder(io.LimitReader(r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "设置请求无效"})
		return
	}
	cfg := g.app.currentSettings()
	if update.RoutingStrategy != nil {
		cfg.RoutingStrategy = *update.RoutingStrategy
	}
	if update.SessionAffinity != nil {
		cfg.SessionAffinity = *update.SessionAffinity
	}
	if update.SessionAffinityTTL != nil {
		cfg.SessionAffinityTTL = *update.SessionAffinityTTL
	}
	if update.RequestRetry != nil {
		cfg.RequestRetry = *update.RequestRetry
	}
	if update.MaxRetryCredentials != nil {
		cfg.MaxRetryCredentials = *update.MaxRetryCredentials
	}
	if update.MaxRetryInterval != nil {
		cfg.MaxRetryInterval = *update.MaxRetryInterval
	}
	if update.AutoRefreshMinutes != nil {
		cfg.AutoRefreshMinutes = *update.AutoRefreshMinutes
	}
	if update.QuotaWarningPercent != nil {
		cfg.QuotaWarningPercent = *update.QuotaWarningPercent
	}
	if update.EnableGrokModels != nil {
		cfg.EnableGrokModels = *update.EnableGrokModels
	}
	if update.EnableOtherModels != nil {
		cfg.EnableOtherModels = *update.EnableOtherModels
	}
	if update.BackgroundModel != nil {
		cfg.BackgroundModel = *update.BackgroundModel
	}
	if update.Theme != nil {
		cfg.Theme = *update.Theme
	}
	if update.LiquidGlass != nil {
		cfg.LiquidGlass = *update.LiquidGlass
	}
	if update.ProxyURL != nil {
		cfg.ProxyURL = *update.ProxyURL
	}
	if err := g.app.saveUserSettings(cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, g.managerReport())
}

func readManagerAccounts(dir string) ([]managerAccount, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []managerAccount{}, nil
	}
	if err != nil {
		return nil, err
	}
	accounts := make([]managerAccount, 0, len(entries))
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		provider := ""
		if strings.HasPrefix(name, "antigravity-") {
			provider = "antigravity"
		}
		if strings.HasPrefix(name, "xai-") {
			provider = "xai"
		}
		if entry.IsDir() || provider == "" || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, errInfo := entry.Info()
		if errInfo != nil || info.Size() <= 0 || info.Size() > 4<<20 {
			continue
		}
		raw, errRead := os.ReadFile(path)
		if errRead != nil {
			continue
		}
		var metadata map[string]any
		if json.Unmarshal(raw, &metadata) != nil {
			continue
		}
		label := firstMetadataString(metadata, "email", "account", "user", "subject")
		if label == "" {
			label = strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if len(label) > 32 {
				label = label[:28] + "…"
			}
		}
		plan := firstMetadataString(metadata, "plan", "subscription", "tier")
		status := "active"
		if disabled, _ := metadata["disabled"].(bool); disabled {
			status = "disabled"
		}
		stableID := sha256.Sum256([]byte(entry.Name()))
		accounts = append(accounts, managerAccount{
			ID: provider + "-" + fmt.Sprintf("%x", stableID[:6]), Provider: provider, Label: redactAccountLabel(label), Plan: plan,
			Status: status, Updated: info.ModTime().UTC(),
		})
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Provider != accounts[j].Provider {
			return accounts[i].Provider < accounts[j].Provider
		}
		return accounts[i].Updated.After(accounts[j].Updated)
	})
	return accounts, nil
}

func firstMetadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func redactAccountLabel(value string) string {
	value = strings.TrimSpace(value)
	if at := strings.IndexByte(value, '@'); at > 1 {
		return value[:1] + strings.Repeat("*", min(6, at-1)) + value[at:]
	}
	if len(value) > 10 {
		return value[:3] + "…" + value[len(value)-3:]
	}
	return value
}
