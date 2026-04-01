#!/usr/bin/env bash

set -euo pipefail

readonly reserved_prefix="kk-smoke-"
readonly legacy_session_pattern="session-id=(help-smoke|ink-shell-smoke|ink-shell-startup-smoke|header-shell-smoke|grouped-transcript-smoke|tool-turn-smoke|permission-card-smoke|footer-status-smoke|prompt-state-smoke|help-smoke-tty|hint-smoke|copy-smoke|brand-smoke|artifact-view-smoke|artifact-native-smoke|header-status-smoke|tmux-ui-review)"
readonly -a legacy_tmux_sessions=(
  "klaude-ui-smoke"
  "tmux-ui-review"
)
readonly -a extra_process_patterns=(
  "oauth-shell-test"
)
collected_pids=()

kill_tmux_session() {
  local session_name="$1"
  if tmux has-session -t "$session_name" 2>/dev/null; then
    tmux kill-session -t "$session_name" 2>/dev/null || true
    printf 'killed tmux session: %s\n' "$session_name"
  fi
}

if command -v tmux >/dev/null 2>&1; then
  while IFS= read -r session_name; do
    if [[ -n "$session_name" && "$session_name" == "${reserved_prefix}"* ]]; then
      kill_tmux_session "$session_name"
    fi
  done < <(tmux ls -F '#S' 2>/dev/null || true)

  for session_name in "${legacy_tmux_sessions[@]}"; do
    kill_tmux_session "$session_name"
  done
fi

collect_process_pid() {
  local pid="$1"
  local existing
  for existing in "${collected_pids[@]-}"; do
    if [[ "$existing" == "$pid" ]]; then
      return
    fi
  done
  collected_pids+=("$pid")
}

while IFS= read -r line; do
  local_pid="${line%% *}"
  local_command="${line#* }"

  if [[ -z "$local_pid" || "$local_pid" == "$$" || "$local_pid" == "$PPID" ]]; then
    continue
  fi

  if [[ "$local_command" == *"$reserved_prefix"* ]]; then
    collect_process_pid "$local_pid"
    continue
  fi

  if [[ "$local_command" =~ $legacy_session_pattern ]]; then
    collect_process_pid "$local_pid"
    continue
  fi

  for pattern in "${extra_process_patterns[@]}"; do
    if [[ "$local_command" == *"$pattern"* ]]; then
      collect_process_pid "$local_pid"
      break
    fi
  done
done < <(ps -Ao pid=,command=)

for pid in "${collected_pids[@]-}"; do
  if [[ -z "$pid" ]]; then
    continue
  fi
  if kill "$pid" 2>/dev/null; then
    printf 'killed process pid=%s\n' "$pid"
  fi
done
