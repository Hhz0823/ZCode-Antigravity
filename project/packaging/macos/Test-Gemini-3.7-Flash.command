#!/bin/zsh
set -euo pipefail
package_root=${0:A:h}
exec "$package_root/Run.command" smoke gemini-3.7-flash
