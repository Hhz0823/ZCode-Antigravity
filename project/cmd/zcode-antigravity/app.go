package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	providerID   = "zcode-antigravity-local"
	providerName = "Google"
)

type providerAccountCounts struct {
	Antigravity int `json:"antigravity"`
	XAI         int `json:"xai"`
}

func (c providerAccountCounts) total() int {
	return c.Antigravity + c.XAI
}

type settings struct {
	PreferredPort         int    `json:"preferredPort"`
	PortScanEnd           int    `json:"portScanEnd"`
	CallbackPreferredPort int    `json:"callbackPreferredPort"`
	CallbackScanEnd       int    `json:"callbackScanEnd"`
	ProxyURL              string `json:"proxyURL"`
	RoutingStrategy       string `json:"routingStrategy"`
	SessionAffinity       bool   `json:"sessionAffinity"`
	SessionAffinityTTL    string `json:"sessionAffinityTTL"`
	RequestRetry          int    `json:"requestRetry"`
	MaxRetryCredentials   int    `json:"maxRetryCredentials"`
	MaxRetryInterval      int    `json:"maxRetryInterval"`
	AutoRefreshMinutes    int    `json:"autoRefreshMinutes"`
	QuotaWarningPercent   int    `json:"quotaWarningPercent"`
	EnableGrokModels      bool   `json:"enableGrokModels"`
	EnableOtherModels     bool   `json:"enableOtherModels"`
	BackgroundModel       string `json:"backgroundModel"`
	Theme                 string `json:"theme"`
	LiquidGlass           bool   `json:"liquidGlass"`
}

type paths struct {
	Root          string
	Data          string
	Backend       string
	Config        string
	AuthDir       string
	LogsDir       string
	ConsoleLog    string
	State         string
	Secret        string
	Lock          string
	Settings      string
	UserSettings  string
	ZCodeConfig   string
	ZCodeBackups  string
	PackageReadme string
	UsageMetrics  string
}

type app struct {
	paths        paths
	settings     settings
	settingsMu   sync.RWMutex
	proxyMu      sync.RWMutex
	activeProxy  string
	proxySource  string
	apiKey       string
	now          func() time.Time
	zcodeRunning func() bool
	guiMode      bool
}

type state struct {
	Port           int       `json:"port"`
	PID            int       `json:"pid"`
	BackendPath    string    `json:"backendPath"`
	StartedAt      time.Time `json:"startedAt"`
	ZCodeConfig    string    `json:"zcodeConfig,omitempty"`
	ZCodeBackup    string    `json:"zcodeBackup,omitempty"`
	Models         []string  `json:"models,omitempty"`
	Launcher       string    `json:"launcherVersion"`
	LastHealthTime time.Time `json:"lastHealthTime,omitempty"`
}

func defaultSettings() settings {
	return settings{
		PreferredPort:         18080,
		PortScanEnd:           18180,
		CallbackPreferredPort: 51121,
		CallbackScanEnd:       51221,
		RoutingStrategy:       "round-robin",
		SessionAffinity:       true,
		SessionAffinityTTL:    "2h",
		RequestRetry:          2,
		MaxRetryCredentials:   2,
		MaxRetryInterval:      20,
		AutoRefreshMinutes:    5,
		QuotaWarningPercent:   20,
		EnableGrokModels:      false,
		EnableOtherModels:     false,
		BackgroundModel:       "gemini-3.6-flash",
		Theme:                 "system",
		LiquidGlass:           true,
	}
}

