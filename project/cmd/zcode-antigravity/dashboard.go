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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed dashboard.html
var dashboardHTML string

type guiRuntime struct {
	app          *app
	token        string
	autoSetup    bool
	operationMu  sync.Mutex
	operation    guiOperation
	quotaMu      sync.Mutex
	lastActivity atomic.Int64
	shutdown     func()
}

type guiOperation struct {
	Running     bool      `json:"running"`
	Name        string    `json:"name,omitempty"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt,omitempty"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

type dashboardStatus struct {
	Version   string        `json:"version"`
	Gateway   dashboardItem `json:"gateway"`
	Proxy     dashboardItem `json:"proxy"`
	TUN       dashboardItem `json:"tun"`
	ZCode     dashboardItem `json:"zcode"`
	Accounts  int           `json:"accounts"`
	Models    []string      `json:"models"`
	Operation guiOperation  `json:"operation"`
	UpdatedAt time.Time     `json:"updatedAt"`
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

func (a *app) runGUI(autoSetup bool) error {
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
		app:       a,
		token:     base64.RawURLEncoding.EncodeToString(tokenBytes),
		autoSetup: autoSetup,
	}
	runtime.lastActivity.Store(time.Now().UnixNano())
	mux := http.NewServeMux()
	mux.HandleFunc("/", runtime.serveDashboard)
	mux.HandleFunc("/api/status", runtime.authorized(runtime.serveStatus))
	mux.HandleFunc("/api/quota", runtime.authorized(runtime.serveQuota))
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
	go runtime.stopWhenInactive(shutdownCh)

	address := listener.Addr().(*net.TCPAddr)
	dashboardURL := fmt.Sprintf("http://127.0.0.1:%d/?session=%s", address.Port, url.QueryEscape(runtime.token))
	if autoSetup {
		dashboardURL += "&auto=1"
	}
	if errOpen := launchDashboardWindow(dashboardURL); errOpen != nil {
		runtime.shutdown()
		return fmt.Errorf("打开控制中心窗口: %w", errOpen)
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
	status := dashboardStatus{Version: version, Models: []string{}, UpdatedAt: time.Now().UTC()}
	accounts, errAccounts := countAntigravityAccounts(g.app.paths.AuthDir)
	if errAccounts == nil {
		status.Accounts = accounts
	}

	current, errState := g.app.loadState()
	if errState == nil && current.Port > 0 {
		if errProbe := g.app.probeGateway(current.Port); errProbe == nil {
			status.Gateway = dashboardItem{OK: true, Label: "网关在线", Detail: g.app.gatewayURL(current.Port), Running: true}
			if catalog, errModels := g.app.fetchModels(current.Port); errModels == nil {
				if models, errSelect := selectZCodeModels(catalog); errSelect == nil {
					status.Models = modelIDs(models)
				}
			}
		} else {
			status.Gateway = dashboardItem{Label: "网关离线", Detail: "点击“一键接入 ZCode”启动"}
		}
	} else {
		status.Gateway = dashboardItem{Label: "网关未启动", Detail: "点击“一键接入 ZCode”启动"}
	}

	proxyURL := strings.TrimSpace(g.app.settings.ProxyURL)
	if runtime.GOOS == "darwin" && proxyURL == "" {
		status.Proxy = dashboardItem{OK: true, Label: "使用系统网络 / TUN", Detail: "未固定专用代理", Running: true}
	} else if reachable, detail := proxyEndpointStatus(proxyURL); reachable {
		status.Proxy = dashboardItem{OK: true, Label: "本机代理在线", Detail: detail, Running: true}
	} else {
		status.Proxy = dashboardItem{Label: "本机代理不可用", Detail: detail}
	}
	if name, ok := detectTunAdapter(); ok {
		status.TUN = dashboardItem{OK: true, Label: "TUN 已开启", Detail: name, Running: true}
	} else {
		detail := "请在 v2rayN 中开启 TUN 模式"
		if runtime.GOOS == "darwin" {
			detail = "未发现活动的 utun；如使用直连或显式代理可忽略"
		}
		status.TUN = dashboardItem{Label: "TUN 未检测到", Detail: detail}
	}

	configured, baseURL, errProvider := zcodeProviderStatus(g.app.paths.ZCodeConfig)
	if errProvider == nil && configured {
		status.ZCode = dashboardItem{OK: true, Label: "ZCode 已接入", Detail: baseURL, Running: g.app.zcodeRunning()}
	} else if errProvider != nil {
		status.ZCode = dashboardItem{Label: "ZCode 配置不可读", Detail: errProvider.Error(), Running: g.app.zcodeRunning()}
	} else {
		status.ZCode = dashboardItem{Label: "ZCode 尚未接入", Detail: "只会注入 Gemini 3.7 Flash 与 3.6 Flash", Running: g.app.zcodeRunning()}
	}

	g.operationMu.Lock()
	status.Operation = g.operation
	g.operationMu.Unlock()
	return status
}

func proxyEndpointStatus(value string) (bool, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, "settings.json 未配置专用代理"
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
	report, errQuota := g.app.fetchQuotaReport()
	if errQuota != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errQuota.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
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
	if action == "open-zcode" {
		if errOpen := openZCodeApplication(); errOpen != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errOpen.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if !map[string]bool{"setup": true, "login": true, "sync": true, "stop": true}[action] {
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
		switch action {
		case "setup":
			errOperation = g.app.setup()
			if errOperation == nil {
				errOperation = openZCodeApplication()
			}
		case "login":
			errOperation = g.app.login()
		case "sync":
			errOperation = g.app.startAndConfigure()
		case "stop":
			errOperation = g.app.stop()
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

func operationStartMessage(action string) string {
	switch action {
	case "setup":
		return "正在登录、启动网关并直接接入 ZCode…"
	case "login":
		return "正在打开 Google OAuth 登录…"
	case "sync":
		return "正在同步两个 Gemini 模型…"
	case "stop":
		return "正在安全停止本地网关…"
	default:
		return "正在处理…"
	}
}

func operationSuccessMessage(action string) string {
	switch action {
	case "setup":
		return "ZCode 已接入并启动"
	case "login":
		return "Antigravity 登录成功"
	case "sync":
		return "Gemini 3.7 / 3.6 已同步"
	case "stop":
		return "本地网关已停止"
	default:
		return "操作完成"
	}
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
	go g.shutdown()
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
