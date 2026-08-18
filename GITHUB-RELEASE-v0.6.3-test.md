# v0.6.3-test：Windows Grok 额度自动恢复修复

本测试版修复 Windows 控制中心中 Grok 已登录、网关也在线，但额度卡仍停留在
“本地网关尚未运行”的问题。

## 修复内容

- 5 秒状态轮询与手动刷新、提供商切换或“一键接入”完成后的额度刷新重叠时，
  强制刷新不再被静默丢弃，而是合并排队并在当前请求结束后立即执行。
- 检测到网关已从离线恢复，但额度卡仍带有旧的离线提示时，最迟在下一次状态轮询
  自动重新读取额度。
- 保留额度刷新失败隔离：单次 xAI billing 请求失败不会清空账号、模型、网关或
  Agent 接入状态。

## Windows 实机验证

- xAI Grok Build 原始额度接口返回 HTTP 200 和实时共享额度。
- `v0.6.3-test` 已在 Windows 11 x64 目标机安装，账号和本地密钥均保留。
- 网关启动并切换到 Grok 后，控制中心显示实时共享额度、重置时间和 Credits，
  不再显示启动前的离线提示。
- React/Tailwind 生产构建、Go 单测/race/vet、Rust 测试、Windows x64 Release
  交叉编译和发布包完整性检查均通过。

## 下载建议

普通 Windows 用户下载 `ZCode-Antigravity-Setup-v0.6.3-test.exe`。
安装器尚未使用商业代码签名证书，请先校验 `SHA256SUMS-v0.6.3-test.txt`，
并仅在确认哈希一致后处理 SmartScreen 提示。
