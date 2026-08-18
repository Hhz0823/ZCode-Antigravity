# ZCode Antigravity v0.6.4-test

本测试版统一了 Windows 与 macOS 的浅色液态玻璃界面，修复状态栏点击会连带置顶主窗口的问题，并针对双平台滚动卡顿完成合成层减负。

## 主要变化

- Windows Tauri 2 控制中心改为与 macOS 原生版一致的布局：顶部品牌栏、横向七页导航、Antigravity / Grok 双提供商切换、四个连接状态卡、额度与 Token 区、右侧接入控制。
- Windows 主窗口和任务栏额度小组件使用浅色 DWM Acrylic；桌面背景由系统实时模糊，不读取或保存桌面截图。
- Windows 系统托盘单击只显示独立额度小组件，不恢复、不聚焦、不置顶主控制中心；小组件可切换提供商并显示额度、重置时间、累计输出 Token 与 Token/s。
- macOS 菜单栏单击只显示原生额度 Popover；只有明确点击“打开主界面”才激活控制中心。
- Windows 移除可滚动卡片上的重复 `backdrop-filter`；macOS 移除滚动内容中的嵌套 SwiftUI `Material`，改用轻量半透明填充和 `LazyVStack`。
- 滚动或拖动期间暂停背景光晕及非必要过渡，降低 WebView2 / SwiftUI 的实时合成负担。
- 保留 v0.6.3 的 Grok 网关恢复后额度自动重取、xAI 设备验证码、Antigravity 403 降级与双提供商缓存隔离修复。

## 实机验证

- Windows 11 x64：最终控制中心 SHA-256 与安装包一致，原生 Acrylic、真实 Grok 账号与额度、连续上下滚动、网关启动和系统托盘独立小组件通过。
- Apple Silicon macOS：SwiftUI 主窗口连续上下滚动、菜单栏 Popover 与主窗口独立激活通过。
- Windows React/Tailwind 生产构建、Rust GNU x64 Release 交叉编译、Go 全量测试、CLIProxyAPI 模块验证与全量测试、SwiftUI 类型检查通过。
- Windows 与 macOS ZIP 完整性、包内 SHA-256、PE32+ GUI、Universal Mach-O 和 macOS 临时签名校验通过。

## 下载建议

- Windows 普通用户：`ZCode-Antigravity-Setup-v0.6.4-test.exe`
- macOS 用户：`ZCode-Antigravity-macOS-Universal-v0.6.4-test.zip`
- Windows 便携排错：`ZCode-Antigravity-Windows-x64-0.6.4-test.zip`
- 单文件备用安装：`ZCode-Antigravity-OneClick-v0.6.4-test.bat`

当前构建没有商业代码签名或 Apple 公证。运行前请对照 `SHA256SUMS-v0.6.4-test.txt`；Windows SmartScreen 或 macOS Gatekeeper 可能提示未知开发者。
