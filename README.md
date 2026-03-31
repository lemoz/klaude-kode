# Klaude Kode

Klaude Kode is a new repo for a near-full-parity rewrite of Claude Code with:

- a Go core for the engine, session runtime, providers, MCP, and transports
- a thin TypeScript shell for early UX parity
- first-class Anthropic and OpenRouter provider support

This repo currently contains:

- an RFC-grade rewrite blueprint
- an initial repository layout
- compile-safe Go interfaces and starter binaries
- a thin TS shell placeholder
- a phased roadmap that requires verification, testing, commit, and push after
  every implementation step

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

This repo is intentionally in blueprint-and-scaffold state. It is not yet a
working Claude Code replacement.

The next implementation steps are:

1. Build the canonical event schema and transport framing in Go.
2. Implement headless local sessions in `cc-engine`.
3. Wire the TS shell to engine events over stdio.
4. Add Anthropic and OpenRouter provider adapters.
5. Move tool runtime, permissions, replay, and MCP into engine-owned services.
6. Add harness-facing replay/eval artifacts and offline benchmark workflows.

## Working Agreement

Every implementation step is atomic and must end with:

1. behavior verification
2. relevant automated tests or checks
3. a commit for that step
4. a push before starting the next step

This repo should not accumulate multiple unfinished steps in one local-only
change set.

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
npm run dev
```

## Blueprint Docs

- [01-rfc-architecture.md](/Users/cdossman/klaude-kode/docs/01-rfc-architecture.md)
- [02-event-model.md](/Users/cdossman/klaude-kode/docs/02-event-model.md)
- [03-provider-auth-spec.md](/Users/cdossman/klaude-kode/docs/03-provider-auth-spec.md)
- [04-compatibility-matrix.md](/Users/cdossman/klaude-kode/docs/04-compatibility-matrix.md)
- [05-roadmap.md](/Users/cdossman/klaude-kode/docs/05-roadmap.md)
- [06-test-benchmarks.md](/Users/cdossman/klaude-kode/docs/06-test-benchmarks.md)
