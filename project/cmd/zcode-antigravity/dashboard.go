package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed dashboard.html
var dashboardHTML string

type guiRuntime struct {
	app            *app
	token          string
	autoSetup      bool
	operationMu    sync.Mutex
	operation      guiOperation
	quotaMu        sync.Mutex
	providerMu     sync.RWMutex
	provider       string
	trayRefresh    chan struct{}
	usage          *usageTracker
	persistentTray bool
	lastActivity   atomic.Int64
	shutdown       func()
}

type guiOperation struct {
	Running             bool                    `json:"running"`
	Name                string                  `json:"name,omitempty"`
	Message             string                  `json:"message,omitempty"`
	Error               string                  `json:"error,omitempty"`
	DeviceAuthorization *xaiDeviceAuthorization `json:"deviceAuthorization,omitempty"`
	StartedAt           time.Time               `json:"startedAt,omitempty"`
	CompletedAt         time.Time               `json:"completedAt,omitempty"`
}

type dashboardStatus struct {
	Version          string                `json:"version"`
	Gateway          dashboardItem         `json:"gateway"`
	Proxy            dashboardItem         `json:"proxy"`
	TUN              dashboardItem         `json:"tun"`
	ZCode            dashboardItem         `json:"zcode"`
	Accounts         int                   `json:"accounts"`
	ProviderAccounts providerAccountCounts `json:"providerAccounts"`
	SelectedProvider string                `json:"selectedProvider"`
	Models           []string              `json:"models"`
	Operation        guiOperation          `json:"operation"`
	UpdatedAt        time.Time             `json:"updatedAt"`
}

type dashboardItem struct {
	OK      bool   `json:"ok"`
	Label   string `json:"label"`
	Detail  string `json:"detail,omitempty"`
	Running bool   `json:"running,omitempty"`
}

type guiActionRequest struct {
	Action string `json:"action"`
}

type guiProviderRequest struct {
	Provider string `json:"provider"`
}

func (a *app) runGUI(autoSetup bool) error {
	return a.runUIHost(autoSetup, true)
}

func (a *app) runNativeHost(autoSetup bool) error {
	return a.runUIHost(autoSetup, false)
}

