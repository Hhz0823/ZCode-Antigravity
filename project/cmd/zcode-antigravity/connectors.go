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
	model := preferredConnectorModel(provider, models)
	baseURL := g.app.gatewayURL(current.Port)
	writeJSON(w, http.StatusOK, connectorResponse{
		Provider:   provider,
		BaseURL:    baseURL,
		Model:      model,
		Connectors: buildAgentConnectors(baseURL, g.app.apiKey, model),
	})
}

func preferredConnectorModel(provider string, models []modelInfo) string {
	preferred := []string{"gemini-3.7-flash", "gemini-3.6-flash"}
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

func buildAgentConnectors(baseURL, apiKey, model string) []agentConnector {
	openAIBase := strings.TrimRight(baseURL, "/") + "/v1"
	connectors := []agentConnector{
		{
			ID: "grok-build", Name: "Grok Build", Description: "让官方 Grok 终端 Agent 使用本地桥接模型",
			Model: model,
			Snippets: map[string]string{
				"macOS / Linux":      fmt.Sprintf("export GROK_MODELS_BASE_URL=%q\nexport GROK_MODELS_LIST_URL=%q\nexport XAI_API_KEY=%q\ngrok", openAIBase, openAIBase+"/models", apiKey),
				"Windows PowerShell": fmt.Sprintf("$env:GROK_MODELS_BASE_URL = %q\n$env:GROK_MODELS_LIST_URL = %q\n$env:XAI_API_KEY = %q\ngrok", openAIBase, openAIBase+"/models", apiKey),
			},
		},
		{
			ID: "codex", Name: "OpenAI Codex", Description: "添加一个 Responses API 自定义模型提供商",
			Model: model,
			Snippets: map[string]string{
				"~/.codex/config.toml": fmt.Sprintf("model = %q\nmodel_provider = \"zcode_bridge\"\n\n[model_providers.zcode_bridge]\nname = \"ZCode Local Bridge\"\nbase_url = %q\nenv_key = \"ZCODE_BRIDGE_API_KEY\"\nwire_api = \"responses\"", model, openAIBase),
				"环境变量":                 fmt.Sprintf("ZCODE_BRIDGE_API_KEY=%s", apiKey),
			},
		},
		{
			ID: "claude-code", Name: "Claude Code", Description: "通过 Anthropic 兼容接口连接本地网关",
			Model: model,
			Snippets: map[string]string{
				"macOS / Linux":      fmt.Sprintf("export ANTHROPIC_BASE_URL=%q\nexport ANTHROPIC_AUTH_TOKEN=%q\nexport ANTHROPIC_MODEL=%q\nclaude", baseURL, apiKey, model),
				"Windows PowerShell": fmt.Sprintf("$env:ANTHROPIC_BASE_URL = %q\n$env:ANTHROPIC_AUTH_TOKEN = %q\n$env:ANTHROPIC_MODEL = %q\nclaude", baseURL, apiKey, model),
			},
		},
		{
			ID: "opencode", Name: "OpenCode", Description: "OpenAI-compatible Provider 配置",
			Model: model,
			Snippets: map[string]string{
				"opencode.json": fmt.Sprintf("{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"providers\": {\n    \"zcode-bridge\": {\n      \"package\": \"@opencode-ai/ai/providers/openai-compatible\",\n      \"name\": \"ZCode Local Bridge\",\n      \"settings\": {\n        \"baseURL\": %q,\n        \"apiKey\": %q\n      },\n      \"models\": { %q: { \"name\": %q } }\n    }\n  },\n  \"model\": %q\n}", openAIBase, apiKey, model, model, "zcode-bridge/"+model),
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
