package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const connectorProviderName = providerName

type connectorInstallContext struct {
	BaseURL       string
	OpenAIBaseURL string
	APIKey        string
	Model         string
	Provider      string
	ContextWindow int
}

func connectorDisplayName(id string) string {
	names := map[string]string{
		"deepseek-harness": "DeepSeek Harness", "grok-build": "Grok Build", "codex": "OpenAI Codex",
		"claude-code": "Claude Code", "gemini-cli": "Gemini CLI", "qwen-code": "Qwen Code",
		"kimi-code": "Kimi Code", "opencode": "OpenCode",
	}
	return names[id]
}

func (g *guiRuntime) installAgentConnector(id string) error {
	ctx, err := g.connectorInstallContext()
	if err != nil {
		return err
	}
	switch id {
	case "deepseek-harness":
		_, err = g.app.configureDeepSeekHarness(ctx.BaseURL, ctx.Model, ctx.Provider == "antigravity")
	case "grok-build":
		err = g.app.configureGrokBuild(ctx)
	case "codex":
		err = g.app.configureCodex(ctx)
	case "claude-code":
		err = g.app.configureClaudeCode(ctx)
	case "gemini-cli":
		if ctx.Provider != "antigravity" {
			return errors.New("Gemini CLI 原生协议当前仅用于 Antigravity；Grok 请使用 Codex、Claude Code、Qwen Code、Kimi Code 或 OpenCode")
		}
		err = g.app.configureGeminiCLI(ctx)
	case "qwen-code":
		err = g.app.configureQwenCode(ctx)
	case "kimi-code":
		err = g.app.configureKimiCode(ctx)
	case "opencode":
		err = g.app.configureOpenCode(ctx)
	default:
		return errors.New("不支持的一键 Agent 接入")
	}
	if err == nil {
		fmt.Printf("%s 已接入本机网关（model=%s）\n", connectorDisplayName(id), ctx.Model)
	}
	return err
}

func (g *guiRuntime) connectorInstallContext() (connectorInstallContext, error) {
	var ctx connectorInstallContext
	current, err := g.app.loadState()
	if err != nil {
		return ctx, err
	}
	if current.Port <= 0 || g.app.probeGateway(current.Port) != nil {
		return ctx, errors.New("请先启动本地网关，再接入 Agent / CLI")
	}
	counts, err := countProviderAccounts(g.app.paths.AuthDir)
	if err != nil {
		return ctx, err
	}
	ctx.Provider = g.currentProvider()
	catalog, err := g.app.fetchModels(current.Port)
	if err != nil {
		return ctx, err
	}
	models, err := selectAgentModels(catalog, ctx.Provider == "antigravity" && counts.Antigravity > 0, ctx.Provider == "xai" && counts.XAI > 0)
	if err != nil {
		return ctx, err
	}
	ctx.Model = preferredConnectorModel(ctx.Provider, models, g.app.currentSettings().BackgroundModel)
	ctx.ContextWindow = 131072
	for _, model := range models {
		if model.ID == ctx.Model && model.MaxInputTokens > 0 {
			ctx.ContextWindow = model.MaxInputTokens
			break
		}
	}
	ctx.BaseURL = g.app.gatewayURL(current.Port)
	ctx.OpenAIBaseURL = strings.TrimRight(ctx.BaseURL, "/") + "/v1"
	ctx.APIKey = g.app.apiKey
	return ctx, nil
}

func (a *app) configureClaudeCode(ctx connectorInstallContext) error {
	dir, err := connectorConfigDir("CLAUDE_CONFIG_DIR", ".claude")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.json")
	root, err := readJSONMapping(path)
	if err != nil {
		return err
	}
	env, err := ensureMap(root, "env")
	if err != nil {
		return fmt.Errorf("Claude Code settings.json 的 env 不是对象，不会覆盖")
	}
	env["ANTHROPIC_BASE_URL"] = ctx.BaseURL
	env["ANTHROPIC_AUTH_TOKEN"] = ctx.APIKey
	env["ANTHROPIC_MODEL"] = ctx.Model
	return a.writeJSONConnector("claude-code", path, root)
}

