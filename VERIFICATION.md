# 修复与验证记录

验证日期：2026-08-15（Asia/Shanghai）

## 已通过

- 管理器 `go test ./...`、Windows x64 构建、`doctor`：通过。
- CLIProxyAPI `go mod verify`、`go test ./...`、Windows x64 构建：通过。
- OAuth PKCE/callback、DPAPI/原子凭据存储、watcher 运行时解密、Antigravity auth、模型注册、
  Gemini 3.7 `maxOutputTokens`、Windows Git corruption recovery、TTFT 和管理器回归：通过。
- `git apply --check --whitespace=error-all project/docs/CLIProxyAPI-v7.2.132-zcode.patch`：通过。
- Chrome Google OAuth：成功；获得可用 project ID，账号文件仅保留 DPAPI 字段，无明文 token。
- 当前 `0.2.3-test` 网关仅监听 `127.0.0.1:18080`；ZCode Provider 已切换为两个干净模型 ID。
- v2rayN：`EnableTun=true`，`xray_tun` 网卡为 `Up` 并持有 `0.0.0.0/0` 路由；桥接运行配置
  固定为 `http://127.0.0.1:10808`，真实请求期间已观测到网关到该端口的 TCP 连接。
- TUN 出网：在显式禁用 HTTP 代理的条件下，Google Cloud Code 域名 TLS 校验通过；根路径返回预期 404，
  证明 TUN 链路可达。显式 v2rayN 代理隧道也返回 CONNECT 200。
- ZCode 灰屏已恢复：备份 `setting.json`，按精确 PID/路径终止冻结的 ZCode 进程树，只隔离
  `AppData\Roaming\ZCode\session\GPUCache`，并把
  `desktopChromiumHardwareAccelerationEnabled` 改为 `false`。重启后窗口响应正常，截图确认侧栏、
  首页、输入框和 Antigravity 模型选择器均正常渲染；新日志无 renderer/GPU/config 错误。
- Gemini 3.7 配置生成新增 `low / medium / high` 思考档位；完整 Anthropic→Antigravity
  翻译回归确认三档分别生成对应的 Gemini `thinkingLevel`。
- 对照真实推理：`gemini-3.6-flash-high` 通过 Anthropic `/v1/messages` 返回成功，响应模型为
  `gemini-3.6-flash`，精确输出断言 `ZCODE_SMOKE_OK` 通过，审计文件已写入运行目录。
- Antigravity 桌面版已更新为 Google 2.8.1；桥接内置版本同步从 2.2.1 更新为 2.8.1，并恢复
  后台客户端版本刷新，不再以 `-skip-antigravity-version-update` 启动。
- 管理器 `0.2.6-test` 写入 ZCode 的模型恰好为 `gemini-3.7-flash`、`gemini-3.6-flash`；网关
  OAuth 别名分别路由到真实 `*-high` 上游条目并强制把响应模型名映射回干净 ID。Claude、GPT、
  agent、图像及其它 Gemini 模型不会写入。允许列表、别名一致性、备份、幂等和运行中拒写测试通过。
- 单文件 BAT 已通过真实 `cmd.exe -> Windows PowerShell` 自解包测试：内嵌 23 个文件，载荷先校验
  SHA-256，解包后管理器、GUI 和后端三个 EXE 再校验，展开目录逐文件哈希与构建包一致且不含账号、
  key、状态或日志。
- 单文件 BAT 正常模式已验证 v2rayN TUN/`127.0.0.1:10808` 预检和 ZCode 运行中安全门；检测到
  ZCode 托盘进程后明确停止，ZCode `config.json` 哈希前后一致。
- `0.2.4-test` 修复同版本重装顺序：载荷先在隔离临时目录验签，暂存管理器随后按状态 PID、实际
  EXE 路径和网关健康检查停止旧进程，最后才部署。隔离回归已先复现与用户截图相同的后端文件占用，
  再确认停止后同版本覆盖成功且管理器/后端哈希一致。
- `0.2.6-test` 包含 Windows GUI 控制中心；PE Subsystem 实测为 2（Windows GUI），双击一键 BAT
  通过隐藏 PowerShell 启动后立即退出，不保留终端窗口。控制中心仅监听 `127.0.0.1:18200-18250`，
  API 使用每次启动随机会话密钥，并设置 CSP、no-store、DENY frame 和 nosniff。
- 额度解析回归使用与 Antigravity 2.8.1 二进制描述一致的 `retrieveUserQuotaSummary` 字段，确认周额度
  99%、五小时额度 100%、重置时间、Google AI Pro 和 AI Credits 映射；缓存断言不含完整邮箱或
  project ID。波浪额度页已在 1200×900 窗口完成实际渲染截图验收。
