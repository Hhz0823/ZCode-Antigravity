# 修复与验证记录

最新验证日期：2026-08-18（Asia/Shanghai）

## v0.6.4-test 双平台滚动、浅色玻璃与独立额度小组件

- Windows Tauri 2 主界面改为与 macOS SwiftUI 版一致的浅色液态玻璃信息架构：顶部品牌栏、
  横向七页导航、Antigravity / Grok 双栏切换、四个连接状态卡、额度与 Token 指标、右侧接入控制。
- Windows 主窗口和额度小组件继续使用 DWM Acrylic；可滚动内容不再为每张卡重复执行
  `backdrop-filter`，滚动和窗口拖动期间暂停环境光晕与非必要 transition，保留单一系统玻璃层。
- macOS 保留主窗口唯一 `NSVisualEffectView`，把滚动卡片的嵌套 `Material` 改为轻量半透明填充，
  主内容使用 `LazyVStack`，降低大面积阴影和背景模糊半径。
- Windows 11 x64 实机安装后控制中心 SHA-256 与候选包一致；真实 Grok 账号、共享额度、模型、
  Token 与 Token/s 正常显示，连续向下及向上滚动后界面完整返回顶部。
- 实机将 ZCode 保持前台后单击 Windows 系统托盘图标，只出现右下角额度小组件；主控制中心没有
  被恢复、聚焦或置顶。小组件显示 Grok 95% 共享额度、重置时间、累计输出与当前生成速度。
- Apple Silicon Mac 原生 App 完成连续上下滚动；单击菜单栏额度入口只显示原生 Popover，
  只有明确点击“打开主界面”才激活控制中心。
- React/Tailwind 生产构建、Windows GNU target `cargo check` / Release 交叉编译、SwiftUI 双架构
  类型检查、Windows/macOS ZIP 完整性及可执行文件结构检查通过。

## v0.6.3-test Windows Grok 额度自动恢复修复

- 实机原始接口确认 xAI 登录与 billing 服务均正常：同一 Windows 11 账号的
  `/api/quota?provider=xai` 返回 HTTP 200、非缓存数据和 Grok Build 共享额度，问题不在账号授权。
- 根因确认：5 秒状态轮询与“一键接入”完成后的强制额度刷新偶发重叠；旧前端直接丢弃重叠请求，
  后续普通轮询又因已有同提供商额度对象而不再重取，于是在线状态下仍显示启动前的“网关尚未运行”。
- 前端刷新改为单飞队列：忙碌期间收到的手动、切换或接入后强制刷新会合并并在当前请求结束后立即执行；
  同时检测“网关离线提示 + 当前网关在线”的恢复状态，最迟在下一次 5 秒轮询自动重取额度。
- React/Tailwind 生产构建、Go 测试、Rust 测试、Windows x64 Release 交叉编译与目标机安装/额度展示
  验证通过；实机未导出或记录 xAI token。

## v0.6.2-test Windows Grok 空账号与真透明 Acrylic 修复

- 根因确认：选择 Grok 但尚无 xAI 账号时，额度接口把“尚未登录”作为 HTTP 503 返回；Tauri 的
  5 秒状态刷新又把这个额度错误升级成全局“状态刷新失败”，与正在进行的设备登录提示互相覆盖。
- Antigravity / Grok 未登录现在返回 HTTP 200 的正常空额度状态，界面显示明确登录引导且不再请求
  无账号额度；单个额度刷新失败只把额度卡标记为缓存/警告，不再清空网关、账号和 Agent 状态。
- Windows DWM Acrylic 着色层 alpha 从 164 降至 92；窗口、侧栏、状态条和卡片的多层深蓝蒙版同步
  降低不透明度并提高系统模糊与饱和度，桌面内容由系统合成器透入、模糊，文字与控件保持实色清晰。
- 新增 Grok 零账号回归测试；Go 单元测试、React/Tailwind 生产构建和 Windows x64 Release
  交叉编译通过，ZIP、EXE 与单 BAT 均由同一校验载荷重新生成。
- `v0.6.2-test` 已在 Windows 11 目标机完成同版本安装，安装后控制中心 SHA-256 与候选包一致；
  真实点击 Grok 登录后设备授权完成，账号计数由 0 更新为 1，未把临时验证码或 token 写入验证记录。
- 同机在网关停止状态复测：有账号但无法读取额度时，页面显示“本地网关尚未运行，请点击一键接入”，
  不再出现右下角 `http status: 503`，也不会错误显示“尚未登录”。
