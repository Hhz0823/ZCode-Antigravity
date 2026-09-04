# ZCode Antigravity Bridge

Cross-platform local bridge that exposes Antigravity Gemini, Google-account Claude, and xAI Grok accounts to
ZCode and other coding agents. Release packaging supports Windows x64 and macOS Universal
(Apple Silicon plus Intel).

The manager provides Windows DPAPI or macOS Keychain-backed per-user credential storage, random local API
authentication, API and OAuth callback port scanning, ZCode data-directory resolution,
bounded config backups/logs, atomic Windows replacement, model synchronization, real-model
smoke testing, image/audio/video request translation, safe no-op reuse while ZCode is running,
PID/path-safe stop, managed-provider removal, a Gemini-default selector with opt-in Google Claude and Grok text models,
an explicit Gemini Web Search model with grounded Google Search citations,
Grok billing, protocol-derived output-token and token/s metrics, detailed taskbar/menu-bar quota
menus, backed-up one-click configuration for eight Agent/CLI clients, automatic v2rayN/Windows
system-proxy discovery without TUN, a hidden native-host API, SwiftUI/AppKit macOS UI, Electron 44 +
React/Tailwind Windows UI, an authenticated manager API for accounts/protocols/routing/retry/UI settings,
Codex U-inspired liquid-glass native dashboards, and SHA-256 verified Windows installers.
Both native clients check the latest stable GitHub Release after startup and every six hours,
support an opt-in automatic install mode, verify the exact platform asset against GitHub's SHA-256
digest, and restart into a post-update gateway synchronization.

Build and package the macOS Universal App:

```bash
ANTIGRAVITY_OAUTH_CLIENT_ID='<your-client-id>' \
ANTIGRAVITY_OAUTH_CLIENT_SECRET='<your-client-secret>' \
./packaging/macos/Build-Universal.sh
```

The macOS build compiles a native SwiftUI front end plus Universal Go Core/backend binaries and
ships as an ad-hoc-signed `.app`. Public source does not contain an OAuth desktop client identity;
release packaging refuses to continue without an injected or verified configuration. A local-only
development build that intentionally supplies credentials at runtime must explicitly set
`ALLOW_RUNTIME_OAUTH_CONFIG=1`.

Build the manager:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath -ldflags='-s -w' \
  -o ZCode-Antigravity.exe ./cmd/zcode-antigravity
```

Build the Electron Windows control center from macOS or Windows:

```bash
cd native/windows/ui
npm ci
npm run test:electron
npm run package:windows
```

On macOS with MinGW installed, `./packaging/windows/Build-Windows-Cross.sh` produces the
expanded ZIP, Electron GUI installer, and single-BAT fallback in one flow.

The backend is CLIProxyAPI v7.2.132 with
[`docs/CLIProxyAPI-v7.2.132-zcode.patch`](docs/CLIProxyAPI-v7.2.132-zcode.patch).
Replay the patch only after running `go run tools/sanitize_upstream_oauth.go <upstream-dir>`;
this keeps removed upstream OAuth literals out of the redistributable patch.
See the Windows package README for the risk boundary and test workflow.
The quota-card reference boundary is documented in
[`docs/ANTIGRAVITY-MANAGER-REFERENCE.md`](docs/ANTIGRAVITY-MANAGER-REFERENCE.md); the taskbar and
throughput reference boundary is documented in [`docs/CODEXU-REFERENCE.md`](docs/CODEXU-REFERENCE.md).

Build the single-BAT installer from a verified expanded package:

```powershell
.\packaging\windows\Build-Single-Bat.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-OneClick-v1.0.3-test.bat
```

Build the native no-console EXE installer from the same verified package:

```powershell
.\packaging\windows\Build-Exe-Installer.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-Setup-v1.0.3-test.exe
```