func (a *app) runUIHost(autoSetup, launchBrowser bool) error {
	a.guiMode = true
	tokenBytes := make([]byte, 32)
	if _, errRand := rand.Read(tokenBytes); errRand != nil {
		return fmt.Errorf("生成界面会话密钥: %w", errRand)
	}
	listener, errListen := listenDashboard()
	if errListen != nil {
		return errListen
	}

	runtime := &guiRuntime{
		app:            a,
		token:          base64.RawURLEncoding.EncodeToString(tokenBytes),
		autoSetup:      autoSetup,
		provider:       preferredInitialProvider(a.paths.AuthDir),
		trayRefresh:    make(chan struct{}, 1),
		usage:          newUsageTracker(a.paths.UsageMetrics),
		persistentTray: launchBrowser && platformTraySupported(),
	}
	runtime.lastActivity.Store(time.Now().UnixNano())
	mux := http.NewServeMux()
	mux.HandleFunc("/", runtime.serveDashboard)
	mux.HandleFunc("/api/status", runtime.authorized(runtime.serveStatus))
	mux.HandleFunc("/api/quota", runtime.authorized(runtime.serveQuota))
	mux.HandleFunc("/api/usage", runtime.authorized(runtime.serveUsage))
	mux.HandleFunc("/api/provider", runtime.authorized(runtime.serveProvider))
	mux.HandleFunc("/api/connectors", runtime.authorized(runtime.serveConnectors))
	mux.HandleFunc("/api/manager", runtime.authorized(runtime.serveManager))
	mux.HandleFunc("/api/manager/settings", runtime.authorized(runtime.serveManagerSettings))
	mux.HandleFunc("/api/action", runtime.authorized(runtime.serveAction))
	mux.HandleFunc("/api/heartbeat", runtime.authorized(runtime.serveHeartbeat))
	mux.HandleFunc("/api/close", runtime.authorized(runtime.serveClose))

	server := &http.Server{
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       45 * time.Second,
	}
	shutdownCh := make(chan struct{})
	var shutdownOnce sync.Once
	runtime.shutdown = func() {
		shutdownOnce.Do(func() {
			close(shutdownCh)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		})
	}

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(listener)
	}()
	go runtime.monitorUsage(shutdownCh)
	go runtime.monitorGatewayRecovery(shutdownCh)
	if launchBrowser && !platformTraySupported() {
		go runtime.stopWhenInactive(shutdownCh)
	}

	address := listener.Addr().(*net.TCPAddr)
	dashboardURL := fmt.Sprintf("http://127.0.0.1:%d/?session=%s", address.Port, url.QueryEscape(runtime.token))
	if autoSetup {
		dashboardURL += "&auto=1"
	}
	if !launchBrowser {
		connection := map[string]string{
			"baseURL":      fmt.Sprintf("http://127.0.0.1:%d", address.Port),
			"session":      runtime.token,
			"dashboardURL": dashboardURL,
		}
		if errEncode := json.NewEncoder(os.Stdout).Encode(connection); errEncode != nil {
			runtime.shutdown()
			return fmt.Errorf("输出原生客户端连接信息: %w", errEncode)
		}
		_ = os.Stdout.Sync()
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			runtime.shutdown()
		}()
	} else {
		if errOpen := launchDashboardWindow(dashboardURL); errOpen != nil {
			runtime.shutdown()
			return fmt.Errorf("打开控制中心窗口: %w", errOpen)
		}
	}
	if launchBrowser && platformTraySupported() {
		go func() {
			errServe := <-serveErrCh
			if errServe != nil && !errors.Is(errServe, http.ErrServerClosed) {
				runtime.shutdown()
			}
		}()
		errTray := runPlatformTray(shutdownCh, trayHooks{
			Open:           func() { _ = launchDashboardWindow(dashboardURL) },
			Refresh:        runtime.traySnapshot,
			SelectProvider: runtime.setProvider,
			Updates:        runtime.trayRefresh,
			Quit:           runtime.shutdown,
		})
		runtime.shutdown()
		if errTray != nil {
			return fmt.Errorf("任务栏小组件: %w", errTray)
		}
		return nil
	}

	select {
	case <-shutdownCh:
		return nil
	case errServe := <-serveErrCh:
		if errServe == nil || errors.Is(errServe, http.ErrServerClosed) {
			return nil
		}
		return errServe
	}
}

func listenDashboard() (net.Listener, error) {
	for port := 18200; port <= 18250; port++ {
		listener, errListen := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if errListen == nil {
			return listener, nil
		}
	}
	return nil, errors.New("控制中心端口 18200-18250 全部被占用")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (g *guiRuntime) authorized(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.touch()
		if subtleTokenMismatch(r.Header.Get("X-ZCAB-Session"), g.token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func subtleTokenMismatch(value, expected string) bool {
	if len(value) != len(expected) || value == "" {
		return true
	}
	var different byte
	for i := range value {
		different |= value[i] ^ expected[i]
	}
	return different != 0
}

func (g *guiRuntime) touch() {
	g.lastActivity.Store(time.Now().UnixNano())
}

func (g *guiRuntime) serveDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/" || subtleTokenMismatch(r.URL.Query().Get("session"), g.token) {
		http.NotFound(w, r)
		return
	}
	g.touch()
	html := strings.ReplaceAll(dashboardHTML, "__SESSION_TOKEN__", strconv.Quote(g.token))
	html = strings.ReplaceAll(html, "__APP_VERSION__", strconv.Quote(version))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, html)
}

func (g *guiRuntime) serveStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, g.status())
}

