#!/bin/zsh
set -euo pipefail

script_dir=${0:A:h}
project_dir=${script_dir:h:h}
repo_root=${project_dir:h}
backend_dir="$repo_root/third_party/CLIProxyAPI-7.2.132-patched"
release_version=${VERSION:-0.6.13-test}
short_version=${SHORT_VERSION:-0.6.13}
bundle_version=${BUNDLE_VERSION:-613}
output_dir=${OUTPUT_DIR:-$repo_root/dist/macos}
package_name="ZCode-Antigravity-macOS-Universal-v${release_version}"
build_root=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/zcode-antigravity-macos.XXXXXX")
package_root="$build_root/stage/$package_name"
archive_path="$build_root/$package_name.zip"
final_package_root="$output_dir/$package_name"
final_archive_path="$output_dir/$package_name.zip"
trap '/bin/rm -rf "$build_root"' EXIT

if ! /usr/bin/grep -Fq "const version = \"$release_version\"" "$project_dir/cmd/zcode-antigravity/main.go"; then
  print -u2 "VERSION=$release_version 与管理器源码版本不一致"
  exit 1
fi

for tool in /usr/bin/lipo /usr/bin/codesign /usr/bin/ditto /usr/bin/iconutil /usr/bin/plutil /usr/bin/xattr \
  /usr/bin/shasum /usr/bin/sips /usr/bin/strings /usr/bin/unzip go swift swiftc; do
  command -v "$tool" >/dev/null
done

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

if [[ -n ${PREBUILT_BACKEND_UNIVERSAL:-} ]]; then
  if [[ ! -f $PREBUILT_BACKEND_UNIVERSAL ]]; then
    print -u2 "找不到 PREBUILT_BACKEND_UNIVERSAL=$PREBUILT_BACKEND_UNIVERSAL"
    exit 1
  fi
  if /usr/bin/strings "$PREBUILT_BACKEND_UNIVERSAL" | /usr/bin/grep -E '[0-9]+-[A-Za-z0-9_-]+\.apps\.googleusercontent\.com' >/dev/null && \
    /usr/bin/strings "$PREBUILT_BACKEND_UNIVERSAL" | /usr/bin/grep -E 'GOCSPX-[A-Za-z0-9_-]{20,}' >/dev/null; then
    oauth_ready=true
  fi
fi
if [[ $oauth_ready != true && ${ALLOW_RUNTIME_OAUTH_CONFIG:-0} != 1 ]]; then
  print -u2 '拒绝构建无法登录 Antigravity 的发布包：请注入 OAuth 桌面配置，或提供已验证的 PREBUILT_BACKEND_UNIVERSAL。'
  print -u2 '仅开发环境可显式设置 ALLOW_RUNTIME_OAUTH_CONFIG=1，改为运行时环境变量配置。'
  exit 1
fi

commit=$(git -C "$repo_root" rev-parse --short=12 HEAD 2>/dev/null || print none)
build_date=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

/bin/rm -rf "$final_package_root" "$final_archive_path" "$final_archive_path.sha256"
/bin/mkdir -p "$output_dir"
/bin/mkdir -p "$package_root/ZCode Antigravity.app/Contents/MacOS/backend" \
  "$package_root/ZCode Antigravity.app/Contents/Resources" "$package_root/Terminal Tools" \
  "$build_root/arm64" "$build_root/amd64"

app_root="$package_root/ZCode Antigravity.app"
native_source="$repo_root/project/native/macos/ZCodeAntigravityApp.swift"
icon_source="$repo_root/project/native/macos/GenerateAppIcon.swift"
iconset="$build_root/AppIcon.iconset"
/bin/mkdir -p "$iconset"
swift "$icon_source" "$build_root/AppIcon-master.png"
for icon_size in 16 32 128 256 512; do
  /usr/bin/sips -z $icon_size $icon_size "$build_root/AppIcon-master.png" \
    --out "$iconset/icon_${icon_size}x${icon_size}.png" >/dev/null
  retina_size=$((icon_size * 2))
  /usr/bin/sips -z $retina_size $retina_size "$build_root/AppIcon-master.png" \
    --out "$iconset/icon_${icon_size}x${icon_size}@2x.png" >/dev/null
done
/usr/bin/iconutil -c icns "$iconset" -o "$app_root/Contents/Resources/AppIcon.icns"

for target_arch in arm64 amd64; do
  (
    cd "$project_dir"
    CGO_ENABLED=0 GOOS=darwin GOARCH=$target_arch go build \
      -trimpath -ldflags='-s -w' \
      -o "$build_root/$target_arch/ZCode-Antigravity-Core" ./cmd/zcode-antigravity
  )

  swift_arch=$target_arch
  [[ $target_arch == amd64 ]] && swift_arch=x86_64
  swiftc -O -whole-module-optimization -swift-version 5 \
    -target "${swift_arch}-apple-macos12.0" \
    -runtime-compatibility-version none -disable-autolinking-runtime-compatibility \
    -framework SwiftUI -framework AppKit -parse-as-library \
    -o "$build_root/$target_arch/ZCode-Antigravity" "$native_source"

  backend_ldflags="-s -w -X main.Version=7.2.132-zcode.13 -X main.Commit=$commit -X main.BuildDate=$build_date"
  if [[ $env_oauth == true ]]; then
    backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientID=$ANTIGRAVITY_OAUTH_CLIENT_ID"
    backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientSecret=$ANTIGRAVITY_OAUTH_CLIENT_SECRET"
  fi
  if [[ -z ${PREBUILT_BACKEND_UNIVERSAL:-} ]]; then
    (
      cd "$backend_dir"
      CGO_ENABLED=0 GOOS=darwin GOARCH=$target_arch go build \
        -trimpath -ldflags="$backend_ldflags" \
        -o "$build_root/$target_arch/cli-proxy-api" ./cmd/server
    )
  fi
