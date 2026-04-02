#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"

cleanup() {
  (
    cd "${shell_dir}"
    npm run cleanup:smokes >/dev/null 2>&1 || true
  )
}

trap cleanup EXIT
cleanup

(
  cd "${shell_dir}"
  npm run smoke:tmux
  npm run smoke:permissions
  npm run smoke:oauth
  npm run smoke:anthropic
  npm run smoke:profiles
  npm run smoke:models
  npm run smoke:openrouter
)
