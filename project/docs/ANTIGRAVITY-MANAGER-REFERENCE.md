# Antigravity Manager 参考说明

ZCode Antigravity `v0.4.7-test` 的额度面板与定时刷新体验参考了
[lbjlaq/Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager) 的产品思路，
包括账号摘要、分组额度、彩色进度条、重置时间和后台周期刷新。

- 参考快照：`a2e3c45`（2026-08-15）
- 参考项目许可证：CC BY-NC-SA 4.0
- 许可证原文：<https://github.com/lbjlaq/Antigravity-Manager/blob/main/LICENSE>

本项目没有引入参考项目的 Tauri、React 组件或源码文件。Windows 界面由 Rust +
Win32 原生绘制，macOS 界面由 SwiftUI 原生实现；刷新调度和数据适配也是本项目的
独立实现。该说明用于记录设计与行为参考来源，不改变本项目其他源文件或第三方组件
各自的许可证。
