#!/bin/zsh
set -euo pipefail

script_dir=${0:A:h}
project_dir=${script_dir:h:h}
repo_root=${project_dir:h}
backend_dir="$repo_root/third_party/CLIProxyAPI-7.2.132-patched"
native_dir="$project_dir/native/windows"
release_version=${VERSION:-0.6.4-test}
output_dir=${OUTPUT_DIR:-$repo_root/dist/windows}
package_name="ZCode-Antigravity-Windows-x64-${release_version}"
package_root="$output_dir/$package_name"
archive_path="$output_dir/$package_name.zip"
installer_path="$output_dir/ZCode-Antigravity-Setup-v${release_version}.exe"
single_bat_path="$output_dir/ZCode-Antigravity-OneClick-v${release_version}.bat"
build_root=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/zcode-antigravity-windows.XXXXXX")
trap '/bin/rm -rf "$build_root"' EXIT

for tool in go cargo npm x86_64-w64-mingw32-gcc \
  /usr/bin/base64 /usr/bin/ditto /usr/bin/fold /usr/bin/iconv /usr/bin/perl \
  /usr/bin/sed /usr/bin/shasum /usr/bin/strings /usr/bin/zip; do
  command -v "$tool" >/dev/null
done

if ! /usr/bin/grep -Fq "const version = \"$release_version\"" "$project_dir/cmd/zcode-antigravity/main.go"; then
  print -u2 "VERSION=$release_version 与 Go Core 源码版本不一致"
  exit 1
fi
if ! /usr/bin/grep -Fq "version = \"$release_version\"" "$native_dir/Cargo.toml"; then
  print -u2 "VERSION=$release_version 与 Rust 客户端源码版本不一致"
  exit 1
fi
if ! /usr/bin/grep -Fq "\"version\": \"$release_version\"" "$native_dir/ui/package.json"; then
  print -u2 "VERSION=$release_version 与 Tauri 前端版本不一致"
  exit 1
fi
if [[ -n ${ANTIGRAVITY_OAUTH_CLIENT_ID:-} || -n ${ANTIGRAVITY_OAUTH_CLIENT_SECRET:-} ]]; then
  if [[ -z ${ANTIGRAVITY_OAUTH_CLIENT_ID:-} || -z ${ANTIGRAVITY_OAUTH_CLIENT_SECRET:-} ]]; then
    print -u2 "ANTIGRAVITY_OAUTH_CLIENT_ID 与 ANTIGRAVITY_OAUTH_CLIENT_SECRET 必须同时设置"
    exit 1
  fi
  env_oauth=true
else
  env_oauth=false
fi
oauth_ready=$env_oauth

if [[ -n ${PREBUILT_BACKEND_WINDOWS:-} ]]; then
  if [[ ! -f $PREBUILT_BACKEND_WINDOWS ]]; then
    print -u2 "找不到 PREBUILT_BACKEND_WINDOWS=$PREBUILT_BACKEND_WINDOWS"
    exit 1
  fi
  if /usr/bin/strings "$PREBUILT_BACKEND_WINDOWS" | /usr/bin/grep -E '[0-9]+-[A-Za-z0-9_-]+\.apps\.googleusercontent\.com' >/dev/null && \
    /usr/bin/strings "$PREBUILT_BACKEND_WINDOWS" | /usr/bin/grep -E 'GOCSPX-[A-Za-z0-9_-]{20,}' >/dev/null; then
    oauth_ready=true
  fi
fi
if [[ $oauth_ready != true && ${ALLOW_RUNTIME_OAUTH_CONFIG:-0} != 1 ]]; then
  print -u2 '拒绝构建无法登录 Antigravity 的发布包：请注入 OAuth 桌面配置，或提供已验证的 PREBUILT_BACKEND_WINDOWS。'
  print -u2 '仅开发环境可显式设置 ALLOW_RUNTIME_OAUTH_CONFIG=1，改为运行时环境变量配置。'
  exit 1
fi

sha256_upper() {
  /usr/bin/shasum -a 256 "$1" | /usr/bin/awk '{print toupper($1)}'
}

/bin/rm -rf "$package_root" "$archive_path" "$installer_path" "$single_bat_path"
/bin/mkdir -p "$package_root/backend" "$output_dir"

