# ZCode Antigravity V1.0.3 正式版

V1.0.3 将 Gemini 3.8 Flash、Google 账号内的 Claude 模型与安全自动更新带入稳定发布线。
新安装默认仍只显示 Gemini；Grok / xAI、Google Claude 和其他文本模型需在设置中主动开启。

## 本版重点

- **Gemini 3.8 Flash**：新增干净的 `gemini-3.8-flash` 客户端模型，精确映射已验证的 High 路由，支持 Low / Medium / High 思考等级。
- **Google Claude**：可选启用 `claude-sonnet-4-6` 与 `claude-opus-4-6-thinking`，复用 Antigravity Google 账号，无需另填 Anthropic API Key。
- **自动检测与安全更新**：启动后及每 6 小时检查 GitHub 正式版；支持手动更新和可选自动安装，下载后核对平台、文件名、大小与 GitHub SHA-256。
- **更新后自动恢复**：macOS 先备份旧 App、验证新 App 并支持失败回滚；Windows 使用无终端更新模式。两端安装后都会重启并重新同步本地网关。
- **Gemini 原生联网搜索**：选择 `Gemini Web Search (Google)` 可返回 Google Search 结果、请求计数与网页引用。
- **多模态与 Agent 接入**：Gemini 支持文本、图片、音频和视频输入；可一键接入 ZCode、DeepSeek Harness、Grok Build、Codex、Claude Code、Gemini CLI、Qwen Code、Kimi Code 与 OpenCode。
- **无需强制 TUN**：Windows 自动使用系统代理或运行中的 v2rayN 本机代理，不可用时回退直连。
- **额度与 Token 小组件**：macOS 菜单栏和 Windows 系统托盘显示额度、重置时间、输出 Token、推理 Token 与 Token/s。

## 下载

- macOS：`ZCode-Antigravity-macOS-Universal-v1.0.3.zip`
- Windows 推荐安装器：`ZCode-Antigravity-Setup-v1.0.3.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-1.0.3.zip`
- Windows 单文件安装器：`ZCode-Antigravity-OneClick-v1.0.3.bat`
- 源码快照：`ZCode-Antigravity-Source-v1.0.3.zip`
- 全部资产校验：`SHA256SUMS-v1.0.3.txt`

## 安装与升级

1. 下载对应系统文件，并使用 `SHA256SUMS-v1.0.3.txt` 核对 SHA-256。
2. 完全退出 ZCode；只关闭窗口可能仍会保留后台进程。
3. macOS 解压后打开 `ZCode Antigravity.app`；Windows 推荐运行 Setup EXE。
4. 登录 Antigravity，点击“一键接入 ZCode”，再打开 ZCode 并选择 Provider `Google`。
5. 以后可在“设置 → 软件更新”手动检查或开启自动安装。

升级保留账号、加密凭据、额度缓存、Token 统计和 ZCode 聊天；修改客户端配置前仍会创建备份。

## 发布验证与边界

- Go Core 的测试、vet、race，固定 CLIProxyAPI 的依赖与全量测试，Electron IPC / 生产构建和 Swift 双架构类型检查均通过。
- macOS Universal 包完成双架构、临时签名、包内哈希和 Apple Silicon 本机自更新 / 网关恢复验证。
- Windows 包完成 x64 交叉构建、ASAR、PE32+ GUI 子系统、安装载荷与哈希验证。
- Windows 测试机本轮 SSH 超时，因此没有把 macOS 构建主机上的交叉验证写成新的 Windows 实机通过。
- macOS Intel 切片完成交叉编译和结构检查，但尚未完成 Intel 实机启动测试。
- Windows 安装器尚无商业代码签名，macOS App 使用临时签名且尚未公证；运行前请先核对 SHA-256。
