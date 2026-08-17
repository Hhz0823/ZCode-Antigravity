# ZCode Antigravity v0.4.3-test

Windows 原生界面恢复版本。保留 Rust + Win32 原生客户端和 `v0.4.2-test` 的额度修复，恢复此前的
深蓝侧栏、蓝白卡片式控制中心，并完成 150% DPI 实机回归。

## 修复内容

- 恢复旧版控制中心视觉：深蓝侧栏、提供商标签、四个状态卡、额度卡和操作卡。
- 使用 Microsoft YaHei UI 与正确的 point-to-DPI 字体换算，修复高 DPI 下标题和正文异常放大。
- 按窗口所在显示器的真实 DPI 和工作区计算初始尺寸；切换显示器时重建字体并重新布局。
- 使用原生 owner-draw 按钮和蓝色进度状态，不再显示 Win32 默认灰色控件。
- 修复额度刷新与原生控件重绘交叉时可能造成的界面无响应。
- 保留 HTTP 403 无项目字段重试、逐模型额度回退、Grok/xAI、Agent 配置和任务栏额度小组件。

## 实机验证

- Windows 11 Pro x64。
- 144 DPI（150%）显示器完整窗口截图通过。
- 四个状态卡、额度区域和七个操作按钮均在窗口内可见。
- 真实 Google AI Pro 账号额度刷新通过；窗口保持响应。
- Rust `rustfmt`、Clippy `-D warnings`、Windows x64 no-run 测试和 release 交叉编译通过。

## 下载

普通 Windows 用户下载 `ZCode-Antigravity-Setup-v0.4.3-test.exe`。安装前请完全退出旧控制中心和
任务栏小组件。macOS 当前稳定测试包仍为 `v0.4.2-test`。

当前 Windows EXE 未使用商业代码签名证书，首次运行可能出现 SmartScreen 提示；请先按
`SHA256SUMS-Windows.txt` 校验文件。
