ZCode Antigravity Bridge - Windows x64 test build
=================================================

Purpose
-------
This package lets ZCode and other local agents use Antigravity Gemini or xAI Grok text
models through a loopback-only compatible Provider. The control center switches account,
model, quota, and connector views together. Image/video generation models are not injected.
The Electron 44 window uses React, Tailwind CSS, shadcn-style components, native Acrylic, and the same
light liquid-glass information architecture as the macOS app. Only the white window backdrop is
blurred; navigation, cards, buttons, and text remain high contrast. Its seven pages are Overview,
Accounts, API Proxy, Model Routing, Agent Connectors, Analytics, and Settings.

Important risk
--------------
This is an unofficial third-party bridge to undocumented Google Antigravity interfaces.
Google has disrupted or suspended accounts used through third-party proxies. API access,
Gemini CLI / Code Assist access, or the whole Google account may be affected. Do not test
with your primary Gmail, Workspace, or Google Cloud owner account. A model shown in the
list may still be unavailable to your account or out of quota; the first real request is
the authoritative test.

Requirements
------------
- Windows 10 or Windows 11, x64
- ZCode 3.7.x installed and opened at least once
- A browser and access to Google or xAI login for the provider you choose
- No administrator rights, Node.js, Go, Python, Docker, or firewall rule is required

First test
----------
Recommended: fully exit ZCode from the tray and double-click
ZCode-Antigravity-Setup-v0.6.8-test.exe. This is a native Windows GUI installer: it shows no
terminal, verifies the embedded ZIP plus all three executables, installs only for the current
user, creates Desktop/Start Menu shortcuts, and opens the control center after completion.
Do not use v0.4.0-test on Windows; its Rust client expected baseUrl while the Go Core correctly
emitted baseURL, so startup stopped before the control center could open.
Do not use v0.5.2-test for a fresh Antigravity login; that release was packaged without the
required OAuth desktop configuration. v0.6.8-test also repairs a recorded gateway automatically
when its process exits unexpectedly, while preserving an intentional Stop. It also queues a quota
refresh that overlaps the five-second status poll, so Grok quota appears automatically after the
gateway comes online. Its tray click now opens an independent Acrylic quota widget and never raises
the main control-center window unless the user explicitly asks to open it.

For the single-BAT fallback, fully exit ZCode from the tray and double-click
ZCode-Antigravity-OneClick-v0.6.8-test.bat. It verifies and extracts its embedded package,
then opens the graphical control center without leaving a terminal window. The control center
opens OAuth when needed, writes the verified ZCode Provider directly, and starts ZCode after
successful readback.

For the expanded package:
1. Extract the ZIP completely. Do not run files from inside the ZIP preview.
2. Double-click Verify-Package.bat and confirm all three checks say [OK].
3. In the Windows system tray, right-click ZCode and choose Exit. Closing the ZCode window
   normally leaves ZCode.exe running in the tray; the bridge refuses to edit while it runs.
4. Double-click Setup-and-Start.bat. The Electron control center opens without external Chrome
   or a terminal window.
5. Select Antigravity or Grok / xAI and complete the corresponding browser authorization.
   For Grok, copy the temporary code shown inside the control center into the official
   accounts.x.ai page. The app keeps polling and completes automatically after approval;
   never paste an access token or refresh token into the app.
   The Google OAuth loopback callback remains available for up to 30 minutes. If it expires,
   return to the control center and start a new login instead of reloading the old localhost page.
6. The first model-directory load can take up to about 35 seconds on a poor connection.
7. Reopen ZCode. Select Provider "Antigravity + Grok (Local Bridge)" and choose the desired
   Gemini or Grok text model. The system-tray quota widget remains available after closing the panel.
8. Run Test-Gemini-3.7-Flash.bat once. It sends a small real inference request and writes a
   redacted audit result to %LOCALAPPDATA%\ZCodeAntigravity\last-smoke-test.json.
9. In ZCode, select gemini-3.7-flash and send one small prompt.

Known model availability on 2026-08-15
--------------------------------------
Antigravity desktop 2.8.1 changed the upstream model catalog. With the matching 2.8.1 client
identity, the tested account returned 21 live models including gemini-3.7-flash-low,
gemini-3.7-flash-medium, and gemini-3.7-flash-high. A real gemini-3.7-flash-high request succeeded
and returned response model gemini-3.7-flash with the exact output ZCODE_SMOKE_OK. This bridge does
not alias Gemini 3.6 or another model to Gemini 3.7.

