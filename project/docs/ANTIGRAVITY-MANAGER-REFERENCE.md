# Antigravity Manager 参考说明

ZCode Antigravity `v0.5.1-test` 的核心管理闭环参考了
[lbjlaq/Antigravity-Manager](https://github.com/lbjlaq/Antigravity-Manager)（当前产品名 Antigravity Tools）
公开展示的产品思路，包括账号摘要、分组额度、彩色进度条、重置时间、后台周期刷新、
多协议代理、账号路由和失败重试。

- 参考快照：`a2e3c45`（2026-08-15）
- 参考项目许可证：CC BY-NC-SA 4.0
- 许可证原文：<https://github.com/lbjlaq/Antigravity-Manager/blob/main/LICENSE>

本项目的对齐范围为：

- 脱敏账号列表与 OAuth 登录入口；
- OpenAI Chat Completions / Responses、Anthropic Messages、Gemini SDK 三类本机兼容端点；
- 轮询、加权轮询、填满优先、会话亲和与后台 / Agent 默认模型；
- 请求重试、凭据轮换、最大退避、5 分钟额度刷新和预警阈值；
- 输出 / 推理 Token、Token/s、额度窗口与 Agent 接入配置。

本项目没有引入参考项目的 Tauri、React 组件、源码文件或资源。Windows 界面由 Rust +
Win32 原生绘制，macOS 界面由 SwiftUI 原生实现；设置存储、路由配置生成、刷新调度和数据适配
也是本项目的独立实现。参考项目使用 CC BY-NC-SA 4.0，因此本文只记录公开产品行为的参考边界，
不把其代码许可证扩散到本项目其他源文件或第三方组件。
