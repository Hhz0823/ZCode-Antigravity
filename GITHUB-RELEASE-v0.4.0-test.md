# ZCode Antigravity v0.4.0-test

本次测试版把用户可见的控制中心全面原生化，同时保留经过测试的 Go/CLIProxyAPI
本地安全核心，不改写 OAuth、token 加密和模型路由边界。

## 主要变化

- macOS：改用 SwiftUI + AppKit 原生 App，修复程序坞不显示图标；加入原生菜单栏额度组件。
- Windows：改用 Rust 2024 + Win32 原生客户端，以 x86_64 GNU 工具链交叉编译。
- Windows：启用 Per-Monitor DPI V2、Segoe UI Variable、ClearType、DPI 变化自动重排和原生任务栏图标。
- 两端普通启动均不再依赖 Chrome 渲染控制中心，也不会弹出终端窗口。
- Antigravity / Grok xAI 可切换，账号数、模型、额度和 Agent 配置随提供商切换。
- 继续支持 Grok Build、Codex、Claude Code、OpenCode 与通用 OpenAI / Anthropic 客户端。
- 新增随机会话密钥保护的 `native-host` loopback 通道；父客户端退出时 Core 自动关闭并释放端口。
- 新增可重复的 Windows 交叉打包脚本、Rust 锁文件、依赖清单和完整第三方许可文件。

## 已验证

- Go Core：`go test ./...`、Race、Vet 通过。
- CLIProxyAPI：`go mod verify` 与完整 `go test ./...` 通过。
- Windows Rust：rustfmt、Clippy `-D warnings`、Windows target no-run test、Release 交叉编译通过。
- Windows：ControlCenter 和安装器均为 PE32+ GUI 子系统；Core/后端为 CUI，但由 GUI 使用 `CREATE_NO_WINDOW` 隐藏启动。
- EXE 与单 BAT 的内嵌 ZIP、三个可执行文件哈希、PowerShell UTF-8 BOM 和逐文件 SHA-256 通过。
- macOS：SwiftUI 主程序、Go Core、后端均为 arm64 + x86_64 Universal Mach-O。
- macOS：临时签名、App Icon、`LSUIElement=false`、Dock 注册、菜单栏/子进程生命周期、ZIP 与包内哈希通过。

## 尚需目标机回归

- Windows 10/11 真实设备的多显示器混合 DPI、安装/重装和 OAuth 双击流程仍需用户验收。
- Intel Mac 切片已完成构建和结构检查，但未在 Intel 实机启动。
- 当前 Windows EXE 未使用商业代码签名；macOS App 未使用 Developer ID 公证。

请优先下载对应平台的 Setup/Universal 包，并先校验 `SHA256SUMS.txt`。