Verified Gemini 3.7 multimodal boundary on 2026-08-15
------------------------------------------------------
- Text: passed through the ZCode Anthropic Provider path (ZCODE_SMOKE_OK).
- Image understanding: passed; Gemini read the Antigravity screenshot, selected model, and
  Low/Medium/High effort levels correctly.
- Video understanding: passed after the bridge translated base64 video blocks to Gemini
  inlineData; a three-second red -> green -> blue MP4 was identified in the correct order.
- Model-declared inputs: text, image, audio, video. Model-declared output: text only.
- Image generation: not supported by gemini-3.7-flash-high. Asking for TEXT+IMAGE returned
  text/SVG and zero image data parts. Use a dedicated image-output model such as
  gemini-3.1-flash-image when the account has quota; do not treat SVG text as generated raster data.

Ports
-----
- Local API default: 127.0.0.1:18080
- If occupied by another program: automatically scans 18081 through 18180
- OAuth callback default: 127.0.0.1:51121
- If occupied: automatically scans 51122 through 51221
- The chosen API port is saved and the ZCode Provider base URL is updated automatically.
- Neither the API nor the patched OAuth callback listens on LAN interfaces.

Files and privacy
-----------------
- Google/xAI access/refresh tokens, the random local API key, logs, and runtime state are stored in:
    %LOCALAPPDATA%\ZCodeAntigravity
- On Windows, OAuth access/refresh token fields are encrypted with current-user DPAPI.
- They are NOT included in this ZIP and are not written into ZCode credentials.json.
- ZCode config.json contains only the random local gateway key, not Google tokens.
- The single BAT embeds program files only; it contains no OAuth token, account JSON, local
  API key, logs, ZCode config, or runtime state.
- Before every ZCode change, the original bytes are backed up under:
    <ZCode data base>\.zcode\v2\backups\zcode-antigravity
- Detailed request-body logging is disabled. The console log is:
    %LOCALAPPDATA%\ZCodeAntigravity\logs\gateway-console.log
- The quota cache is redacted and stores only display-ready values under:
    %LOCALAPPDATA%\ZCodeAntigravity\quota-cache.json
  It does not contain a complete email, project ID, access token, or refresh token.
- Token performance is stored as sanitized provider/model/token/timing samples under:
    %LOCALAPPDATA%\ZCodeAntigravity\usage-metrics.json
  It does not store prompt text, model replies, API keys, OAuth tokens, or complete account data.

Graphical control center and quota
----------------------------------
- Desktop/Start Menu shortcut: ZCode Antigravity 控制中心
- The control center is an Electron 44 Windows GUI using React, Tailwind CSS, and shadcn-style
  components. It starts the audited Go manager as a hidden child and loads only the packaged
  renderer through a sandboxed, context-isolated preload bridge.
- The main window and taskbar quota widget share a liquid-glass visual language. The client uses
  one native DWM Acrylic backdrop with near-solid white content surfaces. It does not capture
  the desktop, so dragging and resizing do not wait for background resampling.
- Electron mode disables renderer-side ambient orbs, full-window blur, and page-entry animation;
  scrolling cards remain high contrast without stacking WebView blur filters.
- Seven pages expose redacted account state, OpenAI/Anthropic/Gemini endpoints, routing and
  session affinity, retry limits, Agent connectors, local Token analytics, refresh cadence,
  quota warning threshold, and UI settings.
- It uses Segoe UI Variable / Microsoft YaHei, Chromium DPI scaling, a bundled icon, and a
  responsive CSS layout for mixed DPI and different desktop resolutions.
- It shows optional TUN, automatically detected v2rayN/Windows proxy status, bridge, ZCode, provider accounts, models, quota, and Agent connectors.
- When the installer opens it with --auto-setup, the native client immediately begins the same
  one-click setup operation; a separate manual click is not required after a successful install.
- Closing the native window leaves a system-tray widget running. Single-click opens only an
  independent Acrylic summary with provider switching, five-hour/week quota, reset times, latest
  output tokens, and token/s. The main window stays at its existing z-order. Use the widget button
  or the tray context menu only when the full control center is wanted.
