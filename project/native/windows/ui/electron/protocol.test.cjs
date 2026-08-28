"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { assertApiPath, assertXaiURL, normalizeConnection, trayTooltip } = require("./protocol.cjs");

test("accepts only the fixed local API allowlist", () => {
  assert.equal(assertApiPath("GET", "/api/status"), "/api/status");
  assert.equal(assertApiPath("POST", "/api/action"), "/api/action");
  assert.throws(() => assertApiPath("GET", "/api/action"), /不允许/);
  assert.throws(() => assertApiPath("GET", "http://example.com"), /不允许/);
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
