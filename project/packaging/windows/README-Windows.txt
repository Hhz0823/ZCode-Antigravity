ZCode Antigravity Bridge 1.1.0 — Windows x64
============================================

Installation and updates
------------------------
1. Completely exit ZCode from its system-tray menu before installing or syncing.
2. Run ZCode-Antigravity-Setup-v1.1.0.exe. Installation is for the current user.
3. Open the control center, add a Google account if needed, then use One-click ZCode setup.
4. When upgrading, use Repair and resync to apply the new model defaults.

The installer preserves accounts, chats, projects and unrelated ZCode providers. It verifies
its embedded payload and retains the previous version for rollback. Download checksums from
the same GitHub release and compare them before running an unsigned installer.

Alternative packages:
- ZCode-Antigravity-OneClick-v1.1.0.bat: single-file fallback installer.
- ZCode-Antigravity-Windows-x64-1.1.0.zip: portable package with SHA256SUMS.txt.
- Verify-Package.bat: validate the package's core executables.

Network and credentials
-----------------------
The bridge only listens on 127.0.0.1. It automatically discovers v2rayN or the Windows system
proxy; TUN is optional. Without a local proxy it attempts a direct connection. Use settings.json
or the control center for an explicit proxy when necessary. The local API key is generated on
this computer; account tokens are protected with current-user Windows DPAPI.

Data and settings are under %LOCALAPPDATA%\ZCodeAntigravity. Versioned application files are
under app-1.1.0. Do not publish your data directory, auth files, API key, raw configuration or logs.
The distribution includes the OAuth desktop application configuration required for login;
user tokens are never included in release assets.

Gemini defaults and current ZCode
---------------------------------
Only gemini-3.8-flash and gemini-3.7-flash are injected by default. Each clean model ID maps to
its exact upstream High entry. No older model is renamed to impersonate a newer version.

Default context: 384K = 393,216 tokens, capped by a smaller declared upstream input limit.
Default reasoning: High. Current ZCode's enabled switch explicitly requests adaptive thinking
with output_config.effort="high". Disabled requests disabled thinking. Older ZCode clients
retain Low/Medium/High choices with High as the default.

Sync writes both config.json and provider_config.json, backs up existing files before writing,
and preserves unrelated providers and defaults. Exit ZCode before a configuration change.
Managed Gemini 3.6 and older models are removed, including the former Gemini 3.1 web-search alias.
Use your client's search tools for web information. Context quality varies with the task;
384K is the selected default, not a guarantee of perfect retrieval on every workload.

Google Claude / other AI text models and Grok are optional switches. Apply model switches after
changing them. Availability and quotas are determined by the signed-in account. Image/video
creation models are excluded from the text model picker.

Google account verification
----------------------------
The control center opens the official Antigravity client with one click, or opens its official
download page if it is not installed. Sign in with the same Google account and send a message.
If Google asks for identity or age verification, complete it yourself on the official page,
then return and refresh quota. Authorization alone does not guarantee model eligibility.
Generic network, permission and quota errors are not automatically classified as verification.

Control center and taskbar widget
---------------------------------
The Windows interface uses compact navigation, a light neutral background, readable cards and
clear primary actions. New-account guidance is collapsible and expands for verification errors.
Click the tray icon for the quota widget. Click outside it or click the tray icon again to hide.
The widget has transparent rounded edges without a square Acrylic underlay. Open Control Center
from the widget footer or the tray menu. Closing the control-center window hides it to the tray.

The interface shows quota, reset time, output/reasoning tokens and throughput. It does not
record prompt text. Quota normally refreshes every five minutes; status refreshes every five
seconds. The API Proxy page contains network diagnostics and supported protocol addresses.

Maintenance commands
----------------------
ZCode-Antigravity.exe version
ZCode-Antigravity.exe status
ZCode-Antigravity.exe doctor
ZCode-Antigravity.exe sync
ZCode-Antigravity.exe smoke gemini-3.8-flash
ZCode-Antigravity.exe smoke gemini-3.7-flash

A smoke command sends one real, small inference request and consumes the account's quota.
Use supplied maintenance BAT files for the same actions. Source code, release checks and
remaining validation boundaries are documented in the GitHub repository's VERIFICATION.md.

Components and licenses
-----------------------
Bridge: 1.1.0. Electron: 44.0.0. React: 19.2.8. Tailwind CSS: 4.3.3.
Backend: CLIProxyAPI v7.2.132, local build 7.2.132-zcode.14.
Upstream commit: 78f0c4079e3e6273d65d03b5549cffc898703264.
See THIRD-PARTY-NOTICES.txt, WEB-DEPENDENCIES.txt, LICENSE.electron.txt,
LICENSES.chromium.html and LICENSE-CLIProxyAPI.txt.