- 控制中心已统一为蓝白遥测主题；1200×900 实际窗口确认左侧导航、状态条、额度卡和操作栏无横向
  溢出。双层蓝色波浪、额度液面上升、百分比计数、状态脉冲、操作扫光和按钮反馈使用统一 easing，
  `prefers-reduced-motion` 会关闭非必要循环和入场动画，静态主题/动效回归测试通过。
- 原生 Windows GUI EXE 安装器已完成：正常启动只显示标题为 `ZCode Antigravity 安装器` 的原生确认
  对话框，PE Subsystem=2，未创建终端；只读确认界面验收后按精确 PID/路径关闭。`--extract-only`
  解出 23 个文件且逐文件哈希与展开包完全一致；隔离安装根目录首次安装及同版本二次安装均通过，
  三份 EXE 哈希和 `http://127.0.0.1:10808` 设置读回一致。

管理器 `go test -race` 未执行：本机没有 race 构建所需的 GCC。普通测试和 Windows 构建均通过。

## Gemini 3.7 当前结论

- 使用旧客户端标识 2.2.1 时，实时目录为 18 个模型且没有 3.7；更新到 2.8.1 后，同一账户返回
  21 个模型，包含 `gemini-3.7-flash-low`、`gemini-3.7-flash-medium` 和
  `gemini-3.7-flash-high`。
- 已真实请求 `gemini-3.7-flash-high`，HTTP 200，响应模型为 `gemini-3.7-flash`，精确输出
  `ZCODE_SMOKE_OK`。初始 64 token 冒烟预算只返回 `ZCODE_`，提高到 256 后通过；实际响应统计
  110 output tokens，说明 3.7 的隐藏思考预算也计入输出上限。
- 结果证明此前 404/目录缺失是旧客户端版本标识造成的能力门槛，不是模型不存在；本项目仍不做模型别名伪装。

## Windows x64 测试构建

- `ZCode-Antigravity.exe` 0.2.6-test
  - SHA-256：`5c039582dac36eec5c57181e4a647364f55b9e3971615a086379626e6ffc68bb`
- `ZCode-Antigravity-ControlCenter.exe` 0.2.6-test
  - SHA-256：`4aa4f56e0e5b75a9e3f793016176f465e55a8ecddefdac7813515d46f3c66123`
- `cli-proxy-api.exe` 7.2.132-zcode.9
  - SHA-256：`3765e25ae7dcdcfe9bb6027b1cce8468ee9ccfd1ca4f97459300c2ab9b1171f9`
- `ZCode-Antigravity-OneClick-v0.2.6-test.bat`
  - 大小：`37,996,167` bytes
  - SHA-256：`72ea8b6714cdd0436d55fea97ce972ffda9664194a790913639a2fa325fd3c20`
- `ZCode-Antigravity-Windows-x64-0.2.6-test.zip`
  - 大小：`27,757,461` bytes
  - SHA-256：`4cd1767720fa2dddc80f810205dee54da17d93f35de08bd96c8cc53bcdd330df`
- `ZCode-Antigravity-Setup-v0.2.6-test.exe`
  - 大小：`30,221,824` bytes
  - SHA-256：`f7dcfb01d2d7364cec5e7634b643358b779a09df2f70447ba73f355b9d2b7bb7`

二进制只存在于归档外的隔离测试包。后续源码变化必须重新构建，以上哈希不能用于未来构建。

## 尚未宣称完成的边界

- 当前机已切换到 `0.2.3-test`，配置写后读回和 ZCode 重启后读回均只包含
  `gemini-3.6-flash`、`gemini-3.7-flash`，两者都保留 Low / Medium / High 思考选择。
- 已通过 Windows 原生窗口截图目视验证灰屏恢复；没有代替用户在 ZCode 窗口内发送新的模型消息。
- ZCode 使用的本地 Provider URL、认证头、Anthropic 协议和真实 `/v1/messages` 路径均已独立验证；
  可在 ZCode 中选择 `gemini-3.7-flash` 做桌面 UI 手动对话验收。新别名的实时目录加载已通过；
  直接真实推理未在本轮重复，因为 web-access 的专用 Edge CDP 前置端点未启动。OAuth 别名请求与
  响应强制映射的后端离线回归均通过，原始 `*-high` 真实推理证据仍有效。
- 本轮真实额度刷新没有宣称通过：`web-access` 要求的专用 Edge CDP `127.0.0.1:9223` 未监听，且该
  本地技能只允许 Mercado Libre/1688 域名，不能用它绕过限制访问 Google。额度协议、脱敏缓存和
  波浪渲染已用本地模拟上游完整验证；用户在 v2rayN TUN 正常时可直接点控制中心“刷新数据”做实账验收。

## 隐私边界

源码归档不包含 OAuth token、账号 JSON、本地 API key、日志或编译产物。GitHub 源码树进一步
移除了上游静态 Google OAuth 客户端凭据，改为运行时环境变量；示例 key 和测试假 token
仅用于测试，不是用户凭据。
