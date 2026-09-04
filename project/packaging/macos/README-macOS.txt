ZCode Antigravity Bridge - macOS Universal test build
====================================================

版本：1.0.2-test
架构：Apple Silicon (arm64) + Intel (x86_64)
最低系统：macOS 12

用途
----
本程序使用原生 SwiftUI + AppKit 控制中心，让 macOS 版 ZCode 和其他 Agent
通过只监听本机的兼容 Provider 调用 Antigravity Gemini、Google 账号内的 Claude 与 xAI Grok 文本模型。
App 会在程序坞显示图标；菜单栏只保留一个方形额度图标，不再常驻显示 5 小时和本周文字。
主窗口只在白色底板使用透明 NSWindow 与 behindWindow NSVisualEffectView 的系统高斯材质；导航、卡片、按钮和文字保持高对比度，并提供总览、账号、API 代理、模型路由、
Agent 接入、用量统计和设置七个原生页面。
Agent 页面可一键备份并接入 DeepSeek Harness、Grok Build、Codex、Claude Code、Gemini CLI、
Qwen Code、Kimi Code 与 OpenCode；通用客户端配置仍可复制。
额度卡片按账号和时间窗口显示彩色进度条、重置时间与最低余量；程序每 5 分钟
自动刷新一次额度，手动刷新、切换提供商和接入完成后会立即刷新。
单击菜单栏图标只显示独立额度 Popover（5 小时 / 本周余量与重置时间），不会把
主窗口带到其他应用上层；点击 Popover 外部的窗口或桌面空白处会自动收起。主界面显示协议返回的
最近输出 Token、推理 Token、Token/s 和本地累计输出，不保存提示词或回复。

这是非官方测试版，会使用未公开的 Antigravity 接口。Google 可能限流、暂停
API 权限或封禁账号。请只使用专门的测试账号，不要使用主 Gmail、Workspace
或 Google Cloud 所有者账号。

首次使用
--------
1. 完整解压 ZIP，不要直接在压缩包预览中运行。
2. 如需先校验文件，打开“Terminal Tools”后双击 Verify-Package.command，确认均为 OK。
3. 完全退出 ZCode；仅关闭窗口不一定会退出应用。
4. 首次打开时，按住 Control 点击“ZCode Antigravity.app”，选择“打开”。
   当前 App 使用临时签名，没有 Apple Developer ID，也没有公证。
5. 双击“ZCode Antigravity.app”，在原生控制中心选择 Antigravity 或 Grok / xAI。
6. 按需完成 Google OAuth 或 xAI 设备授权，再点击“一键接入 ZCode”。
   Google OAuth 本地回调最长等待 30 分钟；如果页面明确提示超时，请回到程序
   重新点击登录，不要重新加载旧的 localhost 回调页。
7. 在 ZCode 中选择 Google 下的目标模型，先发送一条短消息验收。
   需要实时网页信息时选择 Gemini Web Search (Google)；回答应包含来源引用。
   需要 Claude 时，先在“设置”开启“Google Claude / 其他模型”并点击“应用模型开关”。
8. 关闭原生窗口后菜单栏额度组件继续运行；程序坞或 Popover 内的“打开主界面”可重开窗口。

普通使用只需打开 App，不会启动 Chrome，也不会弹出 Terminal。“Terminal Tools”中的 .command 是
维护和诊断入口，双击它们时 macOS 会按设计显示终端窗口。如果已把 App 拖入
/Applications，这些脚本仍会自动找到它。

网络与代理
----------
默认使用直连 / 系统网络，无需开启 TUN。macOS TUN 模式仍可透明接管流量，也可在 .env 中设置
HTTP_PROXY / HTTPS_PROXY，或编辑 App 内 Contents/Resources/settings.json 的
proxyURL。支持 http、https 和 socks5，且应只指向可信的本机代理。

端口
----
- 本地 API：127.0.0.1:18080，冲突时扫描到 18180
- OAuth callback：127.0.0.1:51121，冲突时扫描到 51221
- 原生客户端会话：127.0.0.1:18200-18250（只允许随机会话密钥访问）

数据与安全
----------
- 运行数据：~/Library/Application Support/ZCodeAntigravity
- ZCode 配置：~/.zcode/v2/config.json，或 setting.json 指向的数据目录
- 修改 ZCode 配置前会建立有上限的备份。
- 本地 API key 为随机值；API、OAuth callback 和控制中心只监听 127.0.0.1。
- Google / xAI access/refresh token 用随机 AES-256-GCM 主密钥加密；主密钥保存在
  macOS 登录钥匙串，服务名为 io.github.hhz0823.zcode-antigravity。
- 发布包不包含用户 token、账号 JSON、本地 API key、日志或 ZCode 配置。
- usage-metrics.json 只保存提供商、模型、Token 数、耗时与时间戳，不保存提示词、
  模型回复、API key、OAuth token 或完整账号信息。
- 停止 Bridge 或删除 Provider 不会撤销 Google 授权。

源码构建
--------
公开源码不包含 OAuth 客户端身份。源码构建需复制 .env.example 为 .env，填入
自己有权使用的桌面 OAuth 客户端配置。SwiftUI App 会读取完整解压目录根部的 .env；
也可通过 Run.command 启动终端工具。发布维护者可在
运行 Build-Universal.sh 时通过同名环境变量把配置注入二进制；脚本不会把它写入
源码树或打包成明文配置文件。为避免再次发布无法登录的包，构建脚本默认拒绝缺少
OAuth 配置的产物；仅本地开发且明确从运行时环境提供配置时，才可设置
ALLOW_RUNTIME_OAUTH_CONFIG=1 跳过该发布门禁。

终端诊断脚本
------------
以下脚本位于“Terminal Tools”，运行时会显示终端，仅用于维护或排错：

- Setup-and-Start.command：首次登录、启动并写入 ZCode Provider
- Login-Antigravity.command：添加或刷新账号
- Login-Grok.command：添加或刷新 Grok / xAI 账号
- Start-ZCode-Antigravity.command：启动网关并同步模型
- Status.command：查看状态
- Doctor.command：本机静态检查
- Test-Gemini-3.8-Flash.command：使用最新 Gemini 3.8 Flash 发送一次小型真实请求
- Test-Gemini-3.7-Flash.command：发送一次小型真实请求
- Test-Claude-Sonnet-4.6.command：使用 Google Antigravity 账号发送 Claude Sonnet 4.6 真实请求
- Test-Grok-Build.command：使用 grok-build-0.1 发送一次小型真实请求
- Stop-ZCode-Antigravity.command：只停止本程序记录且路径匹配的网关
- Remove-ZCode-Provider.command：只删除本程序管理的 Provider

已验证与未验证边界
------------------
- 已在 Apple Silicon Mac 上完成单元测试、SwiftUI 原生窗口与菜单栏启动、Universal
  双架构构建、Mach-O 架构、Dock 图标、App 结构、临时签名和包内哈希检查。
- SwiftUI 主程序、Go Core 与 CLIProxyAPI 后端均为 arm64 + x86_64 Universal；
  普通启动不会打开 Chrome 或 Terminal。
- Intel 切片通过交叉编译和结构验证，但没有 Intel 实机启动测试。
- 当前没有 Apple Developer ID 签名或 Apple 公证；Gatekeeper 可能提示开发者未知。
- 账号资格、上游模型目录、额度和风控以用户第一次真实请求为准。
