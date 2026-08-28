# ZCode Antigravity v0.6.12-test

这个版本修复 macOS 状态栏额度小组件第一次弹出时配色变灰、必须再点击一次才恢复的问题。

## 修复内容

- 第一次点击菜单栏图标，Popover 立即显示完整的蓝色 / 紫色额度进度条。
- 不再需要点击弹出的页面进行第二次激活。
- 修复只覆盖小组件的 SwiftUI 控件外观状态，不会激活整个 App，也不会把控制中心主窗口抬到其他软件上层。
- 保留点击外部空白处自动关闭、一枚菜单栏图标、后台额度刷新和 Gemini-only 默认模型策略。

## 下载建议

- macOS：`ZCode-Antigravity-macOS-Universal-v0.6.12-test.zip`
- Windows：`ZCode-Antigravity-Setup-v0.6.12-test.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-0.6.12-test.zip`
- Windows 单文件包：`ZCode-Antigravity-OneClick-v0.6.12-test.bat`
- 源码：`ZCode-Antigravity-Source-v0.6.12-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请校验发布页 SHA-256。
