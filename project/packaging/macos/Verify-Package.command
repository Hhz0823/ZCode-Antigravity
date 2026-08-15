#!/bin/zsh
set -euo pipefail

package_root=${0:A:h}
cd "$package_root"
if [[ ! -f SHA256SUMS.txt ]]; then
  print -u2 "缺少 SHA256SUMS.txt"
  exit 1
fi
/usr/bin/shasum -a 256 -c SHA256SUMS.txt
print "\n全部文件校验通过。"
