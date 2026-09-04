"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { assertApiPath, assertUpdateInstaller, assertXaiURL, normalizeConnection, trayTooltip } = require("./protocol.cjs");

test("accepts only the fixed local API allowlist", () => {
  assert.equal(assertApiPath("GET", "/api/status"), "/api/status");
  assert.equal(assertApiPath("POST", "/api/action"), "/api/action");
  assert.equal(assertApiPath("GET", "/api/update"), "/api/update");
  assert.equal(assertApiPath("POST", "/api/update"), "/api/update");
  assert.throws(() => assertApiPath("GET", "/api/action"), /不允许/);
  assert.throws(() => assertApiPath("GET", "http://example.com"), /不允许/);
});

test("accepts only a verified-shape installer inside the update directory", () => {
  const download = {
    version: "1.1.0",
    platform: "windows",
    assetName: "ZCode-Antigravity-Setup-v1.1.0.exe",
    path: "C:\\Users\\test\\AppData\\Local\\ZCodeAntigravity\\updates\\1.1.0\\ZCode-Antigravity-Setup-v1.1.0.exe",
    sha256: "a".repeat(64),
  };
  const root = "C:\\Users\\test\\AppData\\Local\\ZCodeAntigravity\\updates";
  assert.equal(assertUpdateInstaller(download, root), download.path);
  assert.throws(() => assertUpdateInstaller({ ...download, path: "C:\\Windows\\System32\\cmd.exe" }, root), /专用目录|名称/);
  assert.throws(() => assertUpdateInstaller({ ...download, platform: "darwin" }, root), /平台/);
  assert.throws(() => assertUpdateInstaller({ ...download, sha256: "bad" }, root), /校验值/);
});

test("normalizes the Go native-host connection acronym", () => {
  assert.deepEqual(
    normalizeConnection({ baseURL: "http://127.0.0.1:18200", session: "0123456789abcdef" }),
    { baseURL: "http://127.0.0.1:18200", session: "0123456789abcdef" },
  );
  assert.throws(() => normalizeConnection({ baseURL: "https://example.com", session: "0123456789abcdef" }), /127\.0\.0\.1/);
});

test("opens only the official xAI authorization origin", () => {
  assert.equal(assertXaiURL("https://accounts.x.ai/sign-in?code=ABC"), "https://accounts.x.ai/sign-in?code=ABC");
  assert.throws(() => assertXaiURL("https://accounts.x.ai.evil.example/sign-in"), /官方/);
});

test("builds the independent tray quota summary", () => {
  assert.equal(
    trayTooltip({ provider: "antigravity", fiveHour: 92.4, week: 78.2, tokensPerSecond: 43.26 }),
    "ZCode · Antigravity · 5小时 92% · 本周 78% · 43.3 tok/s",
  );
  assert.equal(trayTooltip({ provider: "xai" }), "ZCode · Grok · 额度暂不可用");
});

test("delegates desktop blur to DWM without animated renderer blur", () => {
  const css = fs.readFileSync(path.join(__dirname, "..", "src", "index.css"), "utf8");
  assert.match(css, /\.electron-shell \.app-shell,[\s\S]*?backdrop-filter: none;/);
  assert.match(css, /\.electron-shell \.liquid-orb,[\s\S]*?\.electron-shell \.noise-layer \{ display: none; \}/);
  assert.match(css, /\.electron-shell \.page-content \{ animation: none; \}/);
  assert.match(css, /\.electron-shell \.main-panel \{[\s\S]*?contain: layout paint style;/);
  assert.match(css, /\.glass-card \{[^\n]*background: rgba\(250,251,255,\.96\);/);
});
