#!/usr/bin/env bash

set -euo pipefail

readonly repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly shell_dir="${repo_root}/shell"
readonly cleanup_script="${repo_root}/scripts/cleanup-shell-smokes.sh"
readonly session_name="kk-smoke-openrouter-$$"
readonly state_root="$(mktemp -d)"
readonly server_log="$(mktemp)"
server_pid=""

cleanup() {
  if [[ -n "${server_pid}" ]] && kill -0 "${server_pid}" >/dev/null 2>&1; then
    kill "${server_pid}" >/dev/null 2>&1 || true
    wait "${server_pid}" >/dev/null 2>&1 || true
  fi
  "${cleanup_script}" >/dev/null 2>&1 || true
  rm -rf "${state_root}" >/dev/null 2>&1 || true
  rm -f "${server_log}" >/dev/null 2>&1 || true
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

wait_for_log_contains() {
  local needle="$1"
  local attempt
  for attempt in $(seq 1 40); do
    if rg -F -q "${needle}" "${server_log}"; then
      return 0
    fi
    sleep 0.25
  done
  printf 'server log did not contain: %s\n' "${needle}" >&2
  cat "${server_log}" >&2 || true
  return 1
}

trap cleanup EXIT
cleanup

node >"${server_log}" 2>&1 <<'EOF' &
const http = require("node:http");

const expectedKey = "openrouter-secret";

const server = http.createServer((req, res) => {
  if (req.method !== "POST" || req.url !== "/chat/completions") {
    res.statusCode = 404;
    res.end("not found");
    return;
  }

  const authorization = req.headers["authorization"];
  if (authorization !== `Bearer ${expectedKey}`) {
    res.statusCode = 401;
    res.setHeader("content-type", "application/json");
    res.end(JSON.stringify({ error: { message: `unexpected auth header: ${authorization}` } }));
    return;
  }

  let body = "";
  req.setEncoding("utf8");
  req.on("data", (chunk) => {
    body += chunk;
  });
  req.on("end", () => {
    const parsed = JSON.parse(body);
    console.log(`REQUEST_OK model=${parsed.model}`);
    res.setHeader("content-type", "application/json");
    res.end(JSON.stringify({ choices: [{ message: { content: "shell openrouter smoke reply" } }] }));
  });
});

server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") {
    process.exit(1);
  }
  console.log(`PORT=${address.port}`);
});

process.on("SIGTERM", () => {
  server.close(() => process.exit(0));
});
EOF
server_pid="$!"

wait_for_log_contains "PORT="
readonly api_port="$(sed -n 's/^PORT=//p' "${server_log}" | head -n 1)"

tmux new-session -d -x 200 -y 60 -s "${session_name}" \
  "cd '${shell_dir}' && OPENROUTER_SMOKE_KEY='openrouter-secret' npm run dev -- --session-id='${session_name}' --state-root='${state_root}' --cwd='${repo_root}'"

wait_for_capture_contains "enter a prompt or /help" >/dev/null

tmux send-keys -t "${session_name}" "/login openrouter OPENROUTER_SMOKE_KEY model=openrouter/auto api_base=http://127.0.0.1:${api_port}" Enter

login_capture="$(wait_for_capture_contains "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key model=openrouter/auto")"
assert_contains "${login_capture}" "profile=openrouter-main provider=openrouter auth=configured method=openrouter_api_key model=openrouter/auto"
assert_contains "${login_capture}" "surface=profiles status=active"
assert_contains "${login_capture}" "openrouter-main (active) (openrouter/openrouter_api_key)"

tmux send-keys -t "${session_name}" "hello live openrouter" Enter

reply_capture="$(wait_for_capture_contains "klaude: shell openrouter smoke reply")"
assert_contains "${reply_capture}" "klaude: shell openrouter smoke reply"
assert_contains "${reply_capture}" "turn turn_"
wait_for_log_contains "REQUEST_OK model=openrouter/auto"

printf 'login\n%s\n' "${login_capture}"
printf 'reply\n%s\n' "${reply_capture}"
