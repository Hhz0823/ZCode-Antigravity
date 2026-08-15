# ZCode Antigravity Bridge v0.2.7-test — macOS Universal Preview

这是首个 macOS 预览版，同一个 ZIP 同时包含 Apple Silicon (`arm64`) 与 Intel (`x86_64`) 切片。

## 下载

- `ZCode-Antigravity-macOS-Universal-v0.2.7-test.zip`：macOS App、首次设置与维护脚本、包内校验清单。
- `ZCode-Antigravity-macOS-Universal-v0.2.7-test.zip.sha256`：ZIP 的独立 SHA-256。

Windows 用户请继续使用 [`v0.2.6-test`](https://github.com/Hhz0823/ZCode-Antigravity/releases/tag/v0.2.6-test)。

## Mac 新增能力

- Universal Mach-O：Apple Silicon 与 Intel 双架构。
- 可双击的 `ZCode Antigravity.app`，复用 loopback 蓝白控制中心。
- 原生 `open`、ZCode.app 检测/启动、`utun` 检测和基于可执行文件真实路径的安全停止。
- access/refresh token 使用 AES-256-GCM；随机主密钥保存于 macOS 登录钥匙串。
- 包内逐文件 SHA-256、ZIP 独立哈希和 `.command` 维护脚本。

## 安装

1. 完整解压 ZIP，先运行 `Verify-Package.command`。
2. 完全退出 ZCode。
3. 首次启动按住 Control 点击 App，选择“打开”。
4. 运行 `Setup-and-Start.command` 完成登录、启动和 Provider 同步。

## 已知边界

- 当前使用临时签名，没有 Apple Developer ID 签名或 Apple 公证。
- Apple Silicon 已完成本机构建、运行和隔离 `doctor` 检查。
- Intel 切片完成交叉编译、Mach-O、签名和归档校验，尚未进行 Intel 实机测试。
- 这是调用未公开 Antigravity 接口的非官方测试版；请勿使用主账号测试。
