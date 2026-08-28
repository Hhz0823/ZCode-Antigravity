# ZCode Antigravity v0.6.10-test

这个版本统一精简 Agent 中的 Provider 显示名：ZCode 和支持自定义名称的 Agent 现在只显示 **Google**。

## 主要变化

- ZCode Provider 从 `Antigravity + Grok (Local Bridge)` 改为 `Google`。
- OpenAI Codex、Grok Build、OpenCode 与 DeepSeek Harness 的 Provider 显示名统一为 `Google`。
- Agent 内部 ID 继续使用 `zcode-antigravity-local`、`zcode-bridge` 或 `zcode_bridge`，保证已有配置可以原位升级，不生成重复 Provider。
- Claude Code、Gemini CLI 等仅通过环境变量接入且没有 Provider 显示名的客户端保持原协议配置不变。
- Antigravity / Grok 切换、模型清单、额度、Token 统计、联网搜索和本地地址均不受影响。
- 保留 v0.6.9 的 macOS 单图标菜单栏与点击外部自动收起功能。

## 升级后同步

1. 完全退出 ZCode 和需要更新显示名的 Agent。
2. 打开 ZCode Antigravity，点击“修复并重新同步”或对应 Agent 的“一键接入”。
3. 重新打开 Agent，Provider 将显示为 `Google`。

## 验证

- ZCode 配置生成回归确认 Provider `name` 精确为 `Google`。
- Codex、Grok Build、OpenCode、DeepSeek Harness 的真实配置合并测试确认显示名为 `Google`，且保留已有用户设置。
- 旧的本机回环 `zcode-bridge` 配置可安全原位更新；非本程序、非回环的同名配置仍拒绝覆盖。
- Go Core、CLIProxyAPI、Electron IPC/生产构建与 macOS Swift 双架构检查通过。
- macOS Universal、Windows x64 和源码发布包通过签名/结构、ZIP CRC、逐文件 SHA-256 与敏感文件检查。

## 下载建议

- macOS：`ZCode-Antigravity-macOS-Universal-v0.6.10-test.zip`
- Windows：`ZCode-Antigravity-Setup-v0.6.10-test.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-0.6.10-test.zip`
- Windows 单文件包：`ZCode-Antigravity-OneClick-v0.6.10-test.bat`
- 源码：`ZCode-Antigravity-Source-v0.6.10-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请校验发布页 SHA-256。
