#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-permissions"
readonly state_root="$(mktemp -d)"

cleanup() {
  "${cleanup_script}" >/dev/null 2>&1 || true
  rm -rf "${state_root}" >/dev/null 2>&1 || true
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
  for attempt in $(seq 1 30); do
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

trap cleanup EXIT
cleanup

tmux new-session -d -s "${session_name}" \
  "cd '${shell_dir}' && npm run dev -- --session-id='${session_name}' --state-root='${state_root}' --cwd='${repo_root}'"

tmux send-keys -t "${session_name}" "tool:pwd" Enter

request_capture="$(wait_for_capture_contains "Pending Permission")"
assert_contains "${request_capture}" "Pending Permission"
assert_contains "${request_capture}" "decision>"
assert_contains "${request_capture}" "Allow pwd to access workspace?"

tmux send-keys -t "${session_name}" "y" Enter

approve_capture="$(wait_for_capture_contains "permission: allow_once")"
assert_contains "${approve_capture}" "permission: allow_once"
assert_contains "${approve_capture}" "tool:pwd pwd completed output=/Users/cdossman/klaude-kode"

tmux send-keys -t "${session_name}" "tool:pwd" Enter
tmux send-keys -t "${session_name}" "n" Enter

deny_capture="$(wait_for_capture_contains "permission: deny")"
assert_contains "${deny_capture}" "permission: deny"
assert_contains "${deny_capture}" "failure: permission/permission_denied permission denied for tool pwd"

printf 'request\n%s\n' "${request_capture}"
printf 'approve\n%s\n' "${approve_capture}"
printf 'deny\n%s\n' "${deny_capture}"
