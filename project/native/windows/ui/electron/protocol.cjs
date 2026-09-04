"use strict";

const path = require("node:path");

const GET_PATHS = new Set([
  "/api/status",
  "/api/connectors",
  "/api/manager",
  "/api/update",
  "/api/quota?provider=antigravity",
  "/api/quota?provider=xai",
  "/api/usage?provider=antigravity",
  "/api/usage?provider=xai",
]);

const POST_PATHS = new Set([
  "/api/action",
  "/api/provider",
  "/api/manager/settings",
  "/api/update",
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

function assertUpdateInstaller(value, updatesRoot) {
  if (!value || typeof value !== "object" || value.platform !== "windows") throw new Error("更新资产平台无效");
  if (typeof value.version !== "string" || !/^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$/.test(value.version)) throw new Error("更新版本号无效");
  const expectedName = `ZCode-Antigravity-Setup-v${value.version}.exe`;
  if (value.assetName !== expectedName || typeof value.path !== "string") throw new Error("更新资产名称无效");
  if (typeof value.sha256 !== "string" || !/^[0-9a-fA-F]{64}$/.test(value.sha256)) throw new Error("更新资产校验值无效");
  if (typeof updatesRoot !== "string" || !updatesRoot.trim()) throw new Error("更新目录无效");
  const root = path.win32.resolve(updatesRoot);
  const installer = path.win32.resolve(value.path);
  const relative = path.win32.relative(root, installer);
  if (!relative || relative.startsWith(`..${path.win32.sep}`) || relative === ".." || path.win32.isAbsolute(relative)) throw new Error("更新资产超出应用专用目录");
  if (path.win32.basename(installer) !== expectedName || path.win32.basename(path.win32.dirname(installer)) !== value.version) throw new Error("更新资产路径无效");
  return installer;
}

module.exports = { assertApiPath, assertUpdateInstaller, assertXaiURL, normalizeConnection, trayTooltip };
