package main

import (
	"fmt"
	"net/http"
	"strings"
)

type agentConnector struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Model       string            `json:"model"`
	Action      string            `json:"action,omitempty"`
	Snippets    map[string]string `json:"snippets"`
}

type connectorResponse struct {
	Provider   string           `json:"provider"`
	BaseURL    string           `json:"baseURL"`
	Model      string           `json:"model"`
	Connectors []agentConnector `json:"connectors"`
}

func (g *guiRuntime) serveConnectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, errState := g.app.loadState()
	if errState != nil || current.Port <= 0 || g.app.probeGateway(current.Port) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "请先启动本地网关，再生成 Agent 接入配置"})
		return
	}
	counts, errCounts := countProviderAccounts(g.app.paths.AuthDir)
	if errCounts != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errCounts.Error()})
		return
	}
	provider := g.currentProvider()
	catalog, errModels := g.app.fetchModels(current.Port)
	if errModels != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errModels.Error()})
		return
	}
	models, errSelect := selectAgentModels(catalog, provider == "antigravity" && counts.Antigravity > 0, provider == "xai" && counts.XAI > 0)
	if errSelect != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errSelect.Error()})
		return
	}
	model := preferredConnectorModel(provider, models, g.app.currentSettings().BackgroundModel)
	baseURL := g.app.gatewayURL(current.Port)
	writeJSON(w, http.StatusOK, connectorResponse{
		Provider:   provider,
		BaseURL:    baseURL,
		Model:      model,
		Connectors: buildAgentConnectors(baseURL, g.app.apiKey, model, provider),
	})
}

