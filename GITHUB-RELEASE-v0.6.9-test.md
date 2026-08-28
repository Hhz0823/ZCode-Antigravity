# ZCode Antigravity v0.6.9-test

这个版本优化 macOS 菜单栏额度组件：常驻区域只保留一个小图标，点击仍显示完整的 Antigravity / Grok 额度与 Token 卡片。

## 主要变化

- macOS 菜单栏状态项改为系统方形宽度，不再常驻显示 `5h xx% · 周 xx%`，显著减少菜单栏占用。
- 完整的 5 小时、本周额度、重置时间、最近输出 Token 与 Token/s 仍保留在点击后的 Popover 中。
- 修复 Popover 打开后，点击主窗口、其他应用或桌面空白处不能稳定自动关闭的问题。
- 外部点击监听只在 Popover 显示期间启用，关闭后立即释放，不增加常驻轮询或动画负担。
- 保留菜单栏悬停提示，可快速查看当前提供商、账号数和额度摘要。
- Windows 功能与 `v0.6.8-test` 保持一致，并统一更新为 `v0.6.9-test` 安装包。

## 使用方式

1. 安装并打开 ZCode Antigravity。
2. 点击 macOS 菜单栏中的单个额度图标，查看完整额度卡片。
3. 再点击卡片外部的窗口或桌面空白处，卡片会自动收起。
4. 只有点击卡片内“打开主界面”才会显示控制中心。

## 验证

- Swift arm64 / x86_64 macOS 12 类型检查通过。
- 原生候选 App 运行时可访问性检查确认状态项标题为空、尺寸为 `24 × 24`；旧版同一状态项为 `160.5 × 24`。
- Popover 的应用内、应用外点击关闭路径与关闭后监听释放逻辑通过双架构编译检查。
- macOS Universal App 完成双架构、临时签名、ZIP CRC 与包内 SHA-256 检查。
- Go Core、CLIProxyAPI、Electron IPC 与 Windows x64 交叉构建回归通过。

## 下载建议

- macOS 用户：`ZCode-Antigravity-macOS-Universal-v0.6.9-test.zip`
- Windows 普通用户：`ZCode-Antigravity-Setup-v0.6.9-test.exe`
- Windows 便携排错：`ZCode-Antigravity-Windows-x64-0.6.9-test.zip`
- Windows 单文件备用安装：`ZCode-Antigravity-OneClick-v0.6.9-test.bat`
- 审计/二次开发：`ZCode-Antigravity-Source-v0.6.9-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请先对照发布页的 SHA-256；Windows SmartScreen 或 macOS Gatekeeper 可能提示未知开发者。