(
  cd "$project_dir"
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' \
    -o "$package_root/ZCode-Antigravity.exe" ./cmd/zcode-antigravity
)

backend_ldflags="-s -w -X main.Version=7.2.132-zcode.12"
if [[ $env_oauth == true ]]; then
  backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientID=$ANTIGRAVITY_OAUTH_CLIENT_ID"
  backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientSecret=$ANTIGRAVITY_OAUTH_CLIENT_SECRET"
fi
if [[ -n ${PREBUILT_BACKEND_WINDOWS:-} ]]; then
  /bin/cp "$PREBUILT_BACKEND_WINDOWS" "$package_root/backend/cli-proxy-api.exe"
else
  (
    cd "$backend_dir"
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$backend_ldflags" \
      -o "$package_root/backend/cli-proxy-api.exe" ./cmd/server
  )
fi

(
  cd "$native_dir/ui"
  npm ci --ignore-scripts
  npm run build
)
(
  cd "$native_dir"
  cargo build --release --locked --target x86_64-pc-windows-gnu
)
/bin/cp "$native_dir/target/x86_64-pc-windows-gnu/release/zcode-antigravity-tauri.exe" \
  "$package_root/ZCode-Antigravity-ControlCenter.exe"
/bin/cp "$native_dir/target/x86_64-pc-windows-gnu/release/WebView2Loader.dll" \
  "$package_root/WebView2Loader.dll"

for payload in README-Windows.txt TEST-CHECKLIST.txt settings.json THIRD-PARTY-NOTICES.txt \
  LICENSE-CLIProxyAPI.txt LICENSE-TRAY-DEPENDENCIES.txt RUST-DEPENDENCIES.txt \
  WEB-DEPENDENCIES.txt; do
  /bin/cp "$script_dir/$payload" "$package_root/$payload"
