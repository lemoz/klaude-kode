#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-ui-regression"
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

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  if printf "%s" "${haystack}" | rg -F -q "${needle}"; then
    printf 'expected capture not to contain: %s\n' "${needle}" >&2
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

idle_capture="$(wait_for_capture_contains "Klaude Kode Terminal")"
assert_contains "${idle_capture}" "Klaude Kode Terminal"
assert_not_contains "${idle_capture}" "Operations"

tmux send-keys -t "${session_name}" "/help" Enter
help_capture="$(wait_for_capture_contains "Klaude Kode Help")"
assert_contains "${help_capture}" "Operations"
assert_contains "${help_capture}" "Klaude Kode Help"

tmux send-keys -t "${session_name}" "/exit" Enter
sleep 2

exit_capture="$(capture_pane)"
assert_not_contains "${exit_capture}" "closed>"

printf 'idle\n%s\n' "${idle_capture}"
printf 'help\n%s\n' "${help_capture}"
printf 'exit\n%s\n' "${exit_capture}"
