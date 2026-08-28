# ZCode Antigravity v0.6.11-test

这个版本把默认体验收敛为 **Gemini-only**，并加入新的原创黄色袋鼠 + 渐变四芒星图标。

## 主要变化

- 默认只向 ZCode 与 Agent 暴露 3 个 Gemini 模型：
  - `gemini-3.7-flash`
  - `gemini-3.6-flash`
  - `gemini-web-search`
- Grok 和其他 AI 文本模型默认关闭；需要时进入“设置”，打开对应开关，再退出 ZCode 并点击“应用模型开关”。
- Grok 关闭时，主界面、状态栏/任务栏小组件与接入控制不会显示 Grok 入口，后端也会拒绝绕过界面的 Grok 切换和登录。
- 关闭模型开关只调整 ZCode / Agent 的模型清单，不删除已经登录的账号、额度缓存或聊天记录。
- 新图标同时应用于 macOS Dock/窗口、Windows EXE/任务栏/窗口与 favicon。

## 升级与使用

1. 完全退出 ZCode。
2. 安装或替换为 0.6.11。
3. 默认直接登录 Antigravity，并点击“一键接入 ZCode”。
4. 如需 Grok、Claude/GPT 等其他文本模型，进入“设置”开启相应开关，再点击“应用模型开关”。
5. 重新打开 ZCode，在 `Google` Provider 下选择模型。

## 验证

- macOS 0.6.11 实机安装后，两个扩展模型开关默认关闭，主界面不显示 Grok，网关 HTTP 200。
- ZCode 实际配置中的 `Google` Provider 只包含 3 个 Gemini 模型。
- Go Core、CLIProxyAPI、Electron、Swift 与双平台发布包结构/哈希检查通过。
- Windows 本轮完成 x64 交叉构建与静态包验证；目标机 SSH 服务在 banner exchange 阶段超时，未宣称 Windows 实机运行通过。

## 下载建议

- macOS：`ZCode-Antigravity-macOS-Universal-v0.6.11-test.zip`
- Windows：`ZCode-Antigravity-Setup-v0.6.11-test.exe`
- Windows 便携包：`ZCode-Antigravity-Windows-x64-0.6.11-test.zip`
- Windows 单文件包：`ZCode-Antigravity-OneClick-v0.6.11-test.bat`
- 源码：`ZCode-Antigravity-Source-v0.6.11-test.zip`

当前 Windows 与 macOS 包没有商业代码签名或 Apple 公证。运行前请校验发布页 SHA-256。

> 图标是本项目原创组合标识，不是美团或 Google 官方联合标识；相关名称与商标归各自权利人所有。
