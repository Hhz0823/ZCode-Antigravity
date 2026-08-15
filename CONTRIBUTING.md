# 参与 ZCode Antigravity Bridge 开发

感谢关注这个项目。请先阅读仓库首页的非官方接口、账号风险和许可边界。

## 仓库结构

- `project/`：ZCode 桥接管理器、Windows GUI、打包脚本和回归测试。
- `third_party/CLIProxyAPI-7.2.132-patched/`：固定的 CLIProxyAPI 上游源码与本项目修改。
- `project/docs/CLIProxyAPI-v7.2.132-zcode.patch`：对上游版本的历史业务补丁。
- `VERIFICATION.md`：已验证能力和未验证边界。

## 开发环境

- Go 1.26.x
- Windows x64 用于最终 GUI、DPAPI 和安装器验收
- 普通 Go 单元测试可在 macOS、Linux 或 Windows 运行

Antigravity OAuth 客户端凭据不存放在仓库中。仅在需要真实 OAuth 流程时，使用自己有权使用的桌面应用配置：

```powershell
$env:ANTIGRAVITY_OAUTH_CLIENT_ID = '<your-client-id>'
$env:ANTIGRAVITY_OAUTH_CLIENT_SECRET = '<your-client-secret>'
```

请勿提交 `.env`、账号 JSON、token、日志、`local-api-key` 或任何本机 ZCode 配置。

## 测试

管理器：

```bash
cd project
go test ./...
```

CLIProxyAPI 后端：

```bash
cd third_party/CLIProxyAPI-7.2.132-patched
go mod verify
go test ./...
```

如果修改 Windows 安装、DPAPI、进程或 GUI 逻辑，请同时按
`project/packaging/windows/TEST-CHECKLIST.txt` 在 Windows 10/11 x64 上完成验收。

## 提交建议

1. 将修改限定在单一明确问题中。
2. 为行为变化增加回归测试。
3. 不伪造模型 ID、能力、额度或实测结论。
4. 不降低 loopback、PKCE、DPAPI、随机本地密钥和配置备份等安全边界。
5. 不把 EXE、ZIP 或单文件 BAT 提交到 Git 历史；它们应作为 GitHub Release 附件发布。
6. Pull Request 中说明测试命令、结果和仍未覆盖的实机边界。

## 上游同步

更新 CLIProxyAPI 固定版本时，请记录新的 tag、commit、上游源码 SHA-256 和许可证，
然后重新验证本地补丁、两个 Go 模块和 Windows 发布包。
