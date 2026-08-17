# ZCode Antigravity v0.4.4-test

本测试版修复 Windows 原生控制中心点击“一键接入 ZCode”后看起来没有反应的问题，并修复 OAuth 访问令牌过期后额度接口持续返回 HTTP 400 的问题。

## 修复

- 点击任一本地操作后立即显示处理中状态，并临时禁用操作按钮，避免重复点击。
- 一键接入期间每 800ms 更新网关与 ZCode 状态，完成后自动刷新模型和额度。
- 后台请求失败时直接显示错误，不再静默忽略。
- Windows Release 后台内嵌发布所需的 OAuth 桌面客户端配置，访问令牌过期后可以正常刷新。
- 实时额度暂时失败时优先保留上次成功缓存，并显示安全的具体错误原因。

## 下载

普通 Windows 用户请下载 `ZCode-Antigravity-Setup-v0.4.4-test.exe`。便携部署或排错可下载 Windows x64 ZIP。

这是第三方测试版。请先阅读仓库 README 中的账号风险、SmartScreen 与校验说明。