func (g *guiRuntime) status() dashboardStatus {
	status := dashboardStatus{Version: version, Models: []string{}, SelectedProvider: g.currentProvider(), UpdatedAt: time.Now().UTC()}
	accounts, errAccounts := countProviderAccounts(g.app.paths.AuthDir)
	if errAccounts == nil {
		status.ProviderAccounts = accounts
		if status.SelectedProvider == "xai" {
			status.Accounts = accounts.XAI
		} else {
			status.Accounts = accounts.Antigravity
		}
	}

	current, errState := g.app.loadState()
	if errState == nil && current.Port > 0 {
		if errProbe := g.app.probeGateway(current.Port); errProbe == nil {
			status.Gateway = dashboardItem{OK: true, Label: "网关在线", Detail: g.app.gatewayURL(current.Port), Running: true}
			if catalog, errModels := g.app.fetchModels(current.Port); errModels == nil {
				cfg := g.app.currentSettings()
				includeAntigravity := status.SelectedProvider == "antigravity" && accounts.Antigravity > 0
				includeXAI := status.SelectedProvider == "xai" && accounts.XAI > 0 && cfg.EnableGrokModels
				includeOther := includeAntigravity && cfg.EnableOtherModels
				if models, errSelect := selectAgentModels(catalog, includeAntigravity, includeXAI, includeOther); errSelect == nil {
					status.Models = modelIDs(models)
				}
			}
		} else {
			status.Gateway = dashboardItem{Label: "网关离线", Detail: "点击“一键接入 ZCode”启动"}
		}
	} else {
		status.Gateway = dashboardItem{Label: "网关未启动", Detail: "点击“一键接入 ZCode”启动"}
	}

	proxyURL, source := g.app.activeProxyStatus()
	if reachable, detail := proxyEndpointStatus(proxyURL); reachable {
		label := source
		if strings.TrimSpace(proxyURL) == "" {
			label = "直连网络"
		}
		status.Proxy = dashboardItem{OK: true, Label: label, Detail: detail, Running: true}
	} else {
		status.Proxy = dashboardItem{Label: "本机代理不可用", Detail: detail}
	}
	if name, ok := detectTunAdapter(); ok {
		status.TUN = dashboardItem{OK: true, Label: "TUN 已开启", Detail: name, Running: true}
	} else {
		status.TUN = dashboardItem{OK: true, Label: "TUN 未启用（可选）", Detail: "Gemini / Grok 可使用直连或专用代理", Running: false}
	}

	configured, baseURL, errProvider := zcodeProviderStatus(g.app.paths.ZCodeConfig)
	if errProvider == nil && configured {
		status.ZCode = dashboardItem{OK: true, Label: "ZCode 已接入", Detail: baseURL, Running: g.app.zcodeRunning()}
	} else if errProvider != nil {
		status.ZCode = dashboardItem{Label: "ZCode 配置不可读", Detail: errProvider.Error(), Running: g.app.zcodeRunning()}
	} else {
		status.ZCode = dashboardItem{Label: "ZCode 尚未接入", Detail: "可选择 Gemini 或 Grok 文本模型", Running: g.app.zcodeRunning()}
	}

	g.operationMu.Lock()
	status.Operation = g.operation
	g.operationMu.Unlock()
	return status
}

func proxyEndpointStatus(value string) (bool, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return true, "未发现可用 v2rayN / 系统代理；无需 TUN"
	}
	parsed, errURL := url.Parse(value)
	if errURL != nil || parsed.Host == "" {
		return false, "代理地址无效"
	}
	conn, errDial := net.DialTimeout("tcp", parsed.Host, 900*time.Millisecond)
	if errDial != nil {
		return false, parsed.Host + " 未监听"
	}
	_ = conn.Close()
	return true, parsed.Host
}

func (g *guiRuntime) serveQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	g.quotaMu.Lock()
	defer g.quotaMu.Unlock()
	provider := normalizeProvider(r.URL.Query().Get("provider"))
	if strings.TrimSpace(r.URL.Query().Get("provider")) == "" {
		provider = g.currentProvider()
	}
	report, errQuota := g.app.fetchProviderQuotaReport(provider)
	if errQuota != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errQuota.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func preferredInitialProvider(authDir string) string {
	counts, err := countProviderAccounts(authDir)
	if err == nil && counts.Antigravity == 0 && counts.XAI > 0 {
		return "xai"
	}
	return "antigravity"
}

func (g *guiRuntime) grokModelsEnabled() bool {
	return g.app.currentSettings().EnableGrokModels
}

