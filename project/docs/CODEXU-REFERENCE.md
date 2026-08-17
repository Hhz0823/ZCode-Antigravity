# codexU 参考说明

ZCode Antigravity `v0.4.6-test` 的任务栏额度信息层级与推理性能口径参考了
[shanggqm/codexU](https://github.com/shanggqm/codexU) 的公开产品说明：在常驻菜单中直接显示
5 小时 / 7 天额度与重置时间，并把“全部输出 Token ÷ 完整调用时长”明确称为有效吞吐，
不冒充可见文本解码 TPS。

- 参考快照：`eb28c8f`（2026-08-04）
- 参考项目许可证：MIT
- 许可证原文：<https://github.com/shanggqm/codexU/blob/main/LICENSE>

本项目没有引入 codexU 的 SwiftUI、Tauri、React、Rust 组件或源码文件。ZCode Antigravity
直接读取现有 CLIProxyAPI 的脱敏 usage 记录：有 TTFT 时以输出 Token 除以首字节后的生成阶段
耗时并标为“生成速度”；缺少 TTFT 时才使用完整调用时长，并明确标为“有效吞吐”。Windows
任务栏菜单由 Rust + Win32 构建，macOS 菜单栏由 SwiftUI + AppKit 构建。

本地持久化只包含提供商、模型、Token 数、延迟和时间戳，不包含 prompt、回复、API Key、
OAuth token 或完整账号信息。该说明只记录设计与行为参考来源，不改变项目及第三方组件各自
的许可证。
