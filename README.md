# ZCode Antigravity Bridge

> 在 Windows 与 macOS 上统一接入 Antigravity Gemini 和 xAI Grok，
> 并把本机安全网关提供给 ZCode、Grok Build、Codex、Claude Code 与 OpenCode。

**当前版本：** Windows / macOS `v0.5.2-test`
**适用系统：** macOS 12+（Apple Silicon / Intel）、Windows 10 / 11 x64
**项目状态：** 测试版


## 功能特性

- 可在控制中心切换 **Antigravity** 与 **Grok / xAI**；账号、模型、额度和 Agent 配置跟随切换。
- Antigravity 精确写入 `gemini-3.7-flash` / `gemini-3.6-flash`；Grok 只同步 xAI 返回的文本模型，自动排除 Imagine 图片和视频模型。
- 支持 Low / Medium / High 思考等级，并转换为 Gemini `thinkingLevel`。
- 支持文本、图片、音频和视频输入转换；当前 Gemini 3.7 Flash 声明输出仅为文本。
- Windows 任务栏与 macOS 菜单栏提供常驻额度小组件；可切换 Antigravity / Grok、刷新额度、打开面板或退出。
- 单击 Windows 任务栏图标或 macOS 菜单栏图标，可直接查看所选提供商的 **5 小时 / 本周剩余额度、重置时间、最近输出 Token 与 Token/s**；无对应窗口时明确显示“当前提供商未提供”。
- Antigravity 显示每周 / 5 小时余量；Grok 显示官方 Grok Build billing 返回的共享周/月余量、重置时间和 Extra Usage Credits。
- 额度面板每 5 分钟自动刷新；手动刷新、切换提供商和接入完成会立即刷新。刷新失败时保留上次成功数据。
- 本地网关按协议返回的真实 usage 数据显示最近输出 Token、推理 Token、生成速度和本地累计输出；有首字节时间时使用“输出 Token ÷ 生成阶段耗时”，否则明确标为“有效吞吐”。统计不保存提示词或回复。
- Antigravity / Grok 使用独立的原生选择卡、账号数、选中态与切换进度；切换期间保留各自最近成功额度，避免旧请求覆盖新提供商界面。
- 核心管理闭环参考 [Antigravity Tools](https://github.com/lbjlaq/Antigravity-Manager) 的公开产品能力：账号管家、OpenAI / Anthropic / Gemini 三协议代理、轮询 / 加权 / 填满优先路由、会话亲和、401/429 重试、凭据轮换、Agent 默认模型与本机设置持久化；代码与素材均为本项目独立实现。
- 原生界面参考 [codexU](https://github.com/shanggqm/codexU) 的信息层级，重做为蓝紫液态玻璃背景、半透明圆角卡片、顶部胶囊导航和分层额度 / Token 指标。macOS 使用窗口后方 `NSVisualEffectView`；Windows 结合 DWM Desktop Acrylic 与真实桌面采样的三轮可分离高斯模糊，背景不再是静态渐变模拟。
- Antigravity 额度请求支持 sandbox / daily / production 端点、HTTP 403 无项目字段重试和逐模型额度降级。
- 内置 Grok Build、OpenAI Codex、Claude Code、OpenCode、通用 OpenAI / Anthropic 客户端配置卡，可一键复制，不会擅自覆盖外部 Agent 配置文件。
- Windows 控制中心使用 **Rust + Win32** 原生开发，子进程全部隐藏；支持 Per-Monitor DPI V2、ClearType、系统托盘与原生任务栏图标。
- macOS 控制中心使用 **SwiftUI + AppKit** 原生开发，具有真正的 Dock 图标、原生菜单栏额度组件；Universal `.app` 同时兼容 Apple Silicon 与 Intel。
- 七页控制中心统一提供总览、账号、API 代理、模型路由、Agent 接入、用量统计和设置；右侧操作区始终保留本机 OAuth、网关与 ZCode 接入动作。
- 控制中心使用系统字体、原生控件、Per-Monitor DPI / Retina 响应式布局和页面切换反馈；不再依赖 Chrome 渲染，也不会因打开面板弹出终端窗口。
- 提供 v2rayN TUN / 代理预检、ZCode 配置备份、安全停止与同版本重装保护。
- 本地密钥随机生成；Windows 使用 DPAPI、macOS 使用登录钥匙串主密钥保护凭据；API 与 OAuth 回调仅监听 loopback。

## 下载

前往 [GitHub Releases](../../releases) 下载。

| 文件 | 用途 | 建议 |
| --- | --- | --- |
| `ZCode-Antigravity-macOS-Universal-v0.5.2-test.zip` | macOS Universal App 与维护脚本 | **Mac 用户首选** |
| `ZCode-Antigravity-Setup-v0.5.2-test.exe` | Windows 图形化安装器 | **普通用户首选** |
| `ZCode-Antigravity-OneClick-v0.5.2-test.bat` | 内嵌完整运行包的单文件安装器 | 备用方案 |
| `ZCode-Antigravity-Windows-x64-0.5.2-test.zip` | 可展开、可逐文件校验的便携包 | 手动部署 / 排错 |
| `ZCode-Antigravity-Source-v0.5.2-test.zip` | 当前源码快照 | 审计 / 构建 |

## macOS 快速安装

### 准备条件

- macOS 12 或更新版本，Apple Silicon 与 Intel 均可。
- 已安装 macOS 版 ZCode，并至少打开过一次。
- 能正常访问 Google / xAI 登录与相应模型服务；可使用系统 TUN，或自行配置可信的本机 HTTP/SOCKS5 代理。

### 安装步骤

1. 完整解压 macOS Universal ZIP；如需校验，打开 `Terminal Tools` 后双击 `Verify-Package.command`。
2. 完全退出 ZCode。当前 App 使用临时签名且尚未公证；首次运行请按住 Control 点击 `ZCode Antigravity.app`，选择“打开”。
3. 双击 `ZCode Antigravity.app`，选择 Antigravity 或 Grok；按需完成 Google OAuth 或 xAI 设备授权，再点击“一键接入 ZCode”。
4. 重新打开 ZCode，选择 `Antigravity + Grok (Local Bridge)` 下的目标模型，发送一条短消息验收。关闭原生窗口后菜单栏额度组件仍会保留。

Mac 运行数据位于 `~/Library/Application Support/ZCodeAntigravity`。Google token 使用
AES-256-GCM 加密，随机主密钥存放在 macOS 登录钥匙串。Intel 切片已完成交叉编译和
结构校验，但当前尚未完成 Intel 实机启动测试；详见包内 `README-macOS.txt`。

## Windows 快速安装

### 准备条件

- Windows 10 或 Windows 11 x64。
- 已安装 ZCode 3.7.x，并至少打开过一次。
- 已启动 v2rayN 并开启 TUN 模式。
- v2rayN mixed inbound 默认监听 `127.0.0.1:10808`。
- 可正常打开 Google / xAI 授权页面的浏览器。

### 安装步骤

1. 从 Windows 系统托盘完全退出 ZCode。仅关闭窗口可能仍会留下 `ZCode.exe` 进程。
2. 确认 v2rayN 已开启 TUN，且本地代理端口与配置一致。
3. 下载 `ZCode-Antigravity-Setup-v0.5.2-test.exe`，并先校验 SHA-256。Windows 请勿使用点击操作缺少反馈且 OAuth 过期后额度刷新失败的 `v0.4.3-test`、界面退化且高 DPI 字体异常的 `v0.4.2-test`，也不要继续使用更早测试版。
4. 双击安装器。程序会校验内嵌 ZIP 和三个 EXE，然后安装到当前用户目录。
5. 在控制中心选择 Antigravity 或 Grok，完成对应授权并等待状态检查通过。
6. 重新打开 ZCode，选择 Provider `Antigravity + Grok (Local Bridge)`；控制中心会作为任务栏额度小组件继续驻留。
7. 先发送一条短消息进行小规模验收。账号权限和实时额度以第一次真实请求为准。

## 本地端口与代理

| 服务 | 默认地址 | 占用时行为 |
| --- | --- | --- |
| 本地 API | `127.0.0.1:18080` | 自动扫描 `18081–18180` |
| OAuth callback | `127.0.0.1:51121` | 自动扫描 `51122–51221` |
| GUI 控制中心 | `127.0.0.1:18200–18250` | 在范围内选择可用端口 |
| Windows v2rayN 代理 | `http://127.0.0.1:10808` | 需与 `settings.json` 保持一致 |

支持 `http`、`https` 和 `socks5` 代理方案。如果 v2rayN 本地端口已修改，
请先停止 Bridge，再修改展开包中 `settings.json` 的 `proxyURL`。

macOS 包默认不固定代理端口，可直接使用系统 TUN；也可以在包内 `.env` 设置
`HTTP_PROXY` / `HTTPS_PROXY`，或修改 App 的 `Contents/Resources/settings.json`。

安装前也可设置自定义代理端口：

```powershell
$env:ZCODE_ANTIGRAVITY_PROXY_PORT = '10808'
```

## 已验证范围

Windows 基线与 `v0.5.2-test` 新增的构建及实机测试记录包括：

- 管理器与 CLIProxyAPI 的 Go 测试、Windows x64 构建和静态检查通过。
- OAuth PKCE / callback、DPAPI 凭据存储、原子替换和 loopback 约束通过。
- Gemini 3.7 Flash High 与 Gemini 3.6 Flash High 的真实文本请求通过。
- 图片理解与视频时序理解通过；当前不宣称支持原生位图生成。
- 单 BAT 自解包、载荷和三 EXE 哈希校验、同版本重装通过。
- 原生 EXE 安装器的隔离安装、再安装与无终端 GUI 行为通过。
- EXE 内嵌 PowerShell 脚本现在由构建器和运行时双重保证 UTF-8 BOM，并加入中文脚本编码回归测试，修复 Windows PowerShell 5.1 的 `ParserError: UnexpectedToken`。
- 已在构建主机复现中文代码页 936 会产生 `UnexpectedToken`，并确认新 EXE 只内嵌一个 `EF BB BF + param(` 脚本头。
- Windows 全部 GUI 子进程使用 `CREATE_NO_WINDOW`；Windows x64 测试程序交叉编译通过。
- Rust Win32 控制中心通过 `cargo check --locked`、`x86_64-pc-windows-gnu` Release 交叉编译、PE32+ GUI 子系统和资源图标检查；当前构建主机未安装 `cargo-clippy`，因此不把 Clippy 写成已执行项目。
- Rust Win32 控制中心使用深蓝紫液态玻璃、顶部品牌栏、七页胶囊导航、半透明感分层卡片与 Segoe UI Variable 字体；按真实显示器 DPI 创建窗口并在 `WM_DPICHANGED` 时重建字体。
- Windows 11 实机用高对比五色色带和黑白细线作为窗口后方参考：窗口外线条保持锐利，窗口内线条经三轮可分离高斯模糊消失为连续色带；移动或缩放结束后会重新采样，卡片和文字保持不透明清晰。
- 账号、三协议代理、模型路由、会话亲和、重试策略、5/10 分钟刷新与液态玻璃设置均通过带当前用户会话密钥的本机 API 读写；账号 ID 与标签脱敏，接口不返回凭据。
- 双提供商切换、Grok billing 解析、文本模型过滤和五类 Agent 配置均有单元测试。
- Antigravity 额度 HTTP 403 去项目字段重试和 `fetchAvailableModels` 降级路径均有回归测试。
- 旧版网页控制面板仍保留为诊断回退入口；正式客户端已改为原生 SwiftUI / Win32。
- 已在 Windows 11 x64、144 DPI（150%）实机完成真实 Google AI Pro 额度刷新、完整窗口截图和响应性回归；四个状态卡与全部操作按钮均在窗口内可见。
- `v0.5.2-test` 在同一实机显示账号摘要、最低 98%、分组额度进度条和重置时间；状态与 Token 统计每 5 秒更新，但不会改写独立的 5 分钟额度缓存周期。
- 单击 Windows 任务栏通知区域的小组件会同时恢复主窗口、置于前台并打开额度小组件；双击会直接唤醒主窗口。
- 同机真实模型响应记录为输出 45 Token、生成阶段 31.1 Token/s；本地统计文件不含 API key、OAuth token、prompt 或回复字段。单击任务栏图标会打开 codexU 风格原生弹层，可见 5 小时 100%、本周 98%、重置时间和最近吞吐。
- 同机已从主界面切换到“设置”页并打开任务栏额度组件；液态玻璃文本对比度、完整窗口边界、七页导航和 150% DPI 布局正常。“一键接入 ZCode”会启动当前候选包内的本地网关，不再出现点击无反馈。
- 已实测 Antigravity → Grok → Antigravity 切换：Grok 0 账号页面不显示 Gemini Token，切回后立即恢复 Antigravity 的额度与 Token 缓存；安装器启动控制中心后也会自动执行一次接入。
- 已修复旧版复用网关时把状态路径误写成新版本目录的问题；升级只允许停止同一 `ZCodeAntigravity/app-*` 根目录下的历史后端，仍拒绝处理无关进程。

`v0.5.2-test` 的 macOS 验证包括：

- 管理器、Keychain 凭据加密与后端相关测试通过。
- SwiftUI 主程序、Go Core 和后端均为真正的 `x86_64 + arm64` Universal Mach-O。
- App 临时签名、包内逐文件哈希、ZIP 完整性和隔离 `doctor` 检查通过。
- Info.plist 已启用普通 App 激活策略（`LSUIElement=false`），Dock 图标、原生菜单栏组件、关闭窗口驻留和退出清理通过。
- 本机已识别 `/Applications/ZCode.app` 及运行中的 `ZCode` 进程；未修改用户现有配置。
- `v0.5.2-test` SwiftUI 原生界面已实机启动；液态玻璃主窗口、七页导航、5 分钟刷新设置、提供商切换、状态卡和原生菜单栏额度组件显示正常。
- macOS 主窗口改为透明 `NSWindow` 与 `.behindWindow` 的 `NSVisualEffectView`；高对比背景实测可透过主窗口并被系统模糊，卡片层仍保持可读。
- 单击 macOS 菜单栏额度入口会同步取消隐藏或恢复主窗口、激活应用并置于最前层，同时显示额度小组件。

注意：上游模型目录、账号资格、风控和额度都可能随时变化。列表中看到模型并不代表当前账号一定可用。

## 隐私与安全边界

- Google / xAI access、refresh token 和运行状态保存在当前用户的 ZCodeAntigravity 数据目录。
- OAuth token 字段在 Windows 下使用当前用户 DPAPI 保护。
- OAuth token 字段在 macOS 下用登录钥匙串中的随机主密钥进行 AES-256-GCM 加密。
- ZCode `config.json` 只写入随机本地网关密钥，不写入 Google token。
- API、OAuth callback 和管理路由只监听 `127.0.0.1`，远程管理与 Web 控制面板默认关闭。
- 每次修改 ZCode 配置前会创建有上限的备份。
- 发布包不包含 OAuth token、账号 JSON、本地 API key、运行日志或本机 ZCode 配置。
- Windows 高斯玻璃仅在内存中读取当前窗口后方像素用于渲染，移动或缩放后覆盖旧采样；不会写入文件、上传或加入用量统计。
- 停止 Bridge 或移除 Provider 不会自动撤销 Google / xAI 授权；如需彻底撤销，请在对应账号中单独操作。

## 常见问题

### Windows 提示未知发布者

当前安装器、Rust 控制中心和 Go Core 均未使用商业代码签名证书。先对照本页或 `SHA256SUMS.txt`
校验哈希；仅在哈希完全一致时选择继续运行。

### macOS 提示无法验证开发者

当前 Mac App 使用临时签名，没有 Apple Developer ID 签名或公证。先运行
`Verify-Package.command`；哈希通过后，按住 Control 点击 App 并选择“打开”。
不要全局关闭 Gatekeeper。

### 一直重连或请求超时

Windows 检查 v2rayN 是否运行、TUN 是否开启及 `settings.json` 的 `proxyURL`；
macOS 检查系统 TUN，或 `.env` 中的 `HTTP_PROXY` / `HTTPS_PROXY` 是否指向
真正监听的可信 loopback 端口。

### 提示 ZCode 仍在运行

请彻底退出 ZCode。Bridge 不会在 ZCode 仍运行时写入需要修改的配置。

### 401 或没有模型

在控制中心确认当前选择了正确的提供商。Antigravity 可运行 `Login-Antigravity`，
Grok 可运行 `Login-Grok`，然后重新启动并同步网关。

### Grok 额度为什么只有一条周/月额度

xAI 目前把 Build、Chat、Imagine 等产品计入统一共享用量池。面板按官方 Grok Build
billing 响应显示当前周期，不把本地 token 数估算成账号余量。若上游暂时不返回
billing 配置，面板会明确显示错误，不会沿用 Antigravity 的百分比。

### 如何接入其他 Agent

先启动网关，在控制中心切换到目标提供商，再展开“接入更多 Agent 程序”。复制对应的
Grok Build、Codex、Claude Code 或 OpenCode 配置即可。实现依据为
[Grok Build 自定义模型端点](https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-shell/README.md)、
[Codex 自定义 Provider](https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json) 与
[Claude Code LLM Gateway](https://docs.anthropic.com/en/docs/claude-code/llm-gateway)。

### 403、429 或模型不可用

通常属于上游账号资格、风控或额度边界。项目不应绕过验证、资格或风控限制。

## 从源码构建

源码包含 SwiftUI macOS 客户端、Rust Win32 客户端、Go 本地 Core、测试、打包脚本、
固定的 CLIProxyAPI v7.2.132 源码和可重放补丁。已验证 Windows x64 交叉编译与 macOS Universal 构建。
提交修改前请另见 [`CONTRIBUTING.md`](CONTRIBUTING.md)。

GitHub 源码不内置 Google OAuth 客户端凭据。开发者需使用自己有权使用的
OAuth 桌面应用配置，并在运行源码构建前设置：

```powershell
$env:ANTIGRAVITY_OAUTH_CLIENT_ID = '<your-client-id>'
$env:ANTIGRAVITY_OAUTH_CLIENT_SECRET = '<your-client-secret>'
```

macOS Universal 打包：

```bash
cd project/packaging/macos
ANTIGRAVITY_OAUTH_CLIENT_ID='<your-client-id>' \
ANTIGRAVITY_OAUTH_CLIENT_SECRET='<your-client-secret>' \
./Build-Universal.sh
```

不提供 OAuth 环境变量时仍可构建，但运行时必须通过 `.env` 或进程环境提供它们。
发布脚本生成双架构 Mach-O、临时签名 `.app`、包内 SHA-256 清单和 ZIP 校验文件。

公开源码的默认构建从进程环境读取这两个值，不应提交到 Git。发布维护者可在链接时注入；
两种方式都未配置时，OAuth 登录和 token 刷新会返回明确错误。

Windows 原生客户端使用 Rust 2024 Edition 和 Microsoft `windows-sys` 绑定。交叉编译示例：

```bash
rustup target add x86_64-pc-windows-gnu
cd project/native/windows
cargo clippy --target x86_64-pc-windows-gnu -- -D warnings
cargo build --release --target x86_64-pc-windows-gnu
```

同一发布包中的 `ZCode-Antigravity.exe` 是隐藏运行的 Go Core，Rust 客户端通过一次性
loopback 会话与它通信；OAuth、token 加密和模型路由仍由经过测试的 Core/后端负责。

后端固定为 `router-for-me/CLIProxyAPI v7.2.132`，本地修改位于：

```text
project/docs/CLIProxyAPI-v7.2.132-zcode.patch
```

该文件是对上游版本的历史业务补丁。GitHub 发布树中将 OAuth 凭据改为环境变量的安全改动以当前源码为准。

## 第三方说明与许可

- 上游项目：[`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI)
- 原生 Windows 客户端：Rust、[`windows-sys`](https://github.com/microsoft/windows-rs)、`ureq`、`serde`
- 兼容回退任务栏组件：[`gogpu/systray`](https://github.com/gogpu/systray) `v0.2.8`
- 固定版本：`v7.2.132`
- 固定提交：`78f0c4079e3e6273d65d03b5549cffc898703264`
- CLIProxyAPI 与 systray 上游许可：MIT License；平台依赖许可见发布包第三方说明。

上游 MIT License 仅适用于对应的 CLIProxyAPI 内容。本项目其余代码尚未在本仓库顶层声明独立许可证，
请勿自动将整个项目视为 MIT 授权。商标和产品名称归各自权利人所有。

## 免责声明

本项目仅用于研究、开发和测试。使用者应自行确认适用的服务条款、账号政策、
网络规则和当地法律要求，并自行承担使用未公开接口带来的风险。
