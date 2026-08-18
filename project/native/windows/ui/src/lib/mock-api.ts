const now = new Date();
const resetFive = new Date(now.getTime() + 3.4 * 60 * 60 * 1000).toISOString();
const resetWeek = new Date(now.getTime() + 4.2 * 24 * 60 * 60 * 1000).toISOString();

const status = {
  version: "0.6.1-test",
  gateway: { ok: true, label: "网关在线", detail: "http://127.0.0.1:18080", running: true },
  proxy: { ok: true, label: "本机代理在线", detail: "127.0.0.1:10808", running: true },
  tun: { ok: true, label: "TUN 已开启", detail: "xray_tun", running: true },
  zcode: { ok: true, label: "ZCode 已接入", detail: "http://127.0.0.1:18080", running: true },
  providerAccounts: { antigravity: 2, xai: 1 },
  selectedProvider: "antigravity",
  models: ["gemini-3.7-flash", "gemini-3.6-flash"],
  operation: { running: false },
  updatedAt: now.toISOString(),
};

function quota(provider: "antigravity" | "xai") {
  return {
    fetchedAt: now.toISOString(), provider, source: "Local quota API", stale: false,
    accounts: provider === "xai" ? [{ account: "grok-user", plan: "SuperGrok", status: "active", groups: [{ name: "Grok", buckets: [{ name: "共享模型", window: "week", remainingPercent: 76, resetTime: resetWeek }] }], credits: { available: true, amount: 1280, creditType: "xAI credits" } }]
      : [{ account: "y*****g@gmail.com", plan: "Google AI Pro", status: "active", groups: [{ name: "Gemini 模型", buckets: [{ name: "当前窗口", window: "5 小时", remainingPercent: 91, resetTime: resetFive }, { name: "周额度", window: "week", remainingPercent: 73, resetTime: resetWeek }] }] }],
  };
}

const usage = {
  provider: "antigravity", available: true,
  latest: { timestamp: now.toISOString(), model: "gemini-3.7-flash", outputTokens: 1428, reasoningTokens: 218, totalTokens: 2180, latencyMs: 11980, ttftMs: 940, generationMs: 11040, outputTokensPerSecond: 129.3, speedBasis: "generation" },
  total: { requests: 38, outputTokens: 28640, reasoningTokens: 5312, averageTokensPerSecond: 116.8 },
  recent: Array.from({ length: 18 }, (_, index) => ({ timestamp: new Date(now.getTime() - (18 - index) * 180000).toISOString(), model: index % 3 ? "gemini-3.7-flash" : "gemini-3.6-flash", outputTokens: 620 + index * 37, reasoningTokens: 120 + index * 4, latencyMs: 8400 + index * 180, ttftMs: 700, generationMs: 7700 + index * 180, outputTokensPerSecond: 72 + ((index * 13) % 64), speedBasis: "generation" })),
};

const manager = {
  version: "0.6.1-test",
  accounts: [{ id: "ag-demo", provider: "antigravity", label: "y*****g@gmail.com", plan: "Google AI Pro", status: "active", updatedAt: now.toISOString() }, { id: "xai-demo", provider: "xai", label: "g***@example.com", plan: "SuperGrok", status: "active", updatedAt: now.toISOString() }],
  proxy: { running: true, baseURL: "http://127.0.0.1:18080", port: 18080, protocols: [{ name: "OpenAI", path: "/v1/chat/completions", description: "Chat Completions / Responses 兼容" }, { name: "Anthropic", path: "/v1/messages", description: "Claude Code 原生消息协议" }, { name: "Gemini", path: "/v1beta/models", description: "Google SDK 兼容协议" }] },
  routing: { strategy: "round-robin", sessionAffinity: true, sessionAffinityTTL: "1h", requestRetry: 3, credentialRetry: 3, retryInterval: 30, backgroundModel: "gemini-3.7-flash" },
  settings: { autoRefreshMinutes: 5, quotaWarningPercent: 20, proxyURL: "http://127.0.0.1:10808", theme: "dark", liquidGlass: true, settingsPath: "%LOCALAPPDATA%\\ZCodeAntigravity\\settings.json" },
  features: [{ id: "accounts", name: "多账号管家", description: "OAuth 登录、账号发现和脱敏状态", available: true }, { id: "protocols", name: "三协议中继", description: "OpenAI、Anthropic、Gemini", available: true }, { id: "routing", name: "模型路由", description: "轮询、加权与填满优先", available: true }, { id: "retry", name: "自动自愈", description: "401/429 重试与凭据轮换", available: true }, { id: "usage", name: "用量统计", description: "输出 Token、推理 Token 与 tok/s", available: true }],
};

export function hasTauri() {
  return "__TAURI_INTERNALS__" in window;
}

export async function mockGet(path: string): Promise<unknown> {
  if (path === "/api/status") return status;
  if (path === "/api/manager") return manager;
  if (path.startsWith("/api/quota")) return quota(path.includes("xai") ? "xai" : "antigravity");
  if (path.startsWith("/api/usage")) return { ...usage, provider: path.includes("xai") ? "xai" : "antigravity" };
  if (path === "/api/connectors") return { provider: "antigravity", baseURL: "http://127.0.0.1:18080", model: "gemini-3.7-flash", connectors: [{ id: "codex", name: "OpenAI Codex", description: "Responses API 自定义模型提供商", model: "gemini-3.7-flash", snippets: { "config.toml": "model_provider = \"zcode_bridge\"", "环境变量": "ZCODE_BRIDGE_API_KEY=local-only" } }, { id: "claude-code", name: "Claude Code", description: "Anthropic 兼容接口", model: "gemini-3.7-flash", snippets: { "PowerShell": "$env:ANTHROPIC_BASE_URL = \"http://127.0.0.1:18080\"" } }] };
  throw new Error("Unknown mock path");
}

export async function mockPost(path: string, body: Record<string, unknown>): Promise<unknown> {
  if (path === "/api/provider") status.selectedProvider = body.provider as "antigravity" | "xai";
  if (path === "/api/manager/settings") Object.assign(manager.settings, body);
  return path === "/api/manager/settings" ? manager : { ok: true };
}
