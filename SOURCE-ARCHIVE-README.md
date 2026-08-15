# ZCode Antigravity Bridge 源码与会话归档

归档修复日期：2026-08-15（Asia/Shanghai）

## 目录

- `project/`：管理器、Windows/macOS 脚本、测试、文档和可重放本地补丁。
- `third_party/CLIProxyAPI-7.2.132-patched/`：完整的固定上游源码及本项目补丁。
- `conversation/`：仅保留在本地归档的会话导出；GitHub 发布时由 `.gitignore` 明确排除。
- `SOURCE-SHA256SUMS.txt`：除清单自身与 `.git` 元数据外公开源码文件的 SHA-256。

旧的 `CLIProxyAPI-7.2.131-patched` 归档缺少 19 个上游文件，已从工作区移到可恢复的
临时保留区，不再作为可构建源码交付。

## 上游固定点

- 项目：`router-for-me/CLIProxyAPI`
- Release：`v7.2.132`
- Commit：`78f0c4079e3e6273d65d03b5549cffc898703264`
- 上游 source ZIP SHA-256：`b87fdbbec6d4cc692d6ecbfad2d24eaba932ffb085f4573617c83244aaa3f63f`
- 许可证：MIT，见 `third_party/CLIProxyAPI-7.2.132-patched/LICENSE`

Antigravity 桌面版从 2.2.1 更新到 2.8.1 后，同一账户的实时目录由 18 个模型更新为 21 个，新增
`gemini-3.7-flash-low`、`gemini-3.7-flash-medium` 和 `gemini-3.7-flash-high`。精确 High 请求已
成功，响应模型为 `gemini-3.7-flash`，输出断言为 `ZCODE_SMOKE_OK`。静态 High 条目的输入上下文为
1,048,576 tokens、最大输出 65,536 tokens；本项目不把 3.6 或其他模型伪装成 3.7。

多模态实测中，ZCode Anthropic Provider 路径的图片识别成功；补齐 base64 `audio`/`video`
到 Gemini `inlineData` 的转换后，三秒红→绿→蓝 MP4 也按正确时序识别。3.7 的声明输出仅为
`text`；即使请求 `TEXT+IMAGE` 也只返回文字/SVG，没有图片数据块，因此不能把它当图片生成模型。

## 本地修改

可重放补丁：`project/docs/CLIProxyAPI-v7.2.132-zcode.patch`，已对干净 v7.2.132 源码执行
`git apply --check --whitespace=error-all`。

主要修改包括：

1. OAuth callback 仅绑定 `127.0.0.1/tcp4`，验证 Host、state，并使用 PKCE S256。
2. Windows access/refresh token 使用当前用户 DPAPI，文件采用原子替换；旧明文文件读取时迁移，
   watcher 运行时统一解密后再交给 executor。
3. 凭据读取不再触发隐式网络和写盘；onboarding 项目传播只做有限重试并失败关闭。
4. 可选能力探测 10 秒超时；后台刷新当前 Antigravity 客户端版本并直接选择 Chrome/Edge。
5. 保留 `gemini-3.7-flash-high` 的 `maxOutputTokens`，模型接口返回真实上下文和模态能力。
6. Windows Git 仓库句柄在恢复/GC/提交后及时关闭；极快响应的 TTFT 保留“已测量”语义。
7. 管理器增加跨进程锁、启动失败回收、账号结构校验、日志/备份保留上限、语义写后校验和
   带精确输出断言的真实推理冒烟测试。
8. ZCode 扩展的 base64 音频/视频内容块转换为 Gemini `inlineData`；ZCode 运行时允许完全相同
   Provider 的只读复用，但任何需要写配置的同步仍失败关闭。
9. Anthropic 模型目录额外返回真实 `thinking.levels`；管理器据此为 Gemini 3.7 生成 Low / Medium /
   High 选择器，实际请求转换为 Gemini `generationConfig.thinkingConfig.thinkingLevel`。