func newApp(rootOverride string) (*app, error) {
	root := strings.TrimSpace(rootOverride)
	if root == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("定位程序目录: %w", err)
		}
		root = filepath.Dir(exe)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析程序目录: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("定位用户目录: %w", err)
	}
	zcodeConfig, zcodeBackups, err := resolveZCodePaths(home)
	if err != nil {
		return nil, err
	}
	backendName := "cli-proxy-api"
	if runtime.GOOS == "windows" {
		backendName += ".exe"
	}
	resourceRoot := absRoot
	if runtime.GOOS == "darwin" && filepath.Base(absRoot) == "MacOS" {
		candidate := filepath.Clean(filepath.Join(absRoot, "..", "Resources"))
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			resourceRoot = candidate
		}
	}
	runtimeData, err := resolveRuntimeDataDir(home)
	if err != nil {
		return nil, err
	}
	p := paths{
		Root:          absRoot,
		Data:          runtimeData,
		Backend:       filepath.Join(absRoot, "backend", backendName),
		Settings:      filepath.Join(resourceRoot, "settings.json"),
		ZCodeConfig:   zcodeConfig,
		ZCodeBackups:  zcodeBackups,
		PackageReadme: filepath.Join(resourceRoot, packageReadmeName()),
	}
	p.Config = filepath.Join(p.Data, "config.yaml")
	p.AuthDir = filepath.Join(p.Data, "auth")
	p.LogsDir = filepath.Join(p.Data, "logs")
	p.ConsoleLog = filepath.Join(p.LogsDir, "gateway-console.log")
	p.State = filepath.Join(p.Data, "state.json")
	p.Secret = filepath.Join(p.Data, "local-api-key")
	p.Lock = filepath.Join(p.Data, "manager.lock")
	p.UsageMetrics = filepath.Join(p.Data, "usage-metrics.json")
	p.UserSettings = filepath.Join(p.Data, "manager-settings.json")

	for _, dir := range []string{p.Data, p.AuthDir, p.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("创建目录 %s: %w", dir, err)
		}
	}
	s, err := loadSettings(p.Settings)
	if err != nil {
		return nil, err
	}
	s, err = overlaySettings(p.UserSettings, s)
	if err != nil {
		return nil, err
	}
	key, err := loadOrCreateAPIKey(p.Secret)
	if err != nil {
		return nil, err
	}
	return &app{paths: p, settings: s, apiKey: key, now: time.Now, zcodeRunning: isZCodeRunning}, nil
}

func packageReadmeName() string {
	if runtime.GOOS == "darwin" {
		return "README-macOS.txt"
	}
	return "README-Windows.txt"
}

func resolveRuntimeDataDir(userHome string) (string, error) {
	if override := strings.TrimSpace(os.Getenv("ZCODE_ANTIGRAVITY_DATA_DIR")); override != "" {
		path, err := filepath.Abs(os.ExpandEnv(override))
		if err != nil {
			return "", fmt.Errorf("解析 ZCODE_ANTIGRAVITY_DATA_DIR: %w", err)
		}
		return path, nil
	}
	if runtime.GOOS == "windows" {
		if local := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); local != "" {
			path, err := filepath.Abs(local)
			if err != nil {
				return "", fmt.Errorf("解析 LOCALAPPDATA: %w", err)
			}
			return filepath.Join(path, "ZCodeAntigravity"), nil
		}
	}
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		configDir = filepath.Join(userHome, ".config")
	}
	path, err := filepath.Abs(configDir)
	if err != nil {
		return "", fmt.Errorf("解析本地应用数据目录: %w", err)
	}
	return filepath.Join(path, "ZCodeAntigravity"), nil
}

func resolveZCodePaths(userHome string) (configPath, backupsPath string, err error) {
	base, found, err := readZCodeDataBaseDir(userHome)
	if err != nil {
		return "", "", err
	}
	if !found || base == "" {
		base = strings.TrimSpace(os.Getenv("ZCODE_DATA_BASE_DIR"))
	}
	if base == "" {
		base = strings.TrimSpace(os.Getenv("HOME"))
	}
	if base == "" {
		base = userHome
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", "", fmt.Errorf("解析 ZCode 数据目录: %w", err)
	}
	base = absBase
	seen := map[string]bool{filepath.Clean(base): true}
	for hop := 0; hop < 4; hop++ {
		next, exists, readErr := readZCodeDataBaseDir(base)
		if readErr != nil {
			return "", "", readErr
		}
		if !exists || next == "" {
			v2 := filepath.Join(base, ".zcode", "v2")
			return filepath.Join(v2, "config.json"), filepath.Join(v2, "backups", "zcode-antigravity"), nil
		}
		nextAbs, absErr := filepath.Abs(next)
		if absErr != nil {
			return "", "", fmt.Errorf("解析 ZCode dataBaseDir: %w", absErr)
		}
		nextAbs = filepath.Clean(nextAbs)
		if nextAbs == filepath.Clean(base) {
			v2 := filepath.Join(base, ".zcode", "v2")
			return filepath.Join(v2, "config.json"), filepath.Join(v2, "backups", "zcode-antigravity"), nil
		}
		if seen[nextAbs] {
			return "", "", fmt.Errorf("ZCode dataBaseDir 存在循环指向，拒绝猜测配置位置")
		}
		seen[nextAbs] = true
		base = nextAbs
	}
	return "", "", fmt.Errorf("ZCode dataBaseDir 连续跳转超过 4 次，拒绝自动修改")
}