done
for payload in "$script_dir"/*.bat; do
  /bin/cp "$payload" "$package_root/${payload:t}"
done
/bin/cp -R "$script_dir/rust-licenses" "$package_root/rust-licenses"
/bin/cp "$repo_root/project/docs/CLIProxyAPI-v7.2.132-zcode.patch" "$package_root/CLIProxyAPI-v7.2.132-zcode.patch"

manager_sha=$(sha256_upper "$package_root/ZCode-Antigravity.exe")
control_sha=$(sha256_upper "$package_root/ZCode-Antigravity-ControlCenter.exe")
backend_sha=$(sha256_upper "$package_root/backend/cli-proxy-api.exe")

/usr/bin/sed -e "s/__MANAGER_SHA256__/$manager_sha/g" \
  -e "s/__CONTROL_CENTER_SHA256__/$control_sha/g" \
  -e "s/__BACKEND_SHA256__/$backend_sha/g" \
  "$script_dir/Verify-Package.bat.template" > "$package_root/Verify-Package.bat"
/usr/bin/sed -e "s/__MANAGER_SHA256__/$manager_sha/g" \
  -e "s/__CONTROL_CENTER_SHA256__/$control_sha/g" \
  -e "s/__BACKEND_SHA256__/$backend_sha/g" \
  "$script_dir/BUILD-INFO.txt.template" > "$package_root/BUILD-INFO.txt"

(
  cd "$package_root"
  find . -type f ! -name SHA256SUMS.txt | LC_ALL=C sort | while IFS= read -r payload_file; do
    /usr/bin/shasum -a 256 "$payload_file"
  done > SHA256SUMS.txt
)

/usr/bin/ditto -c -k --norsrc --keepParent "$package_root" "$archive_path"

payload_zip="$build_root/payload.zip"
(
  cd "$package_root"
  /usr/bin/zip -q -r "$payload_zip" .
)
payload_sha=$(sha256_upper "$payload_zip")

installer_root="$build_root/exe-installer"
/bin/mkdir -p "$installer_root"
/usr/bin/sed -e "s/__PACKAGE_VERSION__/$release_version/g" \
  -e "s/__PAYLOAD_SHA256__/$payload_sha/g" \
  -e "s/__MANAGER_SHA256__/$manager_sha/g" \
  -e "s/__CONTROL_CENTER_SHA256__/$control_sha/g" \
  -e "s/__BACKEND_SHA256__/$backend_sha/g" \
  "$script_dir/exe-installer/main.go.template" > "$installer_root/main.go"
/bin/cp "$script_dir/exe-installer/encoding.go" "$installer_root/encoding.go"
/bin/cp "$payload_zip" "$installer_root/payload.zip"
{
  /usr/bin/printf '\357\273\277'
  /bin/cat "$script_dir/exe-installer/Install-From-Package.ps1"
} > "$installer_root/install.ps1"
(
  cd "$installer_root"
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w -H windowsgui' \
    -o "$installer_path" main.go encoding.go
)

runtime_script="$build_root/oneclick.ps1"
/usr/bin/sed -e "s/__PACKAGE_VERSION__/$release_version/g" \
  -e "s/__PAYLOAD_SHA256__/$payload_sha/g" \
  -e "s/__MANAGER_SHA256__/$manager_sha/g" \
  -e "s/__CONTROL_CENTER_SHA256__/$control_sha/g" \
  -e "s/__BACKEND_SHA256__/$backend_sha/g" \
  "$script_dir/OneClick-Installer.ps1" > "$runtime_script"
if /usr/bin/grep -Eq '__[A-Z0-9_]+__' "$runtime_script" "$installer_root/main.go"; then
  print -u2 "安装器仍包含未替换占位符"
  exit 1
fi

single_lf="$build_root/oneclick.bat"
{
  /bin/cat <<'LAUNCHER'
@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
set "ZCAB_SELF=%~f0"
set "ZCAB_MODE=%~1"
set "ZCAB_TARGET=%~2"
if /I not "%ZCAB_MODE%"=="--extract-only" (
  start "" powershell.exe -NoLogo -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop';$raw=[IO.File]::ReadAllText($env:ZCAB_SELF,[Text.Encoding]::UTF8);$begin='#<ZCAB-'+'PS-BEGIN>';$end='#<ZCAB-'+'PS-END>';$i=$raw.IndexOf($begin,[StringComparison]::Ordinal);$j=$raw.IndexOf($end,[StringComparison]::Ordinal);if($i -lt 0 -or $j -le $i){throw 'Embedded PowerShell section is missing'};$i+=$begin.Length;$script=$raw.Substring($i,$j-$i);& ([ScriptBlock]::Create($script)) -SelfPath $env:ZCAB_SELF -Mode $env:ZCAB_MODE -ExtractTarget $env:ZCAB_TARGET"
  exit /b 0
)
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop';$raw=[IO.File]::ReadAllText($env:ZCAB_SELF,[Text.Encoding]::UTF8);$begin='#<ZCAB-'+'PS-BEGIN>';$end='#<ZCAB-'+'PS-END>';$i=$raw.IndexOf($begin,[StringComparison]::Ordinal);$j=$raw.IndexOf($end,[StringComparison]::Ordinal);if($i -lt 0 -or $j -le $i){throw 'Embedded PowerShell section is missing'};$i+=$begin.Length;$script=$raw.Substring($i,$j-$i);& ([ScriptBlock]::Create($script)) -SelfPath $env:ZCAB_SELF -Mode $env:ZCAB_MODE -ExtractTarget $env:ZCAB_TARGET;exit $LASTEXITCODE"
set "ZCAB_RC=%ERRORLEVEL%"
exit /b %ZCAB_RC%
LAUNCHER
  print '#<ZCAB-PS-BEGIN>'
  /bin/cat "$runtime_script"
  print '#<ZCAB-PS-END>'
  print '#<ZCAB-PAYLOAD-BEGIN>'
  /usr/bin/base64 -i "$payload_zip" | /usr/bin/fold -w 76
  print '#<ZCAB-PAYLOAD-END>'
} > "$single_lf"
/usr/bin/perl -pe 's/(?<!\r)\n/\r\n/g' "$single_lf" > "$single_bat_path"

(
  cd "$output_dir"
  /usr/bin/shasum -a 256 "${archive_path:t}" "${installer_path:t}" "${single_bat_path:t}" \
    > SHA256SUMS-Windows.txt
)

print "Built: $archive_path"
print "Built: $installer_path"
print "Built: $single_bat_path"
print "OAuth desktop configuration available in backend: $oauth_ready"