10. 管理器把 ZCode 侧模型严格限制为 `gemini-3.7-flash` 和 `gemini-3.6-flash`，通过网关原生
    OAuth 别名分别映射到真实上游 `*-high` 条目；单文件 BAT 内嵌无凭据运行包并校验 SHA-256。
11. 单文件安装器先在隔离临时目录验签解包，再用暂存管理器安全停止状态文件记录的旧网关，最后
    部署到版本目录；重复运行同版本 BAT 不再覆盖仍被进程占用的后端 EXE。
12. 增加 Windows GUI 控制中心：无终端完成接入、登录、启停和 ZCode 打开；额度通过随机密钥保护的
    loopback 管理路由读取，仅缓存脱敏展示字段，并以周额度/五小时额度波浪图显示。
13. 控制中心统一为蓝白遥测视觉体系，并加入统一节奏的页面入场、状态脉冲、操作反馈、百分比计数和
    双层波浪液面动效；系统启用“减少动态效果”时自动关闭非必要动画。
14. 增加原生 Windows GUI EXE 安装器：内嵌完整 ZIP，解包前校验载荷 SHA-256、解包后再次校验三份
    EXE；隐藏运行当前用户部署脚本，保留 TUN/代理、ZCode 运行中和旧 Bridge PID/路径安全门。
15. 增加 macOS 平台进程、浏览器、ZCode.app 与 `utun` 检测；停止网关前用 `lsof` 复核 PID 对应的
    真实可执行文件路径，并提供 Apple Silicon + Intel Universal App 与 `.command` 维护脚本。
16. macOS Antigravity access/refresh token 使用 AES-256-GCM 加密，随机主密钥保存在当前用户登录
    钥匙串；公开源码继续不保存 OAuth 客户端身份，发布维护者可在链接时注入。

## 构建

已验证：Go 1.26.x、Windows x64、macOS Universal、`CGO_ENABLED=0`。

GitHub 源码树不包含上游静态 Google OAuth 客户端凭据。构建的后端在运行 OAuth
登录或 token 刷新前，需从 `ANTIGRAVITY_OAUTH_CLIENT_ID` 和
`ANTIGRAVITY_OAUTH_CLIENT_SECRET` 环境变量读取开发者有权使用的配置。

macOS Universal 发布包可运行 `project/packaging/macos/Build-Universal.sh` 构建。
维护者提供上述两个环境变量时，脚本通过 Go 链接参数注入发布二进制；未提供时，
生成的开发包仍需在运行时从 `.env` 或进程环境读取。

```powershell
cd project
$env:CGO_ENABLED='0'; $env:GOOS='windows'; $env:GOARCH='amd64'
go test ./...
go build -trimpath -ldflags='-s -w' -o ZCode-Antigravity.exe ./cmd/zcode-antigravity
go build -trimpath -ldflags='-s -w -H windowsgui -X main.defaultCommand=gui' `
  -o ZCode-Antigravity-ControlCenter.exe ./cmd/zcode-antigravity
```

```powershell
cd third_party/CLIProxyAPI-7.2.132-patched
go mod verify
$env:CGO_ENABLED='0'; $env:GOOS='windows'; $env:GOARCH='amd64'
go build -buildvcs=false -trimpath `
  -ldflags='-s -w -X main.Version=7.2.132-zcode.10 -X main.Commit=78f0c4079e3e6273d65d03b5549cffc898703264 -X main.BuildDate=<UTC>' `
  -o cli-proxy-api.exe ./cmd/server
```

`CGO_ENABLED=0` 会关闭动态插件；本项目配置明确禁用插件，内建 Antigravity Provider 不依赖它。

## 不包含的内容

本源码归档不包含 EXE/DLL、`.git` 历史、依赖缓存、OAuth token、账号文件、本地 API key、
运行日志和测试包。临时 Windows 测试包位于归档外。

该桥接调用未公开的 Google Antigravity/Code Assist 接口，不是官方 Gemini API。请勿使用主
Gmail、Workspace 管理员或重要 Cloud 所有者账号。账号授权、风控、资格和配额属于外部边界，
程序不会填写密码、绕过验证码或在缺少可用 `project_id` 时保存半成品凭据。