- Gemini weekly and five-hour remaining quota use exact percentages, reset times, and a native
  progress display. AI credit balance is shown separately so it is not confused with model quota.
- Connection state remains responsive while live quota calls run on a five-minute schedule.
  Manual refresh, provider switching, and a completed local operation refresh quota immediately.
- The quota dashboard uses account summary cards, grouped color-coded bars, reset time, last
  refresh time, and preserves the last successful display when a later refresh fails.
- Successful model responses display protocol-reported output/reasoning Token and token/s. When
  TTFT exists, speed is output Token divided by the post-first-byte generation duration; otherwise
  the UI explicitly labels output Token divided by full-call duration as effective throughput.
- Antigravity and Grok keep separate quota and Token caches. A slow Grok refresh cannot display
  Gemini usage, and switching back restores the last Antigravity values immediately.
- Grok uses the official Grok Build billing response for shared weekly/monthly usage, reset time,
  pay-as-you-go limits, and Extra Usage Credits. It never estimates account quota from local tokens.
- DeepSeek Harness, Grok Build, Codex, Claude Code, Gemini CLI, Qwen Code, Kimi Code, and OpenCode
  have one-click connectors. Each backs up existing files and merges only the managed Provider;
  generic client cards remain copy-only.
- CSS hover, focus, progress, refresh, and operation feedback are GPU-composited. Expensive ambient
  animations and transitions pause during native window dragging to avoid white flashes and jank.
- Quota is read through the already authenticated local bridge. Only a random-key-protected
  loopback management route is enabled; remote management and the web control panel stay disabled.
- If Antigravity temporarily cannot refresh, the last redacted result is shown with a stale badge
  instead of presenting cached data as current.

Available scripts
-----------------
- ZCode-Antigravity-ControlCenter.exe  Electron setup, status, actions, quota, and tray UI
- resources/app.asar                  Packaged local renderer and restricted preload bridge
- LICENSE.electron.txt                Electron MIT license
- LICENSES.chromium.html              Chromium third-party license notices
- CLIProxyAPI-v7.2.132-zcode.patch    Sanitized replay patch for the pinned upstream source
- sanitize_upstream_oauth.go          Preprocess clean upstream before applying the replay patch
- Run-Menu.bat                  Interactive menu
- Setup-and-Start.bat           Open the GUI and start first-time setup
- Login-Antigravity.bat         Add or refresh an Antigravity account
- Login-Grok.bat                Add or refresh a Grok / xAI account
- Start-ZCode-Antigravity.bat   Start/reuse the bridge and sync models
- Sync-ZCode.bat                Refresh the ZCode model list
- Status.bat                    Health, port, account-file count, and model list
- Test-Gemini-3.6-Flash.bat     Verified 3.6 regression/fallback inference
- Test-Gemini-3.7-Flash.bat     Current verified 3.7 High inference and audit artifact
- Test-Grok-Build.bat           Small grok-build-0.1 inference and audit artifact
- Doctor.bat                    Static local checks
- Stop-ZCode-Antigravity.bat    Stop only the process PID/path started by this bridge
- Remove-ZCode-Provider.bat     Remove only the managed Provider from ZCode

Installer formats
-----------------
- ZCode-Antigravity-Setup-v0.6.8-test.exe: recommended no-terminal current-user installer.
- ZCode-Antigravity-OneClick-v0.6.8-test.bat: fallback single-file installer.
- ZCode-Antigravity-Windows-x64-0.6.8-test.zip: manually verifiable expanded package.
- The EXE installer is custom-built and unsigned. It does not require administrator rights or
  7-Zip on the target computer; Windows SmartScreen may still require manual confirmation.

Proxy
-----
An empty packaged proxyURL enables automatic mode and does not require TUN:
  "proxyURL": ""
The Core first reads the enabled Windows user proxy, then probes v2rayN SOCKS5/mixed port 10808 and
legacy HTTP port 10809 when v2rayN is running; it falls back to direct networking. A manual
http/https/socks5 proxyURL has priority. ZCODE_ANTIGRAVITY_PROXY_PORT pins a verified local port.

Troubleshooting
---------------
- Reconnecting / upstream timeout: inspect the Network output card. If it shows direct, confirm
  v2rayN is running and its mixed/HTTP loopback port is listening. TUN remains optional.
