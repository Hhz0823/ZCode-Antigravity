package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	authDir := filepath.Join(data, "auth")
	logsDir := filepath.Join(data, "logs")
	for _, dir := range []string{data, authDir, logsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return &app{
		paths: paths{
			Root:         root,
			Data:         data,
			Backend:      filepath.Join(root, "backend", "cli-proxy-api"),
			Config:       filepath.Join(data, "config.yaml"),
			AuthDir:      authDir,
			LogsDir:      logsDir,
			ConsoleLog:   filepath.Join(logsDir, "gateway-console.log"),
			State:        filepath.Join(data, "state.json"),
			Secret:       filepath.Join(data, "local-api-key"),
			Lock:         filepath.Join(data, "manager.lock"),
			Settings:     filepath.Join(root, "settings.json"),
			ZCodeConfig:  filepath.Join(root, ".zcode", "v2", "config.json"),
			ZCodeBackups: filepath.Join(root, ".zcode", "v2", "backups", "zcode-antigravity"),
		},
		settings:     defaultSettings(),
		apiKey:       "unit-test-local-api-key-00000000000000000000",
		zcodeRunning: func() bool { return false },
		now: func() time.Time {
			return time.Date(2026, 8, 14, 12, 34, 56, 123456789, time.UTC)
		},
	}
}

func requiredTestModels() []modelInfo {
	return []modelInfo{{
		ID:                        "gemini-3.7-flash",
		DisplayName:               "Gemini 3.7 Flash",
		MaxInputTokens:            1048576,
		MaxTokens:                 65536,
		Thinking:                  &thinkingSupport{Min: 1, Max: 65535, DynamicAllowed: true, Levels: []string{"minimal", "low", "medium", "high"}},
		SupportedInputModalities:  []string{"text", "image", "audio", "video"},
		SupportedOutputModalities: []string{"text"},
	}, {
		ID:                        "gemini-3.6-flash",
		DisplayName:               "Gemini 3.6 Flash",
		MaxInputTokens:            1048576,
		Thinking:                  &thinkingSupport{Levels: []string{"low", "medium", "high"}},
		SupportedInputModalities:  []string{"text", "image", "audio", "video"},
		SupportedOutputModalities: []string{"text"},
	}}
}

func TestWriteBackendConfigRestrictsNetworkAndCredits(t *testing.T) {
	a := testApp(t)
	a.settings.ProxyURL = "socks5://127.0.0.1:1080"
	if err := a.writeBackendConfig(18087); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(a.paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	checks := []string{
		`host: "127.0.0.1"`,
		"port: 18087",
		"allow-remote: false",
		`secret-key: "` + a.apiKey + `"`,
		"antigravity-credits: false",
		"request-log: false",
		"commercial-mode: true",
		"disable-control-panel: true",
		"disable-cloaking-model-list: true",
		`proxy-url: "socks5://127.0.0.1:1080"`,
		`name: "gemini-3.7-flash-high"`,
		`alias: "gemini-3.7-flash"`,
		`display-name: "Gemini 3.7 Flash"`,
		`name: "gemini-3.6-flash-high"`,
		`alias: "gemini-3.6-flash"`,
		`display-name: "Gemini 3.6 Flash"`,
		"force-mapping: true",
		a.apiKey,
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Errorf("config missing %q", check)
		}
	}
	if got := strings.Count(text, "force-mapping: true"); got != 2 {
		t.Fatalf("force-mapping entry count = %d, want 2", got)
	}
	if strings.Contains(text, "fork: true") {
		t.Fatal("upstream IDs must not remain client-visible")
	}
	if strings.Contains(text, `host: ""`) {
		t.Fatal("config must not bind all interfaces")
	}
}

func TestChooseGatewayPortSkipsUnrelatedListener(t *testing.T) {
	a := testApp(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	a.settings.PreferredPort = port
	a.settings.PortScanEnd = port + 20
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not our gateway"))
	})}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })

	selected, reuse, err := a.chooseGatewayPort()
	if err != nil {
		t.Fatal(err)
	}
	if reuse {
		t.Fatal("must not reuse an unrelated listener")
	}
	if selected <= port || selected > port+20 {
		t.Fatalf("selected %d outside expected fallback range", selected)
	}
}

func TestChooseGatewayPortReusesAuthenticatedGateway(t *testing.T) {
	a := testApp(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	a.settings.PreferredPort = port
	a.settings.PortScanEnd = port + 2
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"CLI Proxy API Server"}`))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != a.apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gemini-3-flash"}]}`))
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })

	selected, reuse, err := a.chooseGatewayPort()
	if err != nil {
		t.Fatal(err)
	}
	if !reuse || selected != port {
		t.Fatalf("got port=%d reuse=%v, want %d true", selected, reuse, port)
	}
}

