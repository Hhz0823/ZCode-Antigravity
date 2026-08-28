"use strict";

const GET_PATHS = new Set([
  "/api/status",
  "/api/connectors",
  "/api/manager",
  "/api/quota?provider=antigravity",
  "/api/quota?provider=xai",
  "/api/usage?provider=antigravity",
  "/api/usage?provider=xai",
]);

const POST_PATHS = new Set([
  "/api/action",
  "/api/provider",
  "/api/manager/settings",
  "/api/heartbeat",
  "/api/close",
]);

function assertApiPath(method, value) {
  if (typeof value !== "string") throw new Error("本机接口路径无效");
  const allowed = method === "GET" ? GET_PATHS : method === "POST" ? POST_PATHS : undefined;
  if (!allowed?.has(value)) throw new Error("不允许访问该本机接口");
  return value;
}

function normalizeConnection(value) {
  if (!value || typeof value !== "object") throw new Error("后台连接信息无效");
  const baseURL = value.baseURL ?? value.baseUrl;
  const session = value.session;
  if (typeof baseURL !== "string" || typeof session !== "string" || session.length < 16 || session.length > 4096) {
    throw new Error("后台连接信息缺少 baseURL 或 session");
  }
  const parsed = new URL(baseURL);
  if (parsed.protocol !== "http:" || parsed.hostname !== "127.0.0.1" || parsed.username || parsed.password || parsed.pathname !== "/" || parsed.search || parsed.hash) {
    throw new Error("后台仅允许使用 127.0.0.1 HTTP 地址");
  }
  return { baseURL: parsed.origin, session };
}

function assertXaiURL(value) {
  if (typeof value !== "string" || value.length > 4096) throw new Error("xAI 授权地址无效");
  const parsed = new URL(value);
  if (parsed.protocol !== "https:" || parsed.hostname !== "accounts.x.ai" || parsed.username || parsed.password) {
    throw new Error("只允许打开 xAI 官方授权地址");
  }
  return parsed.href;
}

function finiteNumber(value) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function trayTooltip(summary) {
  const provider = summary?.provider === "xai" ? "Grok" : "Antigravity";
  const parts = [`ZCode · ${provider}`];
  const fiveHour = finiteNumber(summary?.fiveHour);
  const week = finiteNumber(summary?.week);
  const tokensPerSecond = finiteNumber(summary?.tokensPerSecond);
  if (fiveHour !== undefined) parts.push(`5小时 ${fiveHour.toFixed(0)}%`);
  if (week !== undefined) parts.push(`本周 ${week.toFixed(0)}%`);
  if (tokensPerSecond !== undefined) parts.push(`${tokensPerSecond.toFixed(1)} tok/s`);
  if (parts.length === 1) parts.push("额度暂不可用");
  return parts.join(" · ");
}

module.exports = { assertApiPath, assertXaiURL, normalizeConnection, trayTooltip };
