# ZCode Antigravity v0.4.1-test

这是针对 Windows 原生控制中心启动失败的修复版本。

## 修复内容

- 修复启动时报错：`后台连接信息无效: missing field baseUrl`。
- Go Core 的规范字段是 `baseURL`；Rust 客户端现在显式按该字段解析，不再依赖默认 camelCase 转换。
- 同时修复 Agent 接入响应中的 `baseURL` 映射，否则启动修复后打开 Agent 配置仍会读取失败。
- 为两个协议入口增加可在 macOS 构建机真实执行的 Rust 回归测试。
- 保留 `baseUrl` 兼容别名，便于旧测试 Core 或第三方构建过渡。

## 验证

- 两个 Go JSON 协议回归测试执行通过。
- rustfmt、Clippy `-D warnings`、Windows target no-run test 与 Release 交叉编译通过。
- Windows ControlCenter 为 PE32+ GUI 子系统，仍使用 `CREATE_NO_WINDOW` 启动 Go Core。
- EXE、单 BAT、便携 ZIP 的内嵌载荷与逐文件 SHA-256 重新生成并验证。
- macOS Universal 包同步版本号重新构建；SwiftUI、Dock、签名和双架构边界不变。

请停止并退出旧版控制中心后安装 `v0.4.1-test`。Windows 用户优先下载
`ZCode-Antigravity-Setup-v0.4.1-test.exe`。