func (g *guiRuntime) currentProvider() string {
	g.providerMu.RLock()
	provider := normalizeProvider(g.provider)
	g.providerMu.RUnlock()
	if provider == "xai" && !g.grokModelsEnabled() {
		return "antigravity"
	}
	return provider
}

func (g *guiRuntime) setProvider(provider string) {
	g.providerMu.Lock()
	g.provider = normalizeProvider(provider)
	g.providerMu.Unlock()
	if g.trayRefresh != nil {
		select {
		case g.trayRefresh <- struct{}{}:
		default:
		}
	}
}

func (g *guiRuntime) serveProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]string{"provider": g.currentProvider()})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request guiProviderRequest
	if errJSON := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&request); errJSON != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider != "antigravity" && provider != "xai" && provider != "grok" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported provider"})
		return
	}
	if normalizeProvider(provider) == "xai" && !g.grokModelsEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Grok 模型默认关闭；请先在设置中开启 Grok 模型"})
		return
	}
	g.setProvider(provider)
	writeJSON(w, http.StatusOK, map[string]string{"provider": g.currentProvider()})
}

func (g *guiRuntime) serveAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request guiActionRequest
	if errJSON := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&request); errJSON != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action == "login-grok" && !g.grokModelsEnabled() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Grok 模型默认关闭；请先在设置中开启 Grok 模型"})
		return
	}
	if action == "open-zcode" {
		if errOpen := openZCodeApplication(); errOpen != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errOpen.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	_, connectorActionOK := connectorIDFromAction(action)
	if !map[string]bool{"setup": true, "login": true, "login-grok": true, "sync": true, "stop": true}[action] && !connectorActionOK {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported action"})
		return
	}
	if !g.beginOperation(action) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "已有操作正在进行"})
		return
	}
	go g.runOperation(action)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (g *guiRuntime) beginOperation(action string) bool {
	g.operationMu.Lock()
	defer g.operationMu.Unlock()
	if g.operation.Running {
		return false
	}
	g.operation = guiOperation{
		Running:   true,
		Name:      action,
		Message:   operationStartMessage(action),
		StartedAt: time.Now().UTC(),
	}
	return true
}

func (g *guiRuntime) runOperation(action string) {
	release, errLock := g.app.acquireRunLock()
	var errOperation error
	if errLock != nil {
		errOperation = errLock
	} else {
		defer release()
		if connectorID, ok := connectorIDFromAction(action); ok {
			errOperation = g.installAgentConnector(connectorID)
		} else {
			switch action {
			case "setup":
				if g.currentProvider() == "xai" {
					counts, errCounts := countProviderAccounts(g.app.paths.AuthDir)
					if errCounts != nil {
						errOperation = errCounts
					} else if counts.XAI == 0 {
						errOperation = g.app.loginGrokWithDeviceStatus(g.updateXAIDeviceAuthorization)
					}
					if errOperation == nil {
						errOperation = g.app.startAndConfigure()
					}
				} else {
					errOperation = g.app.setup()
				}
				if errOperation == nil {
					errOperation = openZCodeApplication()
				}
			case "login":
				errOperation = g.app.login()
				if errOperation == nil {
					g.setProvider("antigravity")
				}
			case "login-grok":
				errOperation = g.app.loginGrokWithDeviceStatus(g.updateXAIDeviceAuthorization)
				if errOperation == nil {
					g.setProvider("xai")
				}
			case "sync":
				errOperation = g.app.startAndConfigure()
			case "recover":
				errOperation = g.app.startAndConfigure()
			case "stop":
				errOperation = g.app.stop()
			}
		}
	}
	g.operationMu.Lock()
	g.operation.Running = false
	g.operation.CompletedAt = time.Now().UTC()
	if errOperation != nil {
		g.operation.Error = errOperation.Error()
		g.operation.Message = "操作未完成"
	} else {
		g.operation.Message = operationSuccessMessage(action)
	}
	g.operationMu.Unlock()
}