func readZCodeDataBaseDir(base string) (value string, exists bool, err error) {
	settingPath := filepath.Join(base, ".zcode", "v2", "setting.json")
	raw, readErr := os.ReadFile(settingPath)
	if errors.Is(readErr, fs.ErrNotExist) {
		return "", false, nil
	}
	if readErr != nil {
		return "", false, fmt.Errorf("读取 ZCode setting.json: %w", readErr)
	}
	var setting map[string]any
	if decodeErr := json.Unmarshal(raw, &setting); decodeErr != nil {
		return "", false, fmt.Errorf("ZCode setting.json 损坏，无法安全确定数据目录: %w", decodeErr)
	}
	rawValue, exists := setting["dataBaseDir"]
	if !exists || rawValue == nil {
		return "", false, nil
	}
	text, ok := rawValue.(string)
	if !ok {
		return "", false, fmt.Errorf("ZCode setting.json 的 dataBaseDir 不是字符串")
	}
	text = strings.TrimSpace(os.ExpandEnv(text))
	if text == "" {
		return "", false, nil
	}
	return text, true, nil
}

func loadSettings(path string) (settings, error) {
	s := defaultSettings()
	return overlaySettings(path, s)
}

func overlaySettings(path string, base settings) (settings, error) {
	s := base
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("读取 settings.json: %w", err)
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("settings.json 不是有效 JSON: %w", err)
	}
	s.RoutingStrategy = strings.ToLower(strings.TrimSpace(s.RoutingStrategy))
	s.SessionAffinityTTL = strings.TrimSpace(s.SessionAffinityTTL)
	s.ProxyURL = strings.TrimSpace(s.ProxyURL)
	s.BackgroundModel = strings.TrimSpace(s.BackgroundModel)
	s.Theme = strings.ToLower(strings.TrimSpace(s.Theme))
	if err := validateSettings(s); err != nil {
		return s, err
	}
	return s, nil
}

func validateSettings(s settings) error {
	if s.PreferredPort < 1024 || s.PreferredPort > 65535 || s.PortScanEnd < s.PreferredPort || s.PortScanEnd > 65535 || s.PortScanEnd-s.PreferredPort > 1000 {
		return fmt.Errorf("settings.json 的 API 端口范围无效")
	}
	if s.CallbackPreferredPort < 1024 || s.CallbackPreferredPort > 65535 || s.CallbackScanEnd < s.CallbackPreferredPort || s.CallbackScanEnd > 65535 || s.CallbackScanEnd-s.CallbackPreferredPort > 1000 {
		return fmt.Errorf("settings.json 的 OAuth 回调端口范围无效")
	}
	if err := validateProxyURL(s.ProxyURL); err != nil {
		return err
	}
	if s.RoutingStrategy != "round-robin" && s.RoutingStrategy != "weighted-round-robin" && s.RoutingStrategy != "fill-first" {
		return fmt.Errorf("settings.json 的 routingStrategy 无效")
	}
	if strings.TrimSpace(s.SessionAffinityTTL) == "" || len(s.SessionAffinityTTL) > 16 {
		return fmt.Errorf("settings.json 的 sessionAffinityTTL 无效")
	}
	if s.RequestRetry < 0 || s.RequestRetry > 10 || s.MaxRetryCredentials < 0 || s.MaxRetryCredentials > 20 || s.MaxRetryInterval < 1 || s.MaxRetryInterval > 300 {
		return fmt.Errorf("settings.json 的重试设置无效")
	}
	if s.AutoRefreshMinutes < 1 || s.AutoRefreshMinutes > 60 {
		return fmt.Errorf("settings.json 的 autoRefreshMinutes 必须在 1-60 之间")
	}
	if s.QuotaWarningPercent < 1 || s.QuotaWarningPercent > 90 {
		return fmt.Errorf("settings.json 的 quotaWarningPercent 必须在 1-90 之间")
	}
	if len(strings.TrimSpace(s.BackgroundModel)) > 160 {
		return fmt.Errorf("settings.json 的 backgroundModel 过长")
	}
	if s.Theme != "system" && s.Theme != "light" && s.Theme != "dark" {
		return fmt.Errorf("settings.json 的 theme 无效")
	}
	return nil
}

func (a *app) currentSettings() settings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings
}

func (a *app) saveUserSettings(s settings) error {
	s.RoutingStrategy = strings.ToLower(strings.TrimSpace(s.RoutingStrategy))
	s.SessionAffinityTTL = strings.TrimSpace(s.SessionAffinityTTL)
	s.ProxyURL = strings.TrimSpace(s.ProxyURL)
	s.BackgroundModel = strings.TrimSpace(s.BackgroundModel)
	s.Theme = strings.ToLower(strings.TrimSpace(s.Theme))
	if err := validateSettings(s); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(a.paths.UserSettings, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("保存管理设置: %w", err)
	}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	a.proxyMu.Lock()
	a.activeProxy, a.proxySource = "", ""
	a.proxyMu.Unlock()
	return nil
}

func validateProxyURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" {
		return fmt.Errorf("settings.json 的 proxyURL 无效")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5":
		return nil
	default:
		return fmt.Errorf("settings.json 的 proxyURL 只支持 http、https、socks5")
	}
}

func loadOrCreateAPIKey(path string) (string, error) {
	if raw, err := os.ReadFile(path); err == nil {
		key := strings.TrimSpace(string(raw))
		if len(key) < 32 {
			return "", fmt.Errorf("本地 API key 文件过短，请检查 %s", path)
		}
		return key, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("读取本地 API key: %w", err)
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成本地 API key: %w", err)
	}
	key := base64.RawURLEncoding.EncodeToString(random)
	if err := writeFileExclusive(path, []byte(key+"\n"), 0o600); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return loadOrCreateAPIKey(path)
		}
		return "", fmt.Errorf("保存本地 API key: %w", err)
	}
	return key, nil
}

func writeFileExclusive(path string, data []byte, mode fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func (a *app) loadState() (state, error) {
	var s state
	raw, err := os.ReadFile(a.paths.State)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("读取状态文件: %w", err)
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("状态文件损坏（不会自动覆盖）: %w", err)
	}
	return s, nil
}

func (a *app) saveState(s state) error {
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeAtomic(a.paths.State, raw, 0o600)
}

func (a *app) setup() error {
	fmt.Println("\n风险提示：这是非官方第三方桥接，会调用未公开的 Antigravity 接口。Google 可能限流、暂停 API 权限或封禁账号；不要使用主 Gmail/Workspace/Cloud 账号测试。")
	count, err := countAntigravityAccounts(a.paths.AuthDir)
	if err != nil {
		return err
	}
	if count == 0 {
		fmt.Println("\n未发现 Antigravity 登录，开始浏览器 OAuth。")
		if err := a.login(); err != nil {
			return err
		}
	} else {
		fmt.Printf("\n已发现 %d 个 Antigravity 账号文件，跳过重复登录。\n", count)
	}
	return a.startAndConfigure()
}

func countAntigravityAccounts(dir string) (int, error) {
	counts, err := countProviderAccounts(dir)
	return counts.Antigravity, err
}

func countProviderAccounts(dir string) (providerAccountCounts, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return providerAccountCounts{}, nil
	}
	if err != nil {
		return providerAccountCounts{}, err
	}
	var counts providerAccountCounts
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		provider := ""
		switch {
		case strings.HasPrefix(name, "antigravity-"):
			provider = "antigravity"
		case strings.HasPrefix(name, "xai-"):
			provider = "xai"
		}
		if entry.IsDir() || provider == "" || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, errInfo := entry.Info()
		if errInfo != nil {
			return providerAccountCounts{}, fmt.Errorf("检查账号文件 %s: %w", path, errInfo)
		}
		if info.Size() <= 0 || info.Size() > 4<<20 {
			return providerAccountCounts{}, fmt.Errorf("账号文件大小异常: %s", path)
		}
		raw, errRead := os.ReadFile(path)
		if errRead != nil {
			return providerAccountCounts{}, fmt.Errorf("读取账号文件 %s: %w", path, errRead)
		}
		var metadata map[string]any
		if errJSON := json.Unmarshal(raw, &metadata); errJSON != nil {
			return providerAccountCounts{}, fmt.Errorf("账号文件不是有效 JSON %s: %w", path, errJSON)
		}
		authType, _ := metadata["type"].(string)
		accessToken, _ := metadata["access_token"].(string)
		refreshToken, _ := metadata["refresh_token"].(string)
		projectID, _ := metadata["project_id"].(string)
		if !strings.EqualFold(strings.TrimSpace(authType), provider) || strings.TrimSpace(accessToken) == "" || strings.TrimSpace(refreshToken) == "" {
			return providerAccountCounts{}, fmt.Errorf("账号文件缺少 %s 类型或令牌字段: %s", provider, path)
		}
		if provider == "antigravity" && strings.TrimSpace(projectID) == "" {
			return providerAccountCounts{}, fmt.Errorf("账号文件缺少 Antigravity 项目字段: %s", path)
		}
		if provider == "antigravity" {
			counts.Antigravity++
		} else {
			counts.XAI++
		}
	}
	return counts, nil
}

func modelIDs(models []modelInfo) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	sort.Strings(ids)
	return ids
}
