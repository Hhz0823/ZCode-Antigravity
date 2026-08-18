# ZCode Antigravity Bridge

Cross-platform local bridge that exposes pinned Antigravity Gemini and xAI Grok accounts to
ZCode and other coding agents. Release packaging supports Windows x64 and macOS Universal
(Apple Silicon plus Intel).

The manager provides Windows DPAPI or macOS Keychain-backed per-user credential storage, random local API
authentication, API and OAuth callback port scanning, ZCode data-directory resolution,
bounded config backups/logs, atomic Windows replacement, model synchronization, real-model
smoke testing, image/audio/video request translation, safe no-op reuse while ZCode is running,
PID/path-safe stop, managed-provider removal, a strict Gemini plus Grok-text model selector,
Grok billing, protocol-derived output-token and token/s metrics, detailed taskbar/menu-bar quota
menus, Agent connector cards, a hidden native-host API, SwiftUI/AppKit macOS UI, Tauri 2 +
React/Tailwind Windows UI, an authenticated manager API for accounts/protocols/routing/retry/UI settings,
Codex U-inspired liquid-glass native dashboards, and SHA-256 verified Windows installers.

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

Build the Tauri 2 Windows control center with the GNU cross target:

```bash
rustup target add x86_64-pc-windows-gnu
cd native/windows
npm --prefix ui ci
npm --prefix ui run build
cargo check --locked --target x86_64-pc-windows-gnu
cargo build --release --target x86_64-pc-windows-gnu
```

On macOS with MinGW installed, `./packaging/windows/Build-Windows-Cross.sh` produces the
expanded ZIP, Tauri GUI installer, and single-BAT fallback in one flow.

The backend is CLIProxyAPI v7.2.132 with
[`docs/CLIProxyAPI-v7.2.132-zcode.patch`](docs/CLIProxyAPI-v7.2.132-zcode.patch).
See the Windows package README for the risk boundary and test workflow.
The quota-card reference boundary is documented in
[`docs/ANTIGRAVITY-MANAGER-REFERENCE.md`](docs/ANTIGRAVITY-MANAGER-REFERENCE.md); the taskbar and
throughput reference boundary is documented in [`docs/CODEXU-REFERENCE.md`](docs/CODEXU-REFERENCE.md).

Build the single-BAT installer from a verified expanded package:

```powershell
.\packaging\windows\Build-Single-Bat.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-OneClick-v0.6.2-test.bat
```

Build the native no-console EXE installer from the same verified package:

```powershell
.\packaging\windows\Build-Exe-Installer.ps1 `
  -PackageDir C:\path\to\expanded-package `
  -OutputFile C:\path\to\ZCode-Antigravity-Setup-v0.6.2-test.exe
```
