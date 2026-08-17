#!/bin/zsh
set -euo pipefail
package_root=${0:A:h}
exec "$package_root/Run.command" smoke grok-build-0.1