- 高对比桌面与多个窗口置于应用后方实测，桌面图标、壁纸和窗口色块会透入主窗口并被连续模糊；
  标题、额度卡、按钮和状态指示保持独立清晰。测试计划任务、截图脚本和展开目录已清理。

## v0.6.1-test Windows 双登录与 Grok 验证码修复

- 根因确认：Grok Build 后端会先输出 `accounts.x.ai` 设备授权地址和短码，再持续轮询；旧 Tauri
  原生宿主在命令结束前把 stdout/stderr 全部留在内存，导致用户在等待期间看不到验证码。
- Go Core 现在增量读取登录输出，只接受 HTTPS 且主机名严格为 `accounts.x.ai` 的授权地址，
  并只接受大写字母、数字和连字符组成的短码；设备码、access token 和 refresh token 不进入 GUI API。
- Tauri 控制中心新增液态玻璃授权弹层、只读可选中验证码框、复制按钮、官方页面重开按钮和实时等待状态；
  Google OAuth 也显示独立等待弹层。交互登录超时由 2 分钟延长为 10 分钟，超时不再误报“操作已完成”。
- Windows 11 实机确认 Grok 操作会打开 xAI 官方设备页，软件与页面显示同一短码并持续轮询；
  因测试机没有已授权 xAI 账号，测试在创建 xAI 凭据前主动终止。
- 同机完成一次 Antigravity Google OAuth：登录进程正常完成，新增一份当前用户凭据且不删除旧凭据。
  随后恢复网关，`/healthz` 返回 200，模型目录返回 13 项，ZCode 仍在运行。
- Go 单测、race detector、vet、CLIProxyAPI `go mod verify`/全量测试、React 生产构建、
  Windows GNU target `cargo check --locked` 与完整 Release 交叉编译通过。

## v0.6.0-test Windows Tauri 2 界面与拖动闪烁修复

- Windows 控制中心从自绘 Rust/Win32 GDI 完整迁移到 Tauri 2.11.5、React 19、Tailwind CSS 4
  与 shadcn 风格组件；Rust 只保留窗口、系统托盘、单实例、本地 API 代理和隐藏子进程边界。
- 透明无边框窗口启用系统 Acrylic。旧版会在移动结束后重新截取桌面并自行高斯模糊，现已完全移除该路径；
  桌面模糊由 DWM/WebView2 持续合成，拖动期间暂停环境光晕和 CSS transition。
- 前端生产构建、Windows GNU target `cargo check --locked`、Release 交叉编译和 PE32+ GUI 子系统检查通过。
- 首次真实 Windows 启动暴露出动态 `WebView2Loader.dll` 必须随 EXE 分发；构建脚本已将 DLL 纳入
  展开包、ZIP、EXE/BAT 安装载荷与 SHA-256 清单。补齐后 Windows 11 实机成功启动并显示真实账号与额度。
- Windows 11 目标机录制 7 秒、198 帧窗口拖动测试；4 fps 联系表逐帧检查没有白屏、黑屏、
  冻结桌面采样或背景撕裂，控制中心进程测试后仍响应。
- 1180×760、1536×864 浏览器开发预览和 Windows 实机均检查了总览、Token 统计、Grok 切换、
  七页导航、5 小时 / 本周额度与响应式布局。

## v0.5.3-test Windows 登录与 ZCode 重连修复

- 实机确认 `v0.5.2-test` 发布后端未携带 Antigravity OAuth 桌面配置；新登录会在授权流程前失败。构建脚本现默认拒绝生成此类发布包，只有注入完整配置或提供已验证后端才允许继续；纯开发构建必须显式选择运行时配置模式。
- `v0.5.3-test` Windows 候选包已确认同时含 OAuth client ID / client secret 配置。使用不会打开真实浏览器的 OAuth 预检启动登录流程并等待回调，结果 `OAUTH_LOGIN_FLOW_STARTED=True`、`OAUTH_CONFIG_ERROR=False`。
- 实机旧状态最初记录已退出的 `0.5.0-test` 临时网关 PID 和路径，`18080` 未监听，ZCode 因此持续重连。启动新控制中心后，自动恢复为当前 `0.5.3-test` 后端并重写状态，ZCode 随即建立连接。
- 为验证不是一次性巧合，测试中强制结束当前网关 PID；控制中心在 22 秒检查窗口内换新 PID 恢复同一端口，随后观测到 32 条 ZCode 到网关的已建立连接。用户主动“停止”会把 PID 清零，不触发自动恢复。
- 真实账号状态检查返回 1 个 Antigravity 账号、2 个可选模型，Provider 为 `http://127.0.0.1:18080`；真实 `gemini-3.7-flash` 冒烟请求精确返回 `ZCODE_SMOKE_OK`。

