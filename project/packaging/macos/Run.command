#!/bin/zsh
set -euo pipefail

script_dir=${0:A:h}
package_root=$script_dir
if [[ ! -d "$package_root/ZCode Antigravity.app" && -d "${script_dir:h}/ZCode Antigravity.app" ]]; then
  package_root=${script_dir:h}
fi
app_binary="$package_root/ZCode Antigravity.app/Contents/MacOS/ZCode-Antigravity-Core"
if [[ ! -x "$app_binary" ]]; then
  app_binary="/Applications/ZCode Antigravity.app/Contents/MacOS/ZCode-Antigravity-Core"
fi
if [[ ! -x "$app_binary" ]]; then
  print -u2 "找不到 ZCode Antigravity.app。请保留完整解压目录，或把 App 放入 /Applications。"
  exit 1
fi

if [[ -f "$package_root/.env" ]]; then
  set -a
  source "$package_root/.env"
  set +a
fi

exec "$app_binary" "$@"
