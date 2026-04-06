# Klaude Kode

Klaude Kode is a new repo for a near-full-parity rewrite of Claude Code with:

- a Go core for the engine, session runtime, providers, MCP, and transports
- a thin TypeScript shell for early UX parity
- first-class Anthropic and OpenRouter provider support

UX target:

- preserve a similar day-to-day interaction model to Claude Code for common
  workflows
- do not chase pixel-perfect visual cloning
- establish a distinct Klaude Kode brand, copy tone, and visual identity on top
  of the preserved workflow shape

This repo currently contains:

- an RFC-grade rewrite blueprint
- a working Go engine/CLI baseline
- a working TS shell baseline over stdio
- persisted harness artifacts and offline replay/benchmark workflows
- a phased roadmap that requires verification, testing, commit, and push after
  every implementation step

## Parity Sources

Use parity sources in this order:

1. the live upstream Claude Code repo at
   [anthropics/claude-code](https://github.com/anthropics/claude-code)
2. upstream docs and changelog from that repo
3. the extracted reference tree at `/Users/cdossman/Downloads/src` only when
   the public repo and docs do not answer an internal runtime question

The live upstream repo is now the primary reference for current feature, UX,
plugin, hooks, and packaging parity.

## Repo Layout

- [docs](/Users/cdossman/klaude-kode/docs)
  - architecture, events, provider/auth, compatibility, roadmap, and test plan
- [cmd/cc](/Users/cdossman/klaude-kode/cmd/cc)
  - future CLI launcher
- [cmd/cc-engine](/Users/cdossman/klaude-kode/cmd/cc-engine)
  - future Go engine binary
- [internal/contracts](/Users/cdossman/klaude-kode/internal/contracts)
  - provider-neutral core contracts
- [internal/engine](/Users/cdossman/klaude-kode/internal/engine)
  - engine interfaces and starter implementation
- [internal/provider](/Users/cdossman/klaude-kode/internal/provider)
  - provider adapter interfaces
- [internal/toolruntime](/Users/cdossman/klaude-kode/internal/toolruntime)
  - tool runtime interfaces
- [internal/transport](/Users/cdossman/klaude-kode/internal/transport)
  - session transport interfaces
- [shell](/Users/cdossman/klaude-kode/shell)
  - thin TypeScript shell placeholder

## Current Status

This repo is not yet a full Claude Code replacement, but it is past
blueprint-only state. The current implementation supports:

- persisted sessions, replay/resume, and local shell transport
- a completed Phase 2 local shell UX baseline with:
  - grouped transcript rendering
  - help, profile, model, and status surfaces
  - permission prompts
  - native replay/benchmark/frontier/diff views
  - tmux-backed shell smoke coverage
- Anthropic API key, Anthropic OAuth, and OpenRouter API-key profiles
- a completed Phase 3 local provider baseline with:
  - Anthropic OAuth login progress and refresh handling
  - Anthropic API-key login and live provider turns
  - OpenRouter API-key login, custom-model support, and live provider turns
  - capability-mismatch handling and provider/profile/model smoke coverage
- an in-progress Phase 4 plugin baseline with:
  - typed plugin manifest parsing and contribution discovery
  - root validation for `README.md`, `hooks/`, and malformed contribution
    layouts before loader work begins
- live provider-backed CLI and shell turns
- replay-pack export
- candidate validation
- replay eval and benchmark eval
- run summaries, run inspection, diffing, and frontier listing

The next implementation steps are:

1. Expand plugins, hooks, MCP, and marketplace-facing parity.
2. Add remote, detached, and attachable session support.
3. Deepen tool runtime parity and permissions policy.
4. Extend harness reporting, candidate metadata, and external evaluator hooks.
5. Add install and distribution parity for the supported platforms.

## Working Agreement

Every implementation step is atomic and must end with:

1. behavior verification
2. relevant automated tests or checks
3. a commit for that step
4. a push before starting the next step

This repo should not accumulate multiple unfinished steps in one local-only
change set.

## Test Hygiene

Shell and tmux smoke tests must not leave detached sessions or background
processes behind.

- reserve the `kk-smoke-` prefix for automated shell session ids and tmux
  session names
- run `./scripts/cleanup-shell-smokes.sh` before and after shell/tmux smoke
  batches
- use `cd shell && npm run cleanup:smokes` when you are already working from the
  shell package
- if a shell/tmux smoke command launches a long-lived process, wrap it with an
  `EXIT` trap that calls the cleanup script
- do not commit or push while stale smoke terminals are still running

## Quick Commands

Go:

```bash
go test ./...
go run ./cmd/cc-engine
go run ./cmd/cc
```

Shell:

```bash
cd shell
npm install
npm run check
npm run cleanup:smokes
npm run dev
```

## Harness Quickstart

Repository fixtures:

- [pass-basic.json](/Users/cdossman/klaude-kode/benchmarks/replays/pass-basic.json)
- [fail-basic.json](/Users/cdossman/klaude-kode/benchmarks/replays/fail-basic.json)
- [mixed-basic.json](/Users/cdossman/klaude-kode/benchmarks/packs/mixed-basic.json)

Validate a candidate:

```bash
go run ./cmd/cc-engine -validate-candidate -cwd=/path/to/candidate
go run ./cmd/cc -validate-candidate -cwd=/path/to/candidate
```

Run replay and benchmark evals:

```bash
go run ./cmd/cc-engine \
  -run-replay-eval \
  -cwd=/path/to/candidate \
  -replay-path=/Users/cdossman/klaude-kode/benchmarks/replays/pass-basic.json

go run ./cmd/cc-engine \
  -run-benchmark-eval \
  -cwd=/path/to/candidate \
  -benchmark-path=/Users/cdossman/klaude-kode/benchmarks/packs/mixed-basic.json
```

Inspect persisted runs:

```bash
go run ./cmd/cc-engine -summarize-runs -cwd=/path/to/candidate
go run ./cmd/cc-engine -list-frontier -cwd=/path/to/candidate -frontier-limit=5
go run ./cmd/cc-engine -show-run -cwd=/path/to/candidate -run-id=<run-id>
go run ./cmd/cc-engine -diff-runs -cwd=/path/to/candidate -left-run-id=<left> -right-run-id=<right>
```

Shell equivalents:

```text
/validate-candidate
/run-replay /Users/cdossman/klaude-kode/benchmarks/replays/pass-basic.json
/run-benchmark /Users/cdossman/klaude-kode/benchmarks/packs/mixed-basic.json
/summarize-runs
/list-frontier 5
/show-run <run-id>
/diff-runs <left-run-id> <right-run-id>
```

## Blueprint Docs

- [01-rfc-architecture.md](/Users/cdossman/klaude-kode/docs/01-rfc-architecture.md)
- [02-event-model.md](/Users/cdossman/klaude-kode/docs/02-event-model.md)
- [03-provider-auth-spec.md](/Users/cdossman/klaude-kode/docs/03-provider-auth-spec.md)
- [04-compatibility-matrix.md](/Users/cdossman/klaude-kode/docs/04-compatibility-matrix.md)
- [05-roadmap.md](/Users/cdossman/klaude-kode/docs/05-roadmap.md)
- [06-test-benchmarks.md](/Users/cdossman/klaude-kode/docs/06-test-benchmarks.md)
- [07-harness-schemas.md](/Users/cdossman/klaude-kode/docs/07-harness-schemas.md)
- [08-harness-workflows.md](/Users/cdossman/klaude-kode/docs/08-harness-workflows.md)
- [10-upstream-parity-log.md](/Users/cdossman/klaude-kode/docs/10-upstream-parity-log.md)