## v0.5.2-test 原生液态高斯背景与状态栏唤醒修复

- macOS 主窗口使用透明 `NSWindow`、`NSVisualEffectView`、`.behindWindow` 与
  `.underWindowBackground`，不再由不透明渐变伪装玻璃；高对比色带和细线参考窗口实测会透入并被系统模糊。
- Windows Rust 客户端继续启用 DWM Desktop Acrylic，并新增当前窗口后方画面采样、三轮水平/垂直可分离
  高斯近似和深蓝玻璃着色；移动或缩放结束后自动重新采样。
- Windows 11 x64 实机以五色色带和交替黑白细线验证：线条在窗口外锐利，在客户端背景内消失为连续模糊色带；
  原生卡片、额度进度、Token/s 与文字保持清晰，没有使用整窗低透明度冒充模糊。

- macOS 状态栏左键单击现在先取消隐藏或恢复最小化的主窗口，激活应用并把控制中心置于最前层，
  随后显示额度小组件；Dock 重新打开也复用同一唤醒路径。
- macOS 自动化实测先把应用设为完全隐藏，再点击 `ZCode 额度` 状态项，结果进程恢复为
  `visible=true`、`frontmost=true`，控制中心窗口重新出现。
- Windows 任务栏通知区域左键单击现在恢复并置顶主窗口，同时显示额度小组件；双击以及小组件内
  “打开控制中心”使用同一恢复函数。
- Windows 11 目标机实测先用 `SW_HIDE` 隐藏主窗口，再发送托盘左键回调；结果
  `MainVisibleAfterClick=True` 且 `MainMinimizedAfterClick=False`。
- Swift arm64 / x86_64 类型检查、Rust Windows x64 离线检查与 Release 交叉编译、Go 单测、
  race detector 和 vet 均通过；测试机临时进程、计划任务、脚本和压缩包已清理。

## v0.4.4-test Windows 一键接入反馈与额度刷新修复

- 点击“一键接入 ZCode”后立即显示处理中状态、禁用重复操作，并以 800ms 间隔刷新后台状态。
- 后台拒绝请求时不再静默丢弃错误，而是在原生窗口中显示明确错误。
- Release 后台注入与登录时相同的 OAuth 桌面客户端配置，实机验证过期访问令牌可刷新。
- 实时额度临时失败时保留上次成功缓存，错误详情不再只显示笼统的 HTTP 状态码。


## v0.4.3-test Windows 原生界面恢复

- 用户实机截图确认 `v0.4.2-test` 功能虽可用，但界面退化为灰色 Win32 默认控件，标题字号在
  高 DPI 下异常放大，不再符合此前蓝白控制中心视觉。
- Windows Rust 客户端现使用 owner-draw 按钮、深蓝侧栏、蓝白状态卡、浅色网格背景、原生进度条与
  Microsoft YaHei UI；没有退回 Chrome/WebView，也没有改变本地安全 API 架构。
- 字体高度改为真正的 point-to-DPI 换算；窗口按所在显示器的实际 DPI 与工作区创建，
  `WM_DPICHANGED` 会重建字体并重新布局。
- 修复了原生控件重绘回调在持有全局状态锁时可能造成的 UI 线程死锁；刷新数据和移动控件现在先复制
  所需状态再调用 Win32 UI API。
- 在用户 Windows 11 x64 实机、144 DPI（150%）显示器上完成验证：窗口物理尺寸
  `1702 × 1136`，四个状态卡、额度卡和七个操作按钮完整可见；真实 Google AI Pro 额度成功刷新，
  窗口保持响应。

## v0.4.2-test Antigravity 额度 HTTP 403 修复

- 用户 Windows 实机截图确认 Google OAuth 已显示 `Login successful`，控制中心也识别到账号与
  `Google AI Pro`，但 `retrieveUserQuotaSummary` 返回 HTTP 403；因此登录与 token 持久化不是故障点。
- 额度读取现在依次尝试 sandbox、daily 与 production 端点；携带项目 ID 返回 403 时，自动用空请求体
  在同一端点重试，覆盖当前上游对项目字段权限不一致的行为。
