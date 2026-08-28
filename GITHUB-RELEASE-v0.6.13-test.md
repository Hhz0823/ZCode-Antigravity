# ZCode Antigravity v0.6.13-test

这个版本重新修复 macOS 状态栏额度小组件首次弹出时进度条变灰的问题。

## 为什么 0.6.12 没有生效

0.6.12 尝试强制 SwiftUI 的窗口活动状态，但原生 `ProgressView` 底层控件仍会根据真实 key-window
状态降低颜色，因此用户第一次打开时仍可能看到灰色，点击组件后才恢复。

## 0.6.13 的处理

- 删除无效的窗口状态强制覆盖。
- 小组件不再使用原生 `ProgressView`。
- 蓝色 / 紫色额度条由普通 SwiftUI 图形直接绘制，不读取窗口焦点或 App 活动状态。
- 不会激活整个 App，也不会把控制中心主窗口抬到其他软件上层。
- 保留点击外部自动关闭、后台额度刷新和无障碍百分比描述。

## 下载建议

- macOS：`ZCode-Antigravity-macOS-Universal-v0.6.13-test.zip`
- Windows：`ZCode-Antigravity-Setup-v0.6.13-test.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-0.6.13-test.zip`
- Windows 单文件包：`ZCode-Antigravity-OneClick-v0.6.13-test.bat`
- 源码：`ZCode-Antigravity-Source-v0.6.13-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请校验发布页 SHA-256。