func preferredConnectorModel(provider string, models []modelInfo, backgroundModel string) string {
	preferred := []string{}
	if provider != "xai" && strings.TrimSpace(backgroundModel) != "" {
		preferred = append(preferred, strings.TrimSpace(backgroundModel))
	}
	preferred = append(preferred, "gemini-3.7-flash", "gemini-3.6-flash")
	if provider == "xai" {
		preferred = []string{"grok-build-0.1", "grok-4.6", "grok-4.5", "grok-code-fast-1"}
	}
	for _, candidate := range preferred {
		for _, model := range models {
			if model.ID == candidate {
				return candidate
			}
		}
	}
	ids := modelIDs(models)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

func buildAgentConnectors(baseURL, apiKey, model, provider string) []agentConnector {
	openAIBase := strings.TrimRight(baseURL, "/") + "/v1"
	geminiAction := ""
	geminiDescription := "使用 Gemini 原生协议连接当前 Antigravity 模型"
	if provider == "antigravity" {
		geminiAction = connectorAction("gemini-cli")
	} else {
		geminiDescription = "Gemini 原生协议仅支持 Antigravity；Grok 请使用其他 OpenAI / Anthropic 客户端"
	}
	connectors := []agentConnector{
		{
			ID: "deepseek-harness", Name: "DeepSeek Harness", Description: "一键写入当前用户的 DSH Provider、凭据和默认模型；修改前自动备份",
			Model: model, Action: connectorAction("deepseek-harness"),
			Snippets: deepSeekHarnessSnippets(openAIBase, apiKey, model, provider == "antigravity"),
		},
		{
			ID: "grok-build", Name: "Grok Build", Description: "让官方 Grok 终端 Agent 使用本地桥接模型",
			Model: model, Action: connectorAction("grok-build"),
			Snippets: map[string]string{
				"macOS / Linux":      fmt.Sprintf("export GROK_MODELS_BASE_URL=%q\nexport GROK_MODELS_LIST_URL=%q\nexport XAI_API_KEY=%q\ngrok", openAIBase, openAIBase+"/models", apiKey),
				"Windows PowerShell": fmt.Sprintf("$env:GROK_MODELS_BASE_URL = %q\n$env:GROK_MODELS_LIST_URL = %q\n$env:XAI_API_KEY = %q\ngrok", openAIBase, openAIBase+"/models", apiKey),
			},
		},
		{
			ID: "codex", Name: "OpenAI Codex", Description: "添加一个 Responses API 自定义模型提供商",
			Model: model, Action: connectorAction("codex"),
			Snippets: map[string]string{
				"~/.codex/config.toml": fmt.Sprintf("model = %q\nmodel_provider = \"zcode_bridge\"\n\n[model_providers.zcode_bridge]\nname = %q\nbase_url = %q\nwire_api = \"responses\"\nexperimental_bearer_token = %q", model, connectorProviderName, openAIBase, apiKey),
			},
		},
		{
			ID: "claude-code", Name: "Claude Code", Description: "通过 Anthropic 兼容接口连接本地网关",
			Model: model, Action: connectorAction("claude-code"),
			Snippets: map[string]string{
				"macOS / Linux":      fmt.Sprintf("export ANTHROPIC_BASE_URL=%q\nexport ANTHROPIC_AUTH_TOKEN=%q\nexport ANTHROPIC_MODEL=%q\nclaude", baseURL, apiKey, model),
				"Windows PowerShell": fmt.Sprintf("$env:ANTHROPIC_BASE_URL = %q\n$env:ANTHROPIC_AUTH_TOKEN = %q\n$env:ANTHROPIC_MODEL = %q\nclaude", baseURL, apiKey, model),
			},
		},
		{
			ID: "gemini-cli", Name: "Gemini CLI", Description: geminiDescription,
			Model: model, Action: geminiAction,
			Snippets: map[string]string{
				"~/.gemini/settings.json": "{\n  \"security\": { \"auth\": { \"selectedType\": \"gemini-api-key\" } }\n}",
				"~/.gemini/.env":          fmt.Sprintf("GEMINI_API_KEY=%s\nGOOGLE_GEMINI_BASE_URL=%s\nGEMINI_MODEL=%s", apiKey, baseURL, model),
			},
		},
		{
			ID: "qwen-code", Name: "Qwen Code", Description: "添加 OpenAI-compatible Provider 并选中当前模型",
			Model: model, Action: connectorAction("qwen-code"),
			Snippets: map[string]string{
				"~/.qwen/settings.json": fmt.Sprintf("{\n  \"env\": { \"ZCODE_BRIDGE_API_KEY\": %q },\n  \"modelProviders\": { \"zcode-bridge\": [{ \"id\": %q, \"envKey\": \"ZCODE_BRIDGE_API_KEY\", \"baseUrl\": %q }] },\n  \"providerProtocol\": { \"zcode-bridge\": \"openai\" },\n  \"security\": { \"auth\": { \"selectedType\": \"zcode-bridge\" } },\n  \"model\": { \"name\": %q }\n}", apiKey, model, openAIBase, model),
			},
		},
		{
			ID: "kimi-code", Name: "Kimi Code", Description: "添加 OpenAI-compatible Provider 与默认模型",
			Model: model, Action: connectorAction("kimi-code"),
			Snippets: map[string]string{
				"~/.kimi/config.toml": fmt.Sprintf("default_model = \"zcode-bridge\"\n\n[providers.zcode-bridge]\ntype = \"openai\"\nbase_url = %q\napi_key = %q\n\n[models.zcode-bridge]\nprovider = \"zcode-bridge\"\nmodel = %q\nmax_context_size = 131072", openAIBase, apiKey, model),
			},
		},
		{
			ID: "opencode", Name: "OpenCode", Description: "OpenAI-compatible Provider 配置",
			Model: model, Action: connectorAction("opencode"),
			Snippets: map[string]string{
				"~/.config/opencode/opencode.json":    fmt.Sprintf("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"provider\": {\n    \"zcode-bridge\": {\n      \"npm\": \"@ai-sdk/openai-compatible\",\n      \"name\": %q,\n      \"options\": { \"baseURL\": %q, \"apiKey\": \"{file:~/.config/opencode/zcode-bridge-key}\" },\n      \"models\": { %q: { \"name\": %q } }\n    }\n  },\n  \"model\": %q\n}", connectorProviderName, openAIBase, model, model, "zcode-bridge/"+model),
				"~/.config/opencode/zcode-bridge-key": apiKey,
			},
		},
		{
			ID: "generic", Name: "通用 Agent / SDK", Description: "适用于支持 OpenAI 或 Anthropic 自定义地址的程序",
			Model: model,
			Snippets: map[string]string{
				"OpenAI compatible":    fmt.Sprintf("Base URL: %s\nAPI Key: %s\nModel: %s", openAIBase, apiKey, model),
				"Anthropic compatible": fmt.Sprintf("Base URL: %s\nAPI Key: %s\nModel: %s", baseURL, apiKey, model),
			},
		},
	}
	return connectors
}

func connectorAction(id string) string {
	return "connect-agent-" + id
}

func connectorIDFromAction(action string) (string, bool) {
	id := strings.TrimPrefix(action, "connect-agent-")
	if id == action {
		return "", false
	}
	switch id {
	case "deepseek-harness", "grok-build", "codex", "claude-code", "gemini-cli", "qwen-code", "kimi-code", "opencode":
		return id, true
	default:
		return "", false
	}
}

func deepSeekHarnessSnippets(openAIBase, apiKey, model string, imageInput bool) map[string]string {
	input := "[text]"
	if imageInput {
		input = "[text, image]"
	}
	return map[string]string{
		"$DSH_HOME/settings.yaml":     fmt.Sprintf("llm-pi-ai:\n  providers:\n    zcode-bridge:\n      displayName: %s\n      apiKeyEnv: ZCODE_BRIDGE_API_KEY\n      api: openai-completions\n      baseURL: %s\n      compat:\n        supportsDeveloperRole: false\n        maxTokensField: max_tokens\n      models:\n        - id: %s\n          name: %s\n          input: %s\nagent-default-model:\n  provider: zcode-bridge\n  model: %s", connectorProviderName, openAIBase, model, model, input, model),
		"$DSH_HOME/.credentials.yaml": fmt.Sprintf("version: 1\nrefs:\n  ZCODE_BRIDGE_API_KEY: %s", apiKey),
	}
}