func (a *app) configureGeminiCLI(ctx connectorInstallContext) error {
	dir, err := geminiConfigDir()
	if err != nil {
		return err
	}
	settingsPath, envPath := filepath.Join(dir, "settings.json"), filepath.Join(dir, ".env")
	root, err := readJSONMapping(settingsPath)
	if err != nil {
		return err
	}
	security, err := ensureMap(root, "security")
	if err != nil {
		return errors.New("Gemini CLI settings.json 的 security 不是对象，不会覆盖")
	}
	auth, err := ensureMap(security, "auth")
	if err != nil {
		return errors.New("Gemini CLI settings.json 的 security.auth 不是对象，不会覆盖")
	}
	auth["selectedType"] = "gemini-api-key"
	settingsNext, err := marshalJSON(root)
	if err != nil {
		return err
	}
	envRaw, _, err := readOptionalFile(envPath)
	if err != nil {
		return err
	}
	envNext := mergeDotEnv(envRaw, map[string]string{
		"GEMINI_API_KEY": ctx.APIKey, "GOOGLE_GEMINI_BASE_URL": ctx.BaseURL, "GEMINI_MODEL": ctx.Model,
	})
	rollbackEnv, err := a.updateConnectorFile("gemini-cli", envPath, envNext)
	if err != nil {
		return err
	}
	if _, err = a.updateConnectorFile("gemini-cli", settingsPath, settingsNext); err != nil {
		_ = rollbackEnv()
	}
	return err
}

func (a *app) configureQwenCode(ctx connectorInstallContext) error {
	dir, err := connectorConfigDir("QWEN_CODE_HOME", ".qwen")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "settings.json")
	root, err := readJSONMapping(path)
	if err != nil {
		return err
	}
	env, err := ensureMap(root, "env")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 env 不是对象，不会覆盖")
	}
	env["ZCODE_BRIDGE_API_KEY"] = ctx.APIKey
	providers, err := ensureMap(root, "modelProviders")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 modelProviders 不是对象，不会覆盖")
	}
	if existing, exists := providers["zcode-bridge"]; exists {
		entries, ok := existing.([]any)
		if !ok || len(entries) == 0 {
			return errors.New("Qwen Code 已有非本程序格式的 zcode-bridge，不会覆盖")
		}
		entry, ok := entries[0].(map[string]any)
		baseURL, _ := entry["baseUrl"].(string)
		if !ok || !isLoopbackDeepSeekHarnessEndpoint(baseURL) {
			return errors.New("Qwen Code 已存在非本程序创建的 zcode-bridge，不会覆盖")
		}
	}
	providers["zcode-bridge"] = []any{map[string]any{
		"id": ctx.Model, "name": ctx.Model, "envKey": "ZCODE_BRIDGE_API_KEY", "baseUrl": ctx.OpenAIBaseURL,
		"generationConfig": map[string]any{"timeout": 120000, "contextWindowSize": ctx.ContextWindow},
	}}
	protocols, err := ensureMap(root, "providerProtocol")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 providerProtocol 不是对象，不会覆盖")
	}
	protocols["zcode-bridge"] = "openai"
	security, err := ensureMap(root, "security")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 security 不是对象，不会覆盖")
	}
	auth, err := ensureMap(security, "auth")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 security.auth 不是对象，不会覆盖")
	}
	auth["selectedType"] = "zcode-bridge"
	model, err := ensureMap(root, "model")
	if err != nil {
		return errors.New("Qwen Code settings.json 的 model 不是对象，不会覆盖")
	}
	model["name"] = ctx.Model
	return a.writeJSONConnector("qwen-code", path, root)
}

func (a *app) configureOpenCode(ctx connectorInstallContext) error {
	path, err := openCodeConfigPath()
	if err != nil {
		return err
	}
	root, err := readJSONMapping(path)
	if err != nil {
		return err
	}
	providers, err := ensureMap(root, "provider")
	if err != nil {
		return errors.New("OpenCode 配置的 provider 不是对象，不会覆盖")
	}
	if existing, ok := providers["zcode-bridge"].(map[string]any); ok {
		options, _ := existing["options"].(map[string]any)
		baseURL, _ := options["baseURL"].(string)
		name, _ := existing["name"].(string)
		if name != connectorProviderName && !isLoopbackDeepSeekHarnessEndpoint(baseURL) {
			return errors.New("OpenCode 已存在非本程序创建的 zcode-bridge，不会覆盖")
		}
	} else if _, exists := providers["zcode-bridge"]; exists {
		return errors.New("OpenCode 已有 zcode-bridge，但它不是对象，不会覆盖")
	}
	keyPath := filepath.Join(filepath.Dir(path), "zcode-bridge-key")
	providers["zcode-bridge"] = map[string]any{
		"npm": "@ai-sdk/openai-compatible", "name": connectorProviderName,
		"options": map[string]any{"baseURL": ctx.OpenAIBaseURL, "apiKey": "{file:" + filepath.ToSlash(keyPath) + "}"},
		"models":  map[string]any{ctx.Model: map[string]any{"name": ctx.Model}},
	}
	root["model"] = "zcode-bridge/" + ctx.Model
	root["small_model"] = "zcode-bridge/" + ctx.Model
	configNext, err := marshalJSON(root)
	if err != nil {
		return err
	}
	rollbackKey, err := a.updateConnectorFile("opencode", keyPath, []byte(ctx.APIKey+"\n"))
	if err != nil {
		return err
	}
	if _, err = a.updateConnectorFile("opencode", path, configNext); err != nil {
		_ = rollbackKey()
	}
	return err
}

