#!/bin/bash
set -euo pipefail
package_root="$(cd "$(dirname "$0")" && pwd)"
exec "$package_root/Run.command" smoke gemini-3.8-flash
