# ZCode Antigravity v0.6.7-test

这个测试版重做了 Windows / macOS 的液态玻璃分层：只有白色窗口底板由系统高斯模糊，导航、卡片、按钮和文字保持高对比度，并针对拖动和滚动卡顿继续减少实时合成层。

## 主要变化

- Windows Electron 正式模式仅使用 DWM Acrylic 模糊窗口底板，不再由 Chromium 对整窗重复执行 `backdrop-filter`。
- Windows 隐藏动画光团与噪点层，禁用页面入场动画，滚动容器使用 layout/paint/style containment。
- macOS 保留唯一原生 `NSVisualEffectView`，删除三个大尺寸 SwiftUI 模糊色团、`drawingGroup` 和根层隐式切换动画。
- 两端内容面提升到接近实色的白色，深色文字、蓝色主操作和彩色状态不会被窗口后方文字干扰。
- 保留 Antigravity / Grok 切换、五小时/本周额度、Token 与 Token/s、独立任务栏/菜单栏小组件、v2rayN 自动代理和多 Agent 一键接入。

## 验证

- Windows 渲染模式连续滚动 3 轮，每轮 180 帧：P95 17.4–17.5 ms，540 帧中仅 2 帧超过 20 ms，活动渲染动画为 0。
- Apple Silicon macOS 使用最终 Universal 包实际启动；账号、额度、网关和 ZCode 接入状态正常，连续滚到底部再回顶成功。
- Go 管理器测试/竞态测试/静态检查、CLIProxyAPI 模块校验与全量测试、Electron IPC/CSS 回归、React/Tailwind 生产构建、SwiftUI 双架构类型检查通过。
- macOS ZIP 签名/Universal Mach-O/包内 SHA-256 与 Windows ZIP/EXE/BAT/PE32+ GUI/ASAR 验证通过。

Windows Electron 新底板的 DWM 透明度仍建议在实际 Windows 10/11 的不同 DPI 与壁纸上复测；本记录不把 macOS 上的 Chromium 帧时当作 Windows DWM 实机帧时。

## 下载建议

- Windows 普通用户：`ZCode-Antigravity-Setup-v0.6.7-test.exe`
- macOS 用户：`ZCode-Antigravity-macOS-Universal-v0.6.7-test.zip`
- Windows 便携排错：`ZCode-Antigravity-Windows-x64-0.6.7-test.zip`
- Windows 单文件备用安装：`ZCode-Antigravity-OneClick-v0.6.7-test.bat`
- 审计/二次开发：`ZCode-Antigravity-Source-v0.6.7-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请先对照发布页的 SHA-256；Windows SmartScreen 或 macOS Gatekeeper 可能提示未知开发者。