func (g *guiRuntime) updateXAIDeviceAuthorization(authorization xaiDeviceAuthorization) {
	g.operationMu.Lock()
	defer g.operationMu.Unlock()
	if !g.operation.Running || (g.operation.Name != "login-grok" && g.operation.Name != "setup") {
		return
	}
	copy := authorization
	g.operation.DeviceAuthorization = &copy
	g.operation.Message = "请在 xAI 授权页输入软件显示的验证码…"
}

func operationStartMessage(action string) string {
	if connectorID, ok := connectorIDFromAction(action); ok {
		return "正在备份并一键接入 " + connectorDisplayName(connectorID) + "…"
	}
	switch action {
	case "setup":
		return "正在登录所选提供商、启动网关并接入 ZCode…"
	case "login":
		return "正在打开 Google OAuth 登录…"
	case "login-grok":
		return "正在打开 xAI 设备授权…"
	case "sync":
		return "正在同步 Gemini / Grok 模型…"
	case "recover":
		return "检测到网关意外退出，正在自动恢复…"
	case "stop":
		return "正在安全停止本地网关…"
	default:
		return "正在处理…"
	}
}

func operationSuccessMessage(action string) string {
	if connectorID, ok := connectorIDFromAction(action); ok {
		return connectorDisplayName(connectorID) + " 已接入；请新建会话使用当前模型"
	}
	switch action {
	case "setup":
		return "ZCode 已接入并启动"
	case "login":
		return "Antigravity 登录成功"
	case "login-grok":
		return "Grok / xAI 登录成功"
	case "sync":
		return "Gemini / Grok 模型已同步"
	case "recover":
		return "本地网关已自动恢复"
	case "stop":
		return "本地网关已停止"
	default:
		return "操作完成"
	}
}

func gatewayNeedsRecovery(current state, gatewayHealthy, recordedProcessAlive bool, accounts providerAccountCounts, zcodeConfigured bool) bool {
	return current.Port > 0 &&
		current.PID > 0 &&
		!gatewayHealthy &&
		!recordedProcessAlive &&
		accounts.total() > 0 &&
		zcodeConfigured
}

// monitorGatewayRecovery repairs a provider that still points at a recorded
// bridge whose process has exited. An intentional Stop clears state.PID, so it
// is never undone by this monitor.
func (g *guiRuntime) monitorGatewayRecovery(shutdown <-chan struct{}) {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-shutdown:
			return
		case <-timer.C:
			g.recoverGatewayIfStale()
			timer.Reset(15 * time.Second)
		}
	}
}

func (g *guiRuntime) recoverGatewayIfStale() {
	current, errState := g.app.loadState()
	if errState != nil || current.Port <= 0 || current.PID <= 0 {
		return
	}
	gatewayHealthy := g.app.probeGateway(current.Port) == nil
	if gatewayHealthy {
		return
	}
	accounts, errAccounts := countProviderAccounts(g.app.paths.AuthDir)
	if errAccounts != nil {
		return
	}
	configured, _, errProvider := zcodeProviderStatus(g.app.paths.ZCodeConfig)
	if errProvider != nil || !gatewayNeedsRecovery(current, gatewayHealthy, processExists(current.PID), accounts, configured) {
		return
	}
	if !g.beginOperation("recover") {
		return
	}
	go g.runOperation("recover")
}

func (g *guiRuntime) serveHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *guiRuntime) serveClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if !g.persistentTray {
		go g.shutdown()
	}
}

func (g *guiRuntime) traySnapshot() traySnapshot {
	provider := g.currentProvider()
	g.quotaMu.Lock()
	report, err := g.app.fetchProviderQuotaReport(provider)
	g.quotaMu.Unlock()
	if err != nil {
		name := "Antigravity"
		if provider == "xai" {
			name = "Grok"
		}
		return traySnapshot{Provider: provider, Summary: name + " 额度暂不可用", Detail: err.Error()}
	}
	return traySnapshotFromReport(provider, report)
}

func (g *guiRuntime) stopWhenInactive(shutdown <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-shutdown:
			return
		case <-ticker.C:
			last := time.Unix(0, g.lastActivity.Load())
			if time.Since(last) > 3*time.Minute {
				g.shutdown()
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
