#!/bin/zsh
set -euo pipefail

script_dir=${0:A:h}
project_dir=${script_dir:h:h}
repo_root=${project_dir:h}
backend_dir="$repo_root/third_party/CLIProxyAPI-7.2.132-patched"
release_version=${VERSION:-0.2.7-test}
short_version=${SHORT_VERSION:-0.2.7}
bundle_version=${BUNDLE_VERSION:-27}
output_dir=${OUTPUT_DIR:-$repo_root/dist/macos}
package_name="ZCode-Antigravity-macOS-Universal-v${release_version}"
package_root="$output_dir/$package_name"
archive_path="$output_dir/$package_name.zip"
build_root=$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/zcode-antigravity-macos.XXXXXX")
trap '/bin/rm -rf "$build_root"' EXIT

if ! /usr/bin/grep -Fq "const version = \"$release_version\"" "$project_dir/cmd/zcode-antigravity/main.go"; then
  print -u2 "VERSION=$release_version 与管理器源码版本不一致"
  exit 1
fi

for tool in /usr/bin/lipo /usr/bin/codesign /usr/bin/ditto /usr/bin/plutil /usr/bin/shasum go; do
  command -v "$tool" >/dev/null
done

if [[ -n ${ANTIGRAVITY_OAUTH_CLIENT_ID:-} || -n ${ANTIGRAVITY_OAUTH_CLIENT_SECRET:-} ]]; then
  if [[ -z ${ANTIGRAVITY_OAUTH_CLIENT_ID:-} || -z ${ANTIGRAVITY_OAUTH_CLIENT_SECRET:-} ]]; then
    print -u2 "ANTIGRAVITY_OAUTH_CLIENT_ID 与 ANTIGRAVITY_OAUTH_CLIENT_SECRET 必须同时设置"
    exit 1
  fi
  embedded_oauth=true
else
  embedded_oauth=false
fi

commit=$(git -C "$repo_root" rev-parse --short=12 HEAD 2>/dev/null || print none)
build_date=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

/bin/rm -rf "$package_root" "$archive_path" "$archive_path.sha256"
/bin/mkdir -p "$package_root/ZCode Antigravity.app/Contents/MacOS/backend" \
  "$package_root/ZCode Antigravity.app/Contents/Resources" "$build_root/arm64" "$build_root/amd64"

for target_arch in arm64 amd64; do
  (
    cd "$project_dir"
    CGO_ENABLED=0 GOOS=darwin GOARCH=$target_arch go build \
      -trimpath -ldflags='-s -w -X main.defaultCommand=gui' \
      -o "$build_root/$target_arch/ZCode-Antigravity" ./cmd/zcode-antigravity
  )

  backend_ldflags="-s -w -X main.Version=7.2.132-zcode.10 -X main.Commit=$commit -X main.BuildDate=$build_date"
  if [[ $embedded_oauth == true ]]; then
    backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientID=$ANTIGRAVITY_OAUTH_CLIENT_ID"
    backend_ldflags+=" -X github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravitycredentials.embeddedClientSecret=$ANTIGRAVITY_OAUTH_CLIENT_SECRET"
  fi
  (
    cd "$backend_dir"
    CGO_ENABLED=0 GOOS=darwin GOARCH=$target_arch go build \
      -trimpath -ldflags="$backend_ldflags" \
      -o "$build_root/$target_arch/cli-proxy-api" ./cmd/server
  )
done

app_root="$package_root/ZCode Antigravity.app"
/usr/bin/lipo -create "$build_root/arm64/ZCode-Antigravity" "$build_root/amd64/ZCode-Antigravity" \
  -output "$app_root/Contents/MacOS/ZCode-Antigravity"
/usr/bin/lipo -create "$build_root/arm64/cli-proxy-api" "$build_root/amd64/cli-proxy-api" \
  -output "$app_root/Contents/MacOS/backend/cli-proxy-api"
/bin/chmod 755 "$app_root/Contents/MacOS/ZCode-Antigravity" "$app_root/Contents/MacOS/backend/cli-proxy-api"

/usr/bin/sed -e "s/__SHORT_VERSION__/$short_version/g" -e "s/__BUNDLE_VERSION__/$bundle_version/g" \
  "$script_dir/Info.plist.template" > "$app_root/Contents/Info.plist"
/usr/bin/plutil -lint "$app_root/Contents/Info.plist" >/dev/null
/bin/cp "$script_dir/settings.json" "$app_root/Contents/Resources/settings.json"
/bin/cp "$script_dir/README-macOS.txt" "$app_root/Contents/Resources/README-macOS.txt"

for source_file in "$script_dir"/*.command; do
  /bin/cp "$source_file" "$package_root/${source_file:t}"
done
/bin/chmod 755 "$package_root"/*.command
/bin/cp "$script_dir/.env.example" "$package_root/.env.example"
/bin/cp "$script_dir/README-macOS.txt" "$package_root/README-macOS.txt"
/bin/cp "$script_dir/THIRD-PARTY-NOTICES.txt" "$package_root/THIRD-PARTY-NOTICES.txt"
/bin/cp "$repo_root/project/packaging/windows/LICENSE-CLIProxyAPI.txt" "$package_root/LICENSE-CLIProxyAPI.txt"

/usr/bin/codesign --force --sign - "$app_root/Contents/MacOS/backend/cli-proxy-api"
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
(
  cd "$output_dir"
  /usr/bin/shasum -a 256 "${archive_path:t}" > "${archive_path:t}.sha256"
)

print "Built: $archive_path"
print "OAuth desktop configuration embedded: $embedded_oauth"
