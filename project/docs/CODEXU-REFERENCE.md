# codexU 参考说明

ZCode Antigravity `v0.5.0-test` 的主窗口、任务栏 / 菜单栏小组件、额度信息层级与
推理性能口径参考了 [shanggqm/codexU](https://github.com/shanggqm/codexU) 的公开界面和产品说明：
以蓝紫液态玻璃作为主视觉，使用系统高斯材质、柔光色块、半透明感圆角卡片、顶部胶囊导航、
高对比数据色和分层指标结构；常驻小组件直接显示 5 小时 / 7 天额度、重置时间、输出 Token 与 Token/s。

- 参考快照：`eb28c8f`（2026-08-04）
- 参考项目许可证：MIT
- 许可证原文：<https://github.com/shanggqm/codexU/blob/main/LICENSE>

本项目没有引入 codexU 的 SwiftUI、Tauri、React、Rust 组件、资源文件或运行时依赖；界面由
ZCode Antigravity 的 Rust + Win32 与 SwiftUI + AppKit 代码独立重建。Windows 使用 DWM
标题栏材质和深色自绘液态背景，避免浅色系统主题下全窗口玻璃导致文字失去对比；macOS 使用
`NSVisualEffectView` 与 SwiftUI 材质。ZCode Antigravity
直接读取现有 CLIProxyAPI 的脱敏 usage 记录：有 TTFT 时以输出 Token 除以首字节后的生成阶段
耗时并标为“生成速度”；缺少 TTFT 时才使用完整调用时长，并明确标为“有效吞吐”。Windows
任务栏弹层由 Rust + Win32 构建，macOS 菜单栏弹层由 SwiftUI + AppKit 构建。这里的“一比一”
指信息层级、交互密度和液态玻璃视觉语言对齐，不表示复制对方源码或资源。

本地持久化只包含提供商、模型、Token 数、延迟和时间戳，不包含 prompt、回复、API Key、
OAuth token 或完整账号信息。该说明只记录设计与行为参考来源，不改变项目及第三方组件各自
的许可证。
