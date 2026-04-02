#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-profiles"
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
  for attempt in $(seq 1 40); do
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

tmux new-session -d -x 200 -y 60 -s "${session_name}" \
  "cd '${shell_dir}' && npm run dev -- --session-id='${session_name}' --state-root='${state_root}' --cwd='${repo_root}'"

wait_for_capture_contains "enter a prompt or /help" >/dev/null

tmux send-keys -t "${session_name}" "/profile openrouter-main" Enter
openrouter_capture="$(wait_for_capture_contains "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key")"
assert_contains "${openrouter_capture}" "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key"
assert_contains "${openrouter_capture}" "surface=conversation status=active"

tmux send-keys -t "${session_name}" "/profile anthropic-main" Enter
anthropic_capture="$(wait_for_capture_contains "profile=anthropic-main provider=anthropic auth=logged_out method=anthropic_oauth")"
assert_contains "${anthropic_capture}" "profile=anthropic-main provider=anthropic auth=logged_out method=anthropic_oauth"
assert_contains "${anthropic_capture}" "surface=conversation status=active"

tmux send-keys -t "${session_name}" "/exit" Enter
sleep 1

printf 'openrouter\n%s\n' "${openrouter_capture}"
printf 'anthropic\n%s\n' "${anthropic_capture}"
