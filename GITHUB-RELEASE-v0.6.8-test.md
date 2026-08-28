# ZCode Antigravity v0.6.8-test

这个版本修复“Gemini 回答看起来没有联网搜索”：ZCode 模型列表新增独立的 **Gemini Web Search (Google)**，用于需要当前网页信息和可核对来源的问题。

## 主要变化

- 新增客户端模型 `gemini-web-search`，在 ZCode 中显示为 `Gemini Web Search (Google)`。
- 该模型固定路由到 Antigravity 支持原生 Google Search 的搜索通道，不依赖 ZCode 是否自动附加搜索工具。
- 搜索响应会返回 `server_tool_use`、`web_search_tool_result`、`web_search_requests` 和网页引用。
- 修复 OAuth 模型别名在路由时丢失“联网搜索”标记的问题。
- 保留 `gemini-3.7-flash` / `gemini-3.6-flash` 作为普通对话与编程模型，不会将每个请求强制变成搜索。
- Windows Electron 和 macOS SwiftUI 的现有界面、额度小组件、Grok 登录、自动代理与多 Agent 一键接入保持不变。

## 如何使用

1. 更新后完全退出 ZCode。
2. 打开 ZCode Antigravity，点击“修复并重新同步”或“一键接入 ZCode”。
3. 重新打开 ZCode，在 `Antigravity + Grok (Local Bridge)` 下选择 `Gemini Web Search (Google)`。
4. 询问最新新闻、官网当前内容或其他需要联网的问题；回答应包含可检查来源。

## 验证

- Go Core 和 CLIProxyAPI 搜索请求/响应/别名执行器回归测试通过。
- 使用当前本机 Antigravity 账号完成真实 Anthropic `/v1/messages` 请求：实际触发 Google Search 并返回搜索结果块与来源引用。额外使用 3 条不包含“联网/搜索”字样的问题连续验证，每条均为 `web_search_requests=1`，分别返回 2、7、3 条引用。
- 可重放的 CLIProxyAPI v7.2.132 补丁已重新生成且不携带上游 OAuth 字面量；对固定上游先运行 `sanitize_upstream_oauth.go`，再执行 `git apply --check --whitespace=error-all` 通过。

## 下载建议

- Windows 普通用户：`ZCode-Antigravity-Setup-v0.6.8-test.exe`
- macOS 用户：`ZCode-Antigravity-macOS-Universal-v0.6.8-test.zip`
- Windows 便携排错：`ZCode-Antigravity-Windows-x64-0.6.8-test.zip`
- Windows 单文件备用安装：`ZCode-Antigravity-OneClick-v0.6.8-test.bat`
- 审计/二次开发：`ZCode-Antigravity-Source-v0.6.8-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请先对照发布页的 SHA-256；Windows SmartScreen 或 macOS Gatekeeper 可能提示未知开发者。
