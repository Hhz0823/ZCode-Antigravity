# ZCode Antigravity Bridge

Windows x64 local bridge that exposes a pinned, patched CLIProxyAPI Antigravity backend to
ZCode as an Anthropic-compatible custom Provider.

The manager provides Windows DPAPI-protected per-user credential storage, random local API
authentication, API and OAuth callback port scanning, ZCode data-directory resolution,
bounded config backups/logs, atomic Windows replacement, model synchronization, real-model
smoke testing, image/audio/video request translation, safe no-op reuse while ZCode is running,
PID/path-safe stop, managed-provider removal, a strict Gemini 3.7/3.6 Flash allowlist, a
no-console graphical quota control center, and a SHA-256 verified single-BAT Windows installer.

Build the manager:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w' \
  -o ZCode-Antigravity.exe ./cmd/zcode-antigravity
```

Build the Windows GUI control center from the same source:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w -H windowsgui -X main.defaultCommand=gui' \
  -o ZCode-Antigravity-ControlCenter.exe ./cmd/zcode-antigravity
```

The backend is CLIProxyAPI v7.2.132 with
[`docs/CLIProxyAPI-v7.2.132-zcode.patch`](docs/CLIProxyAPI-v7.2.132-zcode.patch).
See the Windows package README for the risk boundary and test workflow.

Build the single-BAT installer from a verified expanded package:

```powershell
.\packaging\windows\Build-Single-Bat.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-OneClick-v0.2.6-test.bat
```

Build the native no-console EXE installer from the same verified package:

```powershell
.\packaging\windows\Build-Exe-Installer.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-Setup-v0.2.6-test.exe
```
