#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-oauth-$$"
readonly state_root="$(mktemp -d)"
readonly fake_bin="$(mktemp -d)"

cleanup() {
  "${cleanup_script}" >/dev/null 2>&1 || true
  rm -rf "${state_root}" >/dev/null 2>&1 || true
  rm -rf "${fake_bin}" >/dev/null 2>&1 || true
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if ! printf "%s" "${haystack}" | rg -F -q "${needle}"; then
    printf 'expected capture to contain: %s\n' "${needle}" >&2
    return 1
  fi
}

capture_pane() {
  tmux capture-pane -pt "${session_name}" 2>/dev/null || true
}

wait_for_capture_contains() {
  local needle="$1"
  local capture=""
  local attempt
  for attempt in $(seq 1 60); do
    capture="$(capture_pane)"
    if printf "%s" "${capture}" | rg -F -q "${needle}"; then
      printf "%s" "${capture}"
      return 0
    fi
    sleep 0.25
  done
  printf "%s" "${capture}"
  return 1
}

cat >"${fake_bin}/open" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "${fake_bin}/open"

trap cleanup EXIT
cleanup

tmux new-session -d -s "${session_name}" \
  "cd '${shell_dir}' && PATH='${fake_bin}':\$PATH npm run dev -- --session-id='${session_name}' --state-root='${state_root}' --cwd='${repo_root}'"

tmux send-keys -t "${session_name}" "/login anthropic oauth" Enter

oauth_capture="$(wait_for_capture_contains "anthropic oauth: open this URL if your browser does not launch:")"
assert_contains "${oauth_capture}" "auth: opening browser for Anthropic OAuth"
assert_contains "${oauth_capture}" "auth: waiting for Anthropic OAuth callback"
assert_contains "${oauth_capture}" "anthropic oauth: open this URL if your browser does not launch:"
assert_contains "${oauth_capture}" "https://claude.com/cai/oauth/authorize"
assert_contains "${oauth_capture}" "wait>   engine busy"

printf 'oauth\n%s\n' "${oauth_capture}"