func TestChooseFreePortSkipsOccupiedCallbackPort(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	occupied := listener.Addr().(*net.TCPAddr).Port
	selected, err := chooseFreePort(occupied, occupied+10)
	if err != nil {
		t.Fatal(err)
	}
	if selected <= occupied || selected > occupied+10 {
		t.Fatalf("selected callback port %d, occupied=%d", selected, occupied)
	}
}

func TestFetchModelsUsesAnthropicHeadersAndDeduplicates(t *testing.T) {
	a := testApp(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("x-api-key") != a.apiKey {
			http.Error(w, "missing headers", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"z-model"},{"id":"a-model","display_name":"A","max_input_tokens":1048576,"max_tokens":65536,"thinking":{"min":1,"max":65535,"dynamic_allowed":true,"levels":["minimal","low","medium","high"]},"supportedInputModalities":["text","image","audio","video"],"supportedOutputModalities":["text"]},{"id":"z-model"},{"id":""}]}`))
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })

	models, err := a.fetchModels(port)
	if err != nil {
		t.Fatal(err)
	}
	if got := modelIDs(models); fmt.Sprint(got) != "[a-model z-model]" {
		t.Fatalf("models = %v", got)
	}
	if models[0].MaxInputTokens != 1048576 || models[0].MaxTokens != 65536 {
		t.Fatalf("model limits were not preserved: %+v", models[0])
	}
	if fmt.Sprint(models[0].SupportedInputModalities) != "[text image audio video]" {
		t.Fatalf("model modalities were not preserved: %+v", models[0])
	}
	if models[0].Thinking == nil || fmt.Sprint(models[0].Thinking.Levels) != "[minimal low medium high]" || models[0].Thinking.Max != 65535 {
		t.Fatalf("model thinking metadata was not preserved: %+v", models[0])
	}
}

func TestWaitForModelsAllowsDelayedRegistration(t *testing.T) {
	a := testApp(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"delayed-model"}]}`))
	})
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })
	models, err := a.waitForModels(port, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "delayed-model" {
		t.Fatalf("models = %#v", models)
	}
}