done

if [[ -n ${PREBUILT_BACKEND_UNIVERSAL:-} ]]; then
  if [[ ! -f $PREBUILT_BACKEND_UNIVERSAL ]]; then
    print -u2 "找不到 PREBUILT_BACKEND_UNIVERSAL=$PREBUILT_BACKEND_UNIVERSAL"
    exit 1
  fi
  if [[ $(/usr/bin/lipo -archs "$PREBUILT_BACKEND_UNIVERSAL") != *arm64* || \
        $(/usr/bin/lipo -archs "$PREBUILT_BACKEND_UNIVERSAL") != *x86_64* ]]; then
    print -u2 "预构建后端不是 arm64 + x86_64 Universal Mach-O"
    exit 1
  fi
  /bin/cp "$PREBUILT_BACKEND_UNIVERSAL" "$build_root/arm64/cli-proxy-api"
  /bin/cp "$PREBUILT_BACKEND_UNIVERSAL" "$build_root/amd64/cli-proxy-api"
fi

/usr/bin/lipo -create "$build_root/arm64/ZCode-Antigravity" "$build_root/amd64/ZCode-Antigravity" \
  -output "$app_root/Contents/MacOS/ZCode-Antigravity"
/usr/bin/lipo -create "$build_root/arm64/ZCode-Antigravity-Core" "$build_root/amd64/ZCode-Antigravity-Core" \
  -output "$app_root/Contents/MacOS/ZCode-Antigravity-Core"
if [[ -n ${PREBUILT_BACKEND_UNIVERSAL:-} ]]; then
  /bin/cp "$PREBUILT_BACKEND_UNIVERSAL" "$app_root/Contents/MacOS/backend/cli-proxy-api"
else
  /usr/bin/lipo -create "$build_root/arm64/cli-proxy-api" "$build_root/amd64/cli-proxy-api" \
    -output "$app_root/Contents/MacOS/backend/cli-proxy-api"
fi
/bin/chmod 755 "$app_root/Contents/MacOS/ZCode-Antigravity" \
  "$app_root/Contents/MacOS/ZCode-Antigravity-Core" "$app_root/Contents/MacOS/backend/cli-proxy-api"

/usr/bin/sed -e "s/__SHORT_VERSION__/$short_version/g" -e "s/__BUNDLE_VERSION__/$bundle_version/g" \
  "$script_dir/Info.plist.template" > "$app_root/Contents/Info.plist"
/usr/bin/plutil -lint "$app_root/Contents/Info.plist" >/dev/null
/bin/cp "$script_dir/settings.json" "$app_root/Contents/Resources/settings.json"
/bin/cp "$script_dir/README-macOS.txt" "$app_root/Contents/Resources/README-macOS.txt"

for source_file in "$script_dir"/*.command; do
  /bin/cp "$source_file" "$package_root/Terminal Tools/${source_file:t}"
done
/bin/chmod 755 "$package_root/Terminal Tools"/*.command
/bin/cp "$script_dir/.env.example" "$package_root/.env.example"
/bin/cp "$script_dir/README-macOS.txt" "$package_root/README-macOS.txt"
/bin/cp "$script_dir/THIRD-PARTY-NOTICES.txt" "$package_root/THIRD-PARTY-NOTICES.txt"
/bin/cp "$repo_root/project/packaging/windows/LICENSE-CLIProxyAPI.txt" "$package_root/LICENSE-CLIProxyAPI.txt"
/bin/cp "$repo_root/project/packaging/windows/LICENSE-TRAY-DEPENDENCIES.txt" "$package_root/LICENSE-TRAY-DEPENDENCIES.txt"

/usr/bin/xattr -cr "$app_root"
/usr/bin/codesign --force --sign - "$app_root/Contents/MacOS/backend/cli-proxy-api"
/usr/bin/codesign --force --sign - "$app_root/Contents/MacOS/ZCode-Antigravity-Core"
/usr/bin/codesign --force --sign - "$app_root/Contents/MacOS/ZCode-Antigravity"
/usr/bin/codesign --force --deep --sign - "$app_root"
/usr/bin/codesign --verify --deep --strict "$app_root"

(
  cd "$package_root"
  find . -type f ! -name SHA256SUMS.txt | LC_ALL=C sort | while IFS= read -r payload_file; do
    /usr/bin/shasum -a 256 "$payload_file"
  done > SHA256SUMS.txt
)

/usr/bin/ditto -c -k --norsrc --keepParent "$package_root" "$archive_path"
/bin/cp "$archive_path" "$final_archive_path"
/usr/bin/unzip -tq "$final_archive_path" >/dev/null
(
  cd "$output_dir"
  /usr/bin/shasum -a 256 "${final_archive_path:t}" > "${final_archive_path:t}.sha256"
)

print "Built: $final_archive_path"
print "OAuth desktop configuration available in backend: $oauth_ready"
