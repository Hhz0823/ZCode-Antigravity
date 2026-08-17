# ZCode Antigravity v0.4.2-test

这是针对 Antigravity 登录成功后额度面板仍显示 HTTP 403 的修复版本。

## 修复内容

- 额度汇总请求新增当前 Antigravity 使用的 sandbox、daily 与 production 三层端点。
- 请求携带项目 ID 返回 HTTP 403 时，会在同一端点自动去掉项目字段重试。
- 汇总额度仍不可用时，自动降级到 `fetchAvailableModels`，显示可读取的 Gemini 逐模型额度，不再让整个额度卡报错。
- Windows 与 macOS 的额度请求 User-Agent 现在使用真实目标系统和架构，不再在 Windows 包中错误声明 `darwin/arm64`。
- 保留套餐与 AI Credits 独立读取；降级数据会在面板中明确标注，避免冒充完整周额度/5 小时汇总。

## 验证

- 新增 HTTP 403 去项目字段重试回归测试。
- 新增汇总接口拒绝后逐模型额度降级回归测试。
- Go 完整测试、Race、Vet 与 Windows x64 构建通过。
- Rust Win32 控制中心格式、Clippy、Windows target 测试编译与 Release 交叉编译通过。
- Windows EXE、单 BAT、便携 ZIP 与 macOS Universal ZIP 重新构建并校验 SHA-256。

请完全退出旧版控制中心和任务栏小组件后安装 `v0.4.2-test`。Windows 用户优先下载
`ZCode-Antigravity-Setup-v0.4.2-test.exe`，登录完成后点击“刷新”验证额度。
