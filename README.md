# ZCode Antigravity Bridge

> 让 ZCode 通过本机 Anthropic-compatible Provider 调用 Antigravity 中的
> Gemini 3.7 Flash 与 Gemini 3.6 Flash。

**当前版本：** `v0.2.6-test`  
**适用系统：** Windows 10 / 11 x64  
**项目状态：** 测试版


## 功能特性

- 只向 ZCode 写入 `gemini-3.7-flash` 和 `gemini-3.6-flash`。
- 支持 Low / Medium / High 思考等级，并转换为 Gemini `thinkingLevel`。
- 支持文本、图片、音频和视频输入转换；当前 Gemini 3.7 Flash 声明输出仅为文本。
- 提供无终端 Windows GUI 安装器，按当前用户安装，无需管理员权限。
- 蓝白控制中心统一显示代理、桥接、ZCode、账号、AI Credits 与额度状态。
- 提供 v2rayN TUN / 代理预检、ZCode 配置备份、安全停止与同版本重装保护。
- 本地密钥随机生成；Windows 凭据使用当前用户 DPAPI 保护；API 与 OAuth 回调仅监听 loopback。

## 下载

前往 [GitHub Releases](../../releases/tag/v0.2.6-test) 下载。

| 文件 | 用途 | 建议 |
| --- | --- | --- |
| `ZCode-Antigravity-Setup-v0.2.6-test.exe` | Windows 图形化安装器 | **普通用户首选** |
| `ZCode-Antigravity-OneClick-v0.2.6-test.bat` | 内嵌完整运行包的单文件安装器 | 备用方案 |
| `ZCode-Antigravity-Windows-x64-0.2.6-test.zip` | 可展开、可逐文件校验的便携包 | 手动部署 / 排错 |
| `ZCode-Antigravity-Source-v0.2.6-test.zip` | 当前版本的干净源码快照 | 审计 / 构建 |

## 快速安装

### 准备条件

- Windows 10 或 Windows 11 x64。
- 已安装 ZCode 3.7.x，并至少打开过一次。
- 已启动 v2rayN 并开启 TUN 模式。
- v2rayN mixed inbound 默认监听 `127.0.0.1:10808`。
- 可正常打开 Google 登录与 Antigravity 授权页面的浏览器。

### 安装步骤

1. 从 Windows 系统托盘完全退出 ZCode。仅关闭窗口可能仍会留下 `ZCode.exe` 进程。
2. 确认 v2rayN 已开启 TUN，且本地代理端口与配置一致。
3. 下载 `ZCode-Antigravity-Setup-v0.2.6-test.exe`，并先校验 SHA-256。
4. 双击安装器。程序会校验内嵌 ZIP 和三个 EXE，然后安装到当前用户目录。
5. 在控制中心中完成 Google OAuth，等待状态检查通过。
6. 重新打开 ZCode，选择 Provider `Antigravity (Local Bridge)`。
7. 先发送一条短消息进行小规模验收。账号权限和实时额度以第一次真实请求为准。

> [!NOTE]
> 安装包未使用商业代码签名，Windows SmartScreen 可能显示“未知发布者”。
> 只有在 SHA-256 完全一致时才应继续运行。

## 本地端口与代理

| 服务 | 默认地址 | 占用时行为 |
| --- | --- | --- |
| 本地 API | `127.0.0.1:18080` | 自动扫描 `18081–18180` |
| OAuth callback | `127.0.0.1:51121` | 自动扫描 `51122–51221` |
| GUI 控制中心 | `127.0.0.1:18200–18250` | 在范围内选择可用端口 |
| v2rayN 代理 | `http://127.0.0.1:10808` | 需与 `settings.json` 保持一致 |

支持 `http`、`https` 和 `socks5` 代理方案。如果 v2rayN 本地端口已修改，
请先停止 Bridge，再修改展开包中 `settings.json` 的 `proxyURL`。

安装前也可设置自定义代理端口：

```powershell
$env:ZCODE_ANTIGRAVITY_PROXY_PORT = '10808'
```