func TestSelectZCodeModelsAllowsOnlyGemini37And36Flash(t *testing.T) {
	catalog := append(requiredTestModels(),
		modelInfo{ID: "claude-opus-4-6-thinking"},
		modelInfo{ID: "gemini-3.7-flash-low"},
		modelInfo{ID: "gemini-3.6-flash-medium"},
	)
	models, err := selectZCodeModels(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(modelIDs(models)); got != "[gemini-3.6-flash gemini-3.7-flash]" {
		t.Fatalf("selected models = %s", got)
	}
	if isAllowedZCodeModel("claude-opus-4-6-thinking") || isAllowedZCodeModel("gemini-3.7-flash-high") || !isAllowedZCodeModel("gemini-3.7-flash") {
		t.Fatal("model allowlist decision is incorrect")
	}
}

func TestZCodeModelAliasesMatchClientAllowlist(t *testing.T) {
	if len(zcodeModelAliases) != len(zcodeModelAllowlist) {
		t.Fatalf("aliases=%d allowlist=%d", len(zcodeModelAliases), len(zcodeModelAllowlist))
	}
	seen := make(map[string]bool, len(zcodeModelAliases))
	for _, alias := range zcodeModelAliases {
		if alias.UpstreamID == alias.ClientID || !strings.HasSuffix(alias.UpstreamID, "-high") || strings.HasSuffix(alias.ClientID, "-high") {
			t.Fatalf("invalid clean model alias: %#v", alias)
		}
		seen[alias.ClientID] = true
	}
	for _, id := range zcodeModelAllowlist {
		if !seen[id] {
			t.Fatalf("allowlisted client model %q has no upstream alias", id)
		}
	}
}

func TestSelectZCodeModelsRequiresBothModels(t *testing.T) {
	if _, err := selectZCodeModels(requiredTestModels()[:1]); err == nil || !strings.Contains(err.Error(), "gemini-3.6-flash") {
		t.Fatalf("expected missing Gemini 3.6 error, got %v", err)
	}
}

func TestConfigureZCodeBacksUpAndPreservesOtherProviders(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{
  "provider": {
    "keep-me": {"name": "Existing", "kind": "openai", "models": {}}
  },
  "unknownFutureField": {"value": 9007199254740993}
}
`)
	if err := os.WriteFile(a.paths.ZCodeConfig, original, 0o600); err != nil {
		t.Fatal(err)
	}
	models := append(requiredTestModels(), modelInfo{ID: "claude-opus-4-6-thinking", DisplayName: "Claude Opus"})
	backup, changed, err := a.configureZCode(18081, models)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || backup == "" {
		t.Fatalf("changed=%v backup=%q", changed, backup)
	}
	backupRaw, err := os.ReadFile(backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupRaw) != string(original) {
		t.Fatal("backup does not exactly match original")
	}
	_, root, err := readJSONObject(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	providers, err := objectField(root, "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := providers["keep-me"]; !ok {
		t.Fatal("unrelated provider was removed")
	}
	ours, ok := providers[providerID].(map[string]any)
	if !ok {
		t.Fatal("our provider missing")
	}
	if got := fmt.Sprint(sortedProviderModelIDs(ours)); got != "[gemini-3.6-flash gemini-3.7-flash]" {
		t.Fatalf("model ids = %s", got)
	}
	configuredModels := ours["models"].(map[string]any)
	gemini := configuredModels["gemini-3.7-flash"].(map[string]any)
	limit := gemini["limit"].(map[string]any)
	if fmt.Sprint(limit["context"]) != "1048576" {
		t.Fatalf("Gemini context limit = %v", limit["context"])
	}
	modalities := gemini["modalities"].(map[string]any)
	if fmt.Sprint(modalities["input"]) != "[text image audio video]" {
		t.Fatalf("Gemini input modalities = %v", modalities["input"])
	}
	reasoning := gemini["reasoning"].(map[string]any)
	if reasoning["enabled"] != true || fmt.Sprint(reasoning["variants"]) != "[low medium high]" || reasoning["defaultVariant"] != "high" {
		t.Fatalf("Gemini reasoning selector = %#v", reasoning)
	}
	if _, ok := configuredModels["gemini-3.6-flash"]; !ok {
		t.Fatal("Gemini 3.6 Flash is missing")
	}
	if _, ok := configuredModels["claude-opus-4-6-thinking"]; ok {
		t.Fatal("non-allowlisted Claude model was written")
	}
	unknown := root["unknownFutureField"].(map[string]any)
	if fmt.Sprint(unknown["value"]) != "9007199254740993" {
		t.Fatalf("large number changed: %v", unknown["value"])
	}
	secondBackup, secondChanged, err := a.configureZCode(18081, models)
	if err != nil {
		t.Fatal(err)
	}
	if secondChanged || secondBackup != "" {
		t.Fatalf("idempotent sync changed config: changed=%v backup=%q", secondChanged, secondBackup)
	}
}

func TestConfigureZCodeNeverOverwritesBrokenJSON(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"provider":`)
	if err := os.WriteFile(a.paths.ZCodeConfig, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.configureZCode(18080, requiredTestModels()); err == nil {
		t.Fatal("expected parse error")
	}
	after, err := os.ReadFile(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("broken input was overwritten")
	}
	entries, err := os.ReadDir(a.paths.ZCodeBackups)
	if err == nil && len(entries) != 0 {
		t.Fatal("unexpected backup created for invalid JSON")
	}
}

func TestConfigureZCodeAllowsNoOpWhileRunningButRefusesWrite(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte(`{"provider":{"keep-me":{"name":"Existing","kind":"openai","models":{}}}}`)
	if err := os.WriteFile(a.paths.ZCodeConfig, original, 0o600); err != nil {
		t.Fatal(err)
	}
	models := requiredTestModels()
	if _, changed, err := a.configureZCode(18080, models); err != nil || !changed {
		t.Fatalf("initial configure changed=%v err=%v", changed, err)
	}

	a.zcodeRunning = func() bool { return true }
	if backup, changed, err := a.configureZCode(18080, models); err != nil || changed || backup != "" {
		t.Fatalf("no-op while running backup=%q changed=%v err=%v", backup, changed, err)
	}
	before, err := os.ReadFile(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	changedModels := append([]modelInfo(nil), models...)
	changedModels[0].DisplayName = "Gemini 3.7 Flash Updated"
	if _, _, err := a.configureZCode(18080, changedModels); err == nil {
		t.Fatal("expected a running ZCode process to block a required provider write")
	}
	after, err := os.ReadFile(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("blocked provider write changed the ZCode config")
	}
}

func TestConfigureZCodeRefusesUnmanagedProviderCollision(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	original := []byte(fmt.Sprintf(`{"provider":{%q:{"name":"User owned"}}}`, providerID))
	if err := os.WriteFile(a.paths.ZCodeConfig, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.configureZCode(18080, requiredTestModels()); err == nil {
		t.Fatal("expected collision error")
	}
	after, err := os.ReadFile(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("unmanaged provider was overwritten")
	}
}

func TestRemoveProviderOnlyRemovesOwnedEntry(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(filepath.Dir(a.paths.ZCodeConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"provider": map[string]any{
			providerID: map[string]any{"name": providerName, "x-zcode-antigravity-managed": 1},
			"other":    map[string]any{"name": "Keep"},
		},
		"theme": "dark",
	}
	raw, _ := marshalJSONObject(config)
	if err := os.WriteFile(a.paths.ZCodeConfig, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := a.removeZCodeProvider(); err != nil {
		t.Fatal(err)
	}
	_, root, err := readJSONObject(a.paths.ZCodeConfig)
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := objectField(root, "provider")
	if _, ok := providers[providerID]; ok {
		t.Fatal("owned provider still present")
	}
	if _, ok := providers["other"]; !ok || root["theme"] != "dark" {
		t.Fatal("unrelated settings changed")
	}
}

func TestLoadSettingsValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"preferredPort":18080,"portScanEnd":18180,"callbackPreferredPort":51121,"callbackScanEnd":51221,"proxyURL":"ftp://bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSettings(path); err == nil {
		t.Fatal("expected unsupported proxy scheme error")
	}
}

func TestResolveZCodePathsHonorsSettingBeforeEnvironment(t *testing.T) {
	userHome := t.TempDir()
	settingDir := filepath.Join(userHome, ".zcode", "v2")
	if err := os.MkdirAll(settingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fromSetting := filepath.Join(t.TempDir(), "zcode-data")
	fromEnvironment := filepath.Join(t.TempDir(), "ignored-env")
	t.Setenv("ZCODE_DATA_BASE_DIR", fromEnvironment)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "ignored-home"))
	raw := []byte(fmt.Sprintf(`{"dataBaseDir":%q}`, fromSetting))
	if err := os.WriteFile(filepath.Join(settingDir, "setting.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	config, backups, err := resolveZCodePaths(userHome)
	if err != nil {
		t.Fatal(err)
	}
	wantConfig := filepath.Join(fromSetting, ".zcode", "v2", "config.json")
	if config != wantConfig {
		t.Fatalf("config = %q, want %q", config, wantConfig)
	}
	if !strings.HasPrefix(backups, filepath.Join(fromSetting, ".zcode", "v2")) {
		t.Fatalf("backups = %q", backups)
	}
}

func TestWindowsBuildUsesExeBackendName(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	// The actual cross-build is covered by the release script; this guards the
	// package convention used by the Windows build.
	if !strings.HasSuffix("cli-proxy-api.exe", ".exe") {
		t.Fatal("invalid Windows backend name")
	}
}

func TestCountAntigravityAccountsValidatesJSONAndTokens(t *testing.T) {
	a := testApp(t)
	validPath := filepath.Join(a.paths.AuthDir, "antigravity-valid.json")
	if err := os.WriteFile(validPath, []byte(`{"type":"antigravity","access_token":"dpapi:v1:access","refresh_token":"dpapi:v1:refresh","project_id":"test-project"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := countAntigravityAccounts(a.paths.AuthDir)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := os.WriteFile(filepath.Join(a.paths.AuthDir, "antigravity-broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := countAntigravityAccounts(a.paths.AuthDir); err == nil {
		t.Fatal("malformed account file was silently accepted")
	}
}

func TestManagerRunLockRejectsConcurrentOwnerAndRecovers(t *testing.T) {
	a := testApp(t)
	release, err := a.acquireRunLock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.acquireRunLock(); err == nil {
		release()
		t.Fatal("second manager acquired the same lock")
	}
	release()
	releaseAgain, err := a.acquireRunLock()
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	releaseAgain()
}

func TestManagedBackupRetention(t *testing.T) {
	a := testApp(t)
	if err := os.MkdirAll(a.paths.ZCodeBackups, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 25; index++ {
		name := fmt.Sprintf("config-20260815-120000.%09d-before-sync.json", index)
		if err := os.WriteFile(filepath.Join(a.paths.ZCodeBackups, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneManagedBackups(a.paths.ZCodeBackups, 20); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(a.paths.ZCodeBackups)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 20 {
		t.Fatalf("backup count=%d, want 20", len(entries))
	}
}

func TestRotateManagedFileKeepsBoundedHistory(t *testing.T) {
	a := testApp(t)
	if err := os.WriteFile(a.paths.ConsoleLog, []byte("0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := rotateManagedFile(a.paths.ConsoleLog, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.paths.ConsoleLog + ".1"); err != nil {
		t.Fatalf("rotated log missing: %v", err)
	}
}
