#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-models-$$"
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

trap cleanup EXIT
cleanup

tmux new-session -d -x 200 -y 60 -s "${session_name}" \
  "cd '${shell_dir}' && npm run dev -- --session-id='${session_name}' --state-root='${state_root}' --cwd='${repo_root}'"

wait_for_capture_contains "enter a prompt or /help" >/dev/null

tmux send-keys -t "${session_name}" "/models anthropic-main" Enter
catalog_capture="$(wait_for_capture_contains "profile: anthropic-main")"
assert_contains "${catalog_capture}" "Model Catalog"
assert_contains "${catalog_capture}" "profile: anthropic-main"
assert_contains "${catalog_capture}" "available: claude-sonnet-4-6 (current), claude-opus-4-6"

tmux send-keys -t "${session_name}" "/model claude-not-real" Enter
invalid_capture="$(wait_for_capture_contains 'error: model "claude-not-real" is not available for provider anthropic')"
assert_contains "${invalid_capture}" 'error: model "claude-not-real" is not available for provider anthropic'
assert_contains "${invalid_capture}" "model=claude-sonnet-4-6"

tmux send-keys -t "${session_name}" "/profile openrouter-main" Enter
profile_capture="$(wait_for_capture_contains "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key")"
assert_contains "${profile_capture}" "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key"

tmux send-keys -t "${session_name}" "/model my/custom-model" Enter
custom_model_capture="$(wait_for_capture_contains "model=my/custom-model")"
assert_contains "${custom_model_capture}" "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key model=my/custom-model"

tmux send-keys -t "${session_name}" "hello custom openrouter" Enter
reply_capture="$(wait_for_capture_contains "klaude: OpenRouter response from my/custom-model")"
assert_contains "${reply_capture}" "klaude: OpenRouter response from my/custom-model"
assert_contains "${reply_capture}" "you: hello custom openrouter"

tmux send-keys -t "${session_name}" "/exit" Enter
sleep 1

printf 'catalog\n%s\n' "${catalog_capture}"
printf 'invalid\n%s\n' "${invalid_capture}"
printf 'custom\n%s\n' "${reply_capture}"