func (a *app) configureCodex(ctx connectorInstallContext) error {
	dir, err := connectorConfigDir("CODEX_HOME", ".codex")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.toml")
	root, err := readTOMLMapping(path)
	if err != nil {
		return err
	}
	providers, err := ensureMap(root, "model_providers")
	if err != nil {
		return errors.New("Codex config.toml 的 model_providers 不是对象，不会覆盖")
	}
	if err := guardManagedProvider(providers, "zcode_bridge", "base_url", "name", "Codex"); err != nil {
		return err
	}
	providers["zcode_bridge"] = map[string]any{
		"name": connectorProviderName, "base_url": ctx.OpenAIBaseURL, "wire_api": "responses",
		"experimental_bearer_token": ctx.APIKey,
	}
	root["model"] = ctx.Model
	root["model_provider"] = "zcode_bridge"
	return a.writeTOMLConnector("codex", path, root)
}

func (a *app) configureKimiCode(ctx connectorInstallContext) error {
	dir, err := connectorConfigDir("KIMI_HOME", ".kimi")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.toml")
	root, err := readTOMLMapping(path)
	if err != nil {
		return err
	}
	providers, err := ensureMap(root, "providers")
	if err != nil {
		return errors.New("Kimi Code config.toml 的 providers 不是对象，不会覆盖")
	}
	if err := guardManagedProvider(providers, "zcode-bridge", "base_url", "", "Kimi Code"); err != nil {
		return err
	}
	providers["zcode-bridge"] = map[string]any{"type": "openai", "base_url": ctx.OpenAIBaseURL, "api_key": ctx.APIKey}
	models, err := ensureMap(root, "models")
	if err != nil {
		return errors.New("Kimi Code config.toml 的 models 不是对象，不会覆盖")
	}
	model := map[string]any{"provider": "zcode-bridge", "model": ctx.Model, "max_context_size": ctx.ContextWindow, "display_name": ctx.Model}
	if ctx.Provider == "antigravity" {
		model["capabilities"] = []string{"thinking", "image_in"}
	}
	models["zcode-bridge"] = model
	root["default_model"] = "zcode-bridge"
	return a.writeTOMLConnector("kimi-code", path, root)
}

func (a *app) configureGrokBuild(ctx connectorInstallContext) error {
	dir, err := connectorConfigDir("GROK_HOME", ".grok")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.toml")
	root, err := readTOMLMapping(path)
	if err != nil {
		return err
	}
	endpoints, err := ensureMap(root, "endpoints")
	if err != nil {
		return errors.New("Grok Build config.toml 的 endpoints 不是对象，不会覆盖")
	}
	endpoints["models_base_url"] = ctx.OpenAIBaseURL
	models, err := ensureMap(root, "models")
	if err != nil {
		return errors.New("Grok Build config.toml 的 models 不是对象，不会覆盖")
	}
	models["default"] = "zcode-bridge"
	modelConfigs, err := ensureMap(root, "model")
	if err != nil {
		return errors.New("Grok Build config.toml 的 model 不是对象，不会覆盖")
	}
	if err := guardManagedProvider(modelConfigs, "zcode-bridge", "base_url", "name", "Grok Build"); err != nil {
		return err
	}
	modelConfigs["zcode-bridge"] = map[string]any{
		"model": ctx.Model, "name": connectorProviderName, "base_url": ctx.OpenAIBaseURL, "api_key": ctx.APIKey,
	}
	return a.writeTOMLConnector("grok-build", path, root)
}

func connectorConfigDir(envKey string, defaultName string) (string, error) {
	dir := strings.TrimSpace(os.Getenv(envKey))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, defaultName)
	}
	return safeConnectorPath(dir)
}

