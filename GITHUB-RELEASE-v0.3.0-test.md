# ZCode Antigravity Bridge v0.3.0-test

这是一个面向 Windows 10/11 x64 与 macOS 12+ 的测试版。请先校验
`SHA256SUMS.txt`，再安装或运行。

## 本次更新

- 控制中心可明确切换 **Antigravity** / **Grok**；账号、模型、额度和 Agent 配置随所选提供商更新。
- 支持 xAI 设备授权、Grok 文本模型与 Grok Build 官方 billing 额度。
- 新增 Windows 任务栏 / macOS 菜单栏额度小组件，点击打开面板，可切换提供商并手动刷新。
- 新增 Grok Build、Codex、Claude Code、OpenCode 与通用 Agent 的本地接入配置卡。
- Windows GUI 与全部子进程保持隐藏控制台，修复中文 PowerShell 5.1 安装器解析问题。
- 控制面板改用系统字体、响应式 DPI 布局、高 DPR Canvas 与可降级动效，改善文字锯齿和不同分辨率显示。
- 一个提供商临时缺模型时不再阻断另一个提供商；多账号小组件按最低剩余额度提示。

## 下载建议

- macOS：`ZCode-Antigravity-macOS-Universal-v0.3.0-test.zip`
- Windows：`ZCode-Antigravity-Setup-v0.3.0-test.exe`
- Windows 备用：`ZCode-Antigravity-OneClick-v0.3.0-test.bat`
- 便携包：`ZCode-Antigravity-Windows-x64-0.3.0-test.zip`
- 源码快照：`ZCode-Antigravity-Source-v0.3.0-test.zip`

## 已知边界

- Windows EXE 尚未使用受信任的 Authenticode 证书签名；macOS App 为临时签名且未公证。
- 本版本已完成 Windows amd64 交叉编译与 macOS arm64 本机测试；仍建议在目标 Windows 10/11 多 DPI 设备、macOS Intel 设备上进行最终实机回归。
- 模型、账号资格、风控和额度由 Google / xAI 上游决定；看见模型不代表当前账号一定可用。
- 这是第三方桥接测试项目。请勿用重要主账号测试，也不要分享账号 JSON、token、日志或本地 API key。

完整安装、隐私边界、构建和排错说明见仓库 [`README.md`](README.md)。