- "ZCode.exe is still running": a bridge restart is allowed when the Provider is already
  identical, but any required config write is still blocked. Exit ZCode from the tray, then sync.
- "config.json not found": open ZCode once, exit it fully, then run setup again.
- ZCode opens as a solid gray/blank window: first back up .zcode\v2\setting.json, fully exit
  the verified ZCode process tree, set desktopChromiumHardwareAccelerationEnabled to false,
  and isolate only AppData\Roaming\ZCode\session\GPUCache. Do not delete the whole session
  directory because it contains unrelated browser/session state.
- Port occupied: no manual change is needed unless every configured port is occupied.
- "The process cannot access ... cli-proxy-api.exe": use 0.2.4-test or newer. The installer now
  verifies into a temporary staging directory and stops the recorded gateway before deployment,
  so rerunning the same BAT no longer tries to overwrite a running backend.
- "ParserError: UnexpectedToken" with garbled Chinese text: do not use the 0.2.6-test EXE.
  Version 0.2.8-test writes the embedded script with a UTF-8 BOM for Windows PowerShell 5.1.
- 401 / no models: select the correct provider, run Login-Antigravity.bat or Login-Grok.bat,
  then run Start-ZCode-Antigravity.bat.
- Quota card HTTP 403: v0.4.2 retries without project metadata and then uses per-model quota fallback.
  If it still fails, the account or token does not have access to any current quota endpoint.
- Model request 403 / 429 / unavailable: account entitlement, verification, risk control, or quota is upstream.
- Missing/404 for gemini-3.7-flash-high: verify Antigravity desktop is 2.8.1 or newer, restart this
  bridge so its client-version refresh runs, then check the live account catalog and entitlement.
- "project id unavailable": onboarding did not yield a usable project. The account may be
  ineligible or restricted; use another dedicated test account instead of bypassing controls.
- Windows SmartScreen: these custom Electron/Go binaries are not code-signed. Verify checksums first,
  then use More info -> Run anyway only if the hashes pass.
- Stopping does not delete or revoke Google tokens. Removing the Provider also does not revoke
  Google authorization. Revoke access from your Google account separately if required.

Security boundaries
-------------------
- API server: loopback only
- OAuth callback: patched to IPv4 loopback only
- OAuth authorization code flow: state validation, Host validation, and PKCE S256
- Local API: random per-user key
- Antigravity token files: current-user DPAPI and atomic replacement on Windows
- Management API: random-key protected and loopback only; remote management disabled
- Management control panel: disabled
- Paid Antigravity credits fallback: disabled
- Remote model-registry/plugins: disabled; Antigravity client-version refresh: enabled
- Existing ZCode Provider collision: never overwritten
- Broken/non-object ZCode JSON: never overwritten

Gemini 3.7 reasoning selector
-----------------------------
The managed ZCode model entry exposes the same Low, Medium, and High reasoning choices as the
Antigravity desktop selector. The selected value is sent as Anthropic adaptive-thinking effort
and translated by the bridge to Gemini generationConfig.thinkingConfig.thinkingLevel.
The client-visible model IDs intentionally omit the redundant -high suffix; the local gateway
maps them to the exact upstream High catalog entries before inference and maps response IDs back.

ZCode model selection
---------------------
Antigravity exposes gemini-3.7-flash and gemini-3.6-flash, mapped to their exact upstream High
entries, plus gemini-web-search (Gemini Web Search (Google)) for native Google Search with source
citations. Use the dedicated search model for current web information; ordinary Gemini 3.7/3.6
requests remain normal chat/coding requests. Grok exposes the text models returned for the logged-in xAI account. Imagine image
and video models and unrelated providers are excluded. Missing required Gemini or Grok text models
cause a clear setup error instead of silently substituting another model.

Build versions
--------------
- ZCode Antigravity Bridge: 0.6.8-test
- Control center: Electron 44.0.0, Chromium, React 19.2.8, Tailwind CSS 4.3.3
- CLIProxyAPI base: v7.2.132, commit 78f0c4079e3e6273d65d03b5549cffc898703264
- Local build: 7.2.132-zcode.13

Read THIRD-PARTY-NOTICES.txt, WEB-DEPENDENCIES.txt, LICENSE.electron.txt,
LICENSES.chromium.html, and LICENSE-CLIProxyAPI.txt for upstream details.