- 完整汇总仍被拒绝时，使用 `fetchAvailableModels` 读取 Gemini 逐模型剩余额度，并在状态文字中明确
  标为回退数据，不把它冒充完整的每周 / 5 小时汇总。
- 新增两项本机可执行回归：403 后无项目重试，以及三个汇总端点拒绝后逐模型额度降级。
- 构建机无法代替目标 Windows 账号进行真实上游刷新；发布后仍需用户点击“刷新”确认该账号实际返回值。

## v0.4.1-test Windows 启动修复

- 用户实机截图确认 v0.4.0 Rust 客户端启动时报告 `missing field baseUrl`。根因是 Go
  `native-host` 的协议字段为 `baseURL`，而 Serde 默认把 Rust `base_url` 映射为 `baseUrl`。
- `NativeConnection` 与 `ConnectorResponse` 现显式使用规范字段 `baseURL`，并兼容读取旧式
  `baseUrl`；没有降低随机 loopback 会话密钥边界。
- 协议结构已拆到可在 macOS 构建机执行的 Rust library，两个真实 Go JSON 形状回归测试通过，
  防止连接阶段或 Agent 配置阶段再次出现同类 acronym 映射错误。

## v0.4.0-test 原生客户端验证

- macOS 用户界面已改为 SwiftUI + AppKit；`LSUIElement=false`、App Icon、Dock 注册和
  菜单栏额度组件通过本机启动检查，普通启动未新建 Chrome 或 Terminal 进程。
- SwiftUI 主程序、Go Core 与 CLIProxyAPI 后端均为 arm64 + x86_64 Universal Mach-O；
  `codesign --verify --deep --strict`、ZIP CRC 和包内逐文件 SHA-256 通过。
- SwiftUI App 启动后成功拉起 `ZCode-Antigravity-Core native-host`，只监听
  `127.0.0.1:18200-18250`；App 退出后子进程和端口均释放。
- Windows 用户界面已改为 Rust 2024 + Win32；rustfmt、Clippy `-D warnings`、Windows target
  no-run test 和 x86_64 GNU Release 交叉编译通过。
- Rust 控制中心为 PE32+ Windows GUI 子系统并包含图标资源、comctl32、shell32 和 user32；
  启用 Per-Monitor DPI V2、ClearType 与 DPI 变化重排。
- Windows EXE 安装器与单 BAT 共用同一 SHA-256 载荷；内嵌 ZIP、三个 EXE 哈希、逐文件清单、
  CRLF 和 PowerShell 5.1 UTF-8 BOM 均通过反向提取验证。
- Go Core `go test ./...`、`go test -race ./...` 和 `go vet ./...` 通过；CLIProxyAPI
  `go mod verify` 与完整测试通过。CLIProxyAPI 上游树仍有 3 处既有 `go vet` 命名/取消函数告警，
  不属于本次原生客户端改动。
- 尚未在 Windows 10/11 实机完成混合 DPI、安装/重装和 OAuth 双击回归；Intel Mac 仍只有
  交叉编译与结构验证，不宣称实机通过。

## macOS v0.2.7-test 新增验证

- 管理器 macOS 平台实现：`open`、Chrome/Edge/Brave App 模式、`ZCode.app` 启动、
  运行中 `ZCode` 检测、`utun` 检测及基于 `lsof` 可执行文件真实路径的安全停止。
- macOS Antigravity token 存储：AES-256-GCM 随机 nonce、篡改拒绝、旧明文迁移和
  FileTokenStore 写后读回测试通过；随机 256-bit 主密钥由登录钥匙串保存。
- Apple Silicon 本机管理器测试、CLIProxyAPI 相关包测试及 `go mod verify` 通过。
- 管理器和后端均构建为 `x86_64 + arm64` Universal Mach-O；`lipo -info` 与 `file` 复核通过。
- `.app` 使用 ad-hoc 临时签名，`codesign --verify --deep --strict` 通过；未宣称具有
  Apple Developer ID 签名或 Apple 公证。
- 包内逐文件 SHA-256、ZIP CRC、隔离 `doctor`、版本输出和 `/Applications/ZCode.app`
  识别通过；检查过程未读取或修改用户现有 ZCode 配置内容。
- Intel 切片仅完成交叉编译、Mach-O、签名和归档结构检查，没有 Intel 实机启动证据。

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

历史 `v0.2.6-test` 验证时未执行管理器 Race；`v0.4.0-test` 已在当前构建机通过 `go test -race ./...`。

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
