# ZCode Antigravity V1.0.0 正式版

V1.0.0 将此前连续验证的测试版功能整理为第一条正式稳定发布线。默认保持 Gemini-only，
需要时再在设置中开启 Grok / xAI 与其他文本模型。

## 正式版重点

- **Gemini + 可选 Grok**：Antigravity 默认启用；Grok 和其他文本模型按需打开，额度、模型与缓存相互隔离。
- **Google 原生联网搜索**：独立的 `Gemini Web Search (Google)` 模型返回搜索结果、网页引用和可检查的搜索请求数据。
- **多模态理解**：支持文本、图片、音频和视频输入转换，保持文本输出边界。
- **更多 Agent 一键接入**：支持 ZCode、DeepSeek Harness、Grok Build、Codex、Claude Code、Gemini CLI、Qwen Code、Kimi Code 与 OpenCode。
- **不强制开启 TUN**：Windows 可自动使用系统代理或 v2rayN 本机代理，均不可用时才直连。
- **桌面额度小组件**：macOS 菜单栏和 Windows 系统托盘显示额度、重置时间、最近输出 Token、推理 Token 与 Token/s。
- **本地安全网关**：管理接口和 OAuth 回调只监听 `127.0.0.1`；Windows 使用 DPAPI，macOS 使用钥匙串主密钥加密凭据。
- **双平台统一体验**：macOS 使用 SwiftUI / AppKit 原生客户端，Windows 使用 Electron / React / Tailwind，高透明玻璃背景与高对比内容层并存。

## 下载

- macOS：`ZCode-Antigravity-macOS-Universal-v1.0.0.zip`
- Windows 推荐安装器：`ZCode-Antigravity-Setup-v1.0.0.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-1.0.0.zip`
- Windows 单文件安装器：`ZCode-Antigravity-OneClick-v1.0.0.bat`
- 源码快照：`ZCode-Antigravity-Source-v1.0.0.zip`
- 全部文件校验：`SHA256SUMS-v1.0.0.txt`

## 升级与使用

1. 下载对应系统的正式版并对照 `SHA256SUMS-v1.0.0.txt` 校验。
2. 完全退出正在运行的 ZCode 和旧版 ZCode Antigravity。
3. 安装 / 打开 V1.0.0，登录 Antigravity，然后点击“一键接入 ZCode”。
4. 重新打开 ZCode，在 Provider `Google` 下选择 Gemini；需要网页信息时选择 `Gemini Web Search (Google)`。
5. 如需 Grok，先在设置中启用，再完成 xAI 设备授权并重新同步。

升级不会主动删除账号、额度缓存或 ZCode 聊天；修改客户端配置前会创建备份。

## 发布验证与边界

- Go Core、CLIProxyAPI、Electron IPC / 生产构建与 Swift 双架构类型检查均通过。
- macOS Universal 包完成双架构、临时签名、包内哈希和 Apple Silicon 本机启动 / 网关健康验证。
- Windows 包完成 x64 交叉构建、ASAR 内容、PE32+ GUI 子系统、安装载荷与哈希验证。
- Windows V1.0.0 本轮未在目标 Windows 10/11 主机重新运行；README 和验证记录没有把交叉构建冒充为实机验证。
- macOS Intel 切片完成交叉编译和结构检查，但尚未完成 Intel 实机启动测试。
- 当前 Windows 安装器没有商业代码签名，macOS App 使用临时签名且尚未公证。请务必先核对 SHA-256。