func geminiConfigDir() (string, error) {
	home := strings.TrimSpace(os.Getenv("GEMINI_CLI_HOME"))
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	return safeConnectorPath(filepath.Join(home, ".gemini"))
}

func openCodeConfigPath() (string, error) {
	if value := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG")); value != "" {
		return safeConnectorPath(value)
	}
	base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return safeConnectorPath(filepath.Join(base, "opencode", "opencode.json"))
}

func safeConnectorPath(path string) (string, error) {
	abs, err := filepath.Abs(os.ExpandEnv(path))
	if err != nil {
		return "", err
	}
	if filepath.Clean(abs) == filepath.VolumeName(abs)+string(os.PathSeparator) {
		return "", errors.New("Agent 配置路径不能是磁盘根目录")
	}
	return abs, nil
}

func readJSONMapping(path string) (map[string]any, error) {
	raw, _, err := readOptionalFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, err)
	}
	root := map[string]any{}
	if len(bytes.TrimSpace(raw)) != 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("%s 不是有效 JSON，不会覆盖: %w", path, err)
		}
	}
	return root, nil
}

func readTOMLMapping(path string) (map[string]any, error) {
	raw, _, err := readOptionalFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", path, err)
	}
	root := map[string]any{}
	if len(bytes.TrimSpace(raw)) != 0 {
		if err := toml.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("%s 不是有效 TOML，不会覆盖: %w", path, err)
		}
	}
	return root, nil
}

func ensureMap(parent map[string]any, key string) (map[string]any, error) {
	if value, ok := parent[key]; ok {
		mapping, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("not a mapping")
		}
		return mapping, nil
	}
	mapping := map[string]any{}
	parent[key] = mapping
	return mapping, nil
}

func guardManagedProvider(parent map[string]any, id, baseURLKey, nameKey, tool string) error {
	value, exists := parent[id]
	if !exists {
		return nil
	}
	provider, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s 已有非本程序格式的 %s，不会覆盖", tool, id)
	}
	baseURL, _ := provider[baseURLKey].(string)
	name, _ := provider[nameKey].(string)
	if !isLoopbackDeepSeekHarnessEndpoint(baseURL) && (nameKey == "" || name != connectorProviderName) {
		return fmt.Errorf("%s 已存在非本程序创建的 %s，不会覆盖", tool, id)
	}
	return nil
}

func marshalJSON(root map[string]any) ([]byte, error) {
	raw, err := json.MarshalIndent(root, "", "  ")
	return append(raw, '\n'), err
}

func (a *app) writeJSONConnector(tool, path string, root map[string]any) error {
	raw, err := marshalJSON(root)
	if err != nil {
		return err
	}
	_, err = a.updateConnectorFile(tool, path, raw)
	return err
}

func (a *app) writeTOMLConnector(tool, path string, root map[string]any) error {
	raw, err := toml.Marshal(root)
	if err != nil {
		return err
	}
	_, err = a.updateConnectorFile(tool, path, raw)
	return err
}

func (a *app) updateConnectorFile(tool, path string, next []byte) (func() error, error) {
	previous, existed, err := readOptionalFile(path)
	if err != nil {
		return nil, err
	}
	rollback := func() error { return restoreOptionalFile(path, previous, existed) }
	if bytes.Equal(previous, next) {
		return rollback, nil
	}
	if existed {
		backupDir := filepath.Join(filepath.Dir(path), "backups", "zcode-antigravity")
		name := a.now().UTC().Format("20060102-150405.000000000") + "-" + tool + "-" + filepath.Base(path)
		if err := writeAtomic(filepath.Join(backupDir, name), previous, 0o600); err != nil {
			return nil, fmt.Errorf("备份 %s: %w", path, err)
		}
	}
	if err := writeAtomic(path, next, 0o600); err != nil {
		return nil, fmt.Errorf("写入 %s: %w", path, err)
	}
	return rollback, nil
}

func mergeDotEnv(raw []byte, values map[string]string) []byte {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines)+len(values))
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		key, _, found := strings.Cut(trimmed, "=")
		if found {
			if _, managed := values[strings.TrimSpace(key)]; managed {
				continue
			}
		}
		if line != "" {
			kept = append(kept, line)
		}
	}
	for _, key := range []string{"GEMINI_API_KEY", "GOOGLE_GEMINI_BASE_URL", "GEMINI_MODEL"} {
		if value, ok := values[key]; ok {
			kept = append(kept, key+"="+value)
		}
	}
	return []byte(strings.Join(kept, "\n") + "\n")
}
