# Automation Handoff

## 2026-04-02

- Completed unit: Phase 3 provider selection hardening so invalid `/model` changes are rejected before session state mutates.
- Why chosen: this was the smallest meaningful Phase 2/3 parity gap still open in the current shell and engine flow, and it keeps the Go engine authoritative for provider/model validation.
- Files changed: `internal/engine/engine.go`, `internal/engine/engine_test.go`, `docs/09-automation-handoff.md`, `docs/10-upstream-parity-log.md`.
- Verification run:
  - `GOCACHE=/tmp/klaude-gocache GOMODCACHE=/tmp/klaude-gomodcache go test ./internal/engine -run 'TestUpdateSessionSetting(ChangesActiveModel|RejectsInvalidModel)|TestProfileSwitchAdoptsStoredProfileDefaults|TestInvalidAnthropicModelFailsBeforeProviderCall|TestOpenRouterCustomModelRemainsUsable'`
  - `GOCACHE=/tmp/klaude-gocache GOMODCACHE=/tmp/klaude-gomodcache go test ./...` attempted, but sandboxed loopback restrictions blocked existing `httptest.NewServer` cases in `internal/auth/anthropicoauth`, `internal/engine`, and `internal/provider`.
  - `printf '{"kind":"update_session_setting","payload":{"setting_key":"model","setting_value":"claude-not-real"}}\n' | GOCACHE=/tmp/klaude-gocache GOMODCACHE=/tmp/klaude-gomodcache go run ./cmd/cc-engine -transport=stdio -format=events -session-id=kk-smoke-invalid-model -state-root="$state_root" -cwd=/Users/cdossman/.codex/worktrees/1744/klaude-kode`
- Commit hash: pending at authoring time; recorded in automation memory and git history after commit.
- Push status: pending at authoring time; recorded in automation memory and git history after push.
- Blockers: full `go test ./...` cannot pass in this sandbox because loopback listener creation is denied for existing OAuth/live-provider tests.
- Next 3 recommended atomic units:
  - Add shell-side `/model` feedback that points users to `/models` output when a model change is rejected.
  - Add a focused `/permissions` read-only shell surface to improve Phase 2 discoverability.
  - Harden provider-switch UX so `/profile` clearly announces the adopted default model and capabilities delta.