## 已验证范围

`v0.2.6-test` 的本地测试记录包括：

- 管理器与 CLIProxyAPI 的 Go 测试、Windows x64 构建和静态检查通过。
- OAuth PKCE / callback、DPAPI 凭据存储、原子替换和 loopback 约束通过。
- Gemini 3.7 Flash High 与 Gemini 3.6 Flash High 的真实文本请求通过。
- 图片理解与视频时序理解通过；当前不宣称支持原生位图生成。
- 单 BAT 自解包、载荷和三 EXE 哈希校验、同版本重装通过。
- 原生 EXE 安装器的隔离安装、再安装与无终端 GUI 行为通过。

注意：上游模型目录、账号资格、风控和额度都可能随时变化。列表中看到模型并不代表当前账号一定可用。

## 隐私与安全边界

- Google access / refresh token 和运行状态保存在 `%LOCALAPPDATA%\ZCodeAntigravity`。
- Antigravity token 字段在 Windows 下使用当前用户 DPAPI 保护。
- ZCode `config.json` 只写入随机本地网关密钥，不写入 Google token。
- API、OAuth callback 和管理路由只监听 `127.0.0.1`，远程管理与 Web 控制面板默认关闭。
- 每次修改 ZCode 配置前会创建有上限的备份。
- 发布包不包含 OAuth token、账号 JSON、本地 API key、运行日志或本机 ZCode 配置。
- 停止 Bridge 或移除 Provider 不会自动撤销 Google 授权；如需彻底撤销，请在 Google 账号中单独操作。

## 常见问题

### Windows 提示未知发布者

当前 EXE 为未签名的自定义 Go 二进制。先对照本页或 `SHA256SUMS.txt`
校验哈希；仅在哈希完全一致时选择继续运行。

### 一直重连或请求超时

检查 v2rayN 是否在运行、TUN 是否仍然开启，以及 `settings.json` 中的
`proxyURL` 是否指向真正监听的 loopback 端口。

### 提示 ZCode.exe 仍在运行

在系统托盘右键 ZCode 并选择退出。Bridge 不会在 ZCode 仍运行时写入需要修改的配置。

### 401 或没有模型

在展开包中运行 `Login-Antigravity.bat`，然后运行
`Start-ZCode-Antigravity.bat`。

### 403、429 或模型不可用

通常属于上游账号资格、风控或额度边界。项目不应绕过验证、资格或风控限制。

## 从源码构建

源码包包含管理器、Windows 脚本、测试、文档、固定的 CLIProxyAPI v7.2.132
源码和可重放补丁。已验证构建环境为 Go 1.26.5、Windows x64、`CGO_ENABLED=0`。

```powershell
cd project
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go test ./...
go build -trimpath -ldflags='-s -w' `
  -o ZCode-Antigravity.exe ./cmd/zcode-antigravity
go build -trimpath -ldflags='-s -w -H windowsgui -X main.defaultCommand=gui' `
  -o ZCode-Antigravity-ControlCenter.exe ./cmd/zcode-antigravity
```

后端固定为 `router-for-me/CLIProxyAPI v7.2.132`，本地修改位于：

```text
project/docs/CLIProxyAPI-v7.2.132-zcode.patch
```

## 第三方说明与许可

- 上游项目：[`router-for-me/CLIProxyAPI`](https://github.com/router-for-me/CLIProxyAPI)
- 固定版本：`v7.2.132`
- 固定提交：`78f0c4079e3e6273d65d03b5549cffc898703264`
- CLIProxyAPI 上游许可：MIT License

上游 MIT License 仅适用于对应的 CLIProxyAPI 内容。本项目其余代码尚未在本仓库顶层声明独立许可证，
请勿自动将整个项目视为 MIT 授权。商标和产品名称归各自权利人所有。

## 免责声明

本项目仅用于研究、开发和测试。使用者应自行确认适用的服务条款、账号政策、
网络规则和当地法律要求，并自行承担使用未公开接口带来的风险。
