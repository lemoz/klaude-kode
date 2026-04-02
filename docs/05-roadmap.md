# Phased Roadmap

## 1. Delivery Strategy

Use a phased roadmap, not a single cutover. Each phase has:

- concrete deliverables
- parity targets
- test gates
- rollback criteria

Use an atomic delivery loop inside every phase. Every implementation step must
end with:

- verification of the changed behavior against the phase scope
- automated tests or checks for the affected surface
- a review of the diff and any remaining known gaps
- a git commit with a phase-and-step-specific message
- a push to the tracked remote branch before the next step begins

Do not batch multiple unfinished steps into one commit. Do not begin the next
step while the current step is unverified or unpushed.

## 2. Step Completion Contract

Every step in every phase follows this order:

1. implement one bounded step
2. verify the behavior locally
3. run the narrowest meaningful automated tests/checks, then broader phase
   checks if the step completes a phase
4. fix failures until the step is green or explicitly blocked
5. commit the step
6. push the step
7. only then start the next step

If tooling is unavailable in the environment, the step is not complete until the
missing verification is called out explicitly in the commit/push handoff.

Shell/tmux verification steps add one more requirement:

- clean stale smoke sessions before and after the verification batch with
  `./scripts/cleanup-shell-smokes.sh`
- reserve the `kk-smoke-` prefix for automated shell session ids and tmux
  session names so cleanup stays targeted
- do not commit or push with stale smoke terminals still running

## 3. Current Phase Status

Status snapshot as of `2026-04-02`:

| Phase | Status | Notes |
| --- | --- | --- |
| Phase 0: Inventory and Contracts | `complete` | upstream-first parity intake and core contracts are documented |
| Phase 1: Headless Core | `complete` | Go engine, persistence, replay/resume, CLI modes, and provider boundary are in place |
| Phase 2: Thin TS Shell | `complete` | interactive Ink shell, auth/profile/model/help/report surfaces, and shell smoke gates are green |
| Phase 3: Provider Expansion | `complete` | Anthropic OAuth/API-key and OpenRouter API-key flows, capability handling, and provider smoke gates are green |
| Phase 4: Plugins, Hooks, MCP, and Marketplace Surfaces | `next` | next execution phase |
| Phase 5: Remote and Detached Sessions | `not_started` | depends on Phase 4 and stronger local parity surfaces |
| Phase 6: Harness Surface and Offline Evaluation | `complete_baseline` | replay/benchmark/export/report baseline is implemented and usable |
| Phase 7: Cutover and Import | `not_started` | import and coexistence work remains ahead |
| Phase 8: Distribution and Install Parity | `not_started` | packaging and install parity remain ahead |

## 4. Phase 0: Inventory and Contracts

### Deliverables

- current-system parity inventory
- canonical command/event schema
- core Go interfaces
- profile and settings schema
- session storage layout

### Step Sequence

1. parity inventory from live upstream Claude Code repo, docs, and changelog
2. canonical command/event schema
3. profile/settings/config precedence rules
4. session storage and artifact layout
5. fallback internal-runtime review from extracted source only where the public
   repo is insufficient

### Verification Per Step

- confirm the targeted contract is explicit and non-overlapping
- update or add schema/docs fixtures where applicable
- record which upstream source was used for the parity claim
- run repo checks affected by the step before commit/push

### Exit Criteria

- no subsystem boundary left implicit
- preserve/simplify/drop decisions approved
- engine and shell responsibilities are non-overlapping

## 5. Phase 1: Headless Core

### Scope

- `cc-engine` headless mode
- local session execution without Ink UI
- Anthropic adapter
- event log and session index
- builtin tool runtime for core local tools

### Deliverables

- print/text mode
- JSON mode
- stream event mode
- replay and resume for headless sessions

### Step Sequence

1. engine session lifecycle and authoritative event log
2. headless command handling and turn loop scaffolding
3. replay/resume and session index
4. text, JSON, and stream output modes
5. Anthropic adapter boundary and local builtin tool runtime

### Verification Per Step

- run Go unit checks for engine packages touched by the step
- exercise the relevant CLI mode manually or through fixtures
- verify no provider-native types cross the engine boundary before commit/push

### Exit Criteria

- headless turn loop passes parity tests
- event log is authoritative
- no Anthropic SDK types escape the adapter boundary

## 6. Phase 2: Thin TS Shell

### Scope

- TS/Ink shell wired to `cc-engine`
- interactive prompt and streaming transcript
- prompt/footer/status presentation
- permission dialogs
- basic slash-command routing
- model/profile switching
- command discoverability and help affordances
- Klaude Kode shell branding and visual system

### Deliverables

- interactive local REPL
- shell projections of engine state
- artifact-aware rendering for diffs and large outputs
- Klaude Kode-branded shell presentation over a familiar Claude Code-like flow
- an explicit UX parity track for layout, status context, permissions, and
  shell discoverability

### UX Track

Phase 2 is not only transport wiring. It is the point where the familiar
day-to-day Claude Code workflow gets re-expressed as Klaude Kode.

Preserve at the interaction level:

- prompt and transcript rhythm
- streamed assistant output behavior
- visible session context such as model/profile/status
- permission prompt timing and decision flow
- command discoverability for common workflows

Rebrand at the product level:

- shell copy tone
- command/help copy
- naming and framing
- visual styling and layout treatment

Do not require:

- pixel-perfect visual cloning
- exact spacing/color/token reproduction
- one-to-one reproduction of every Claude Code visual affordance

### Step Sequence

1. engine transport client in the shell
2. interactive prompt and streaming transcript
3. prompt/footer/status context and shell chrome
4. permission dialogs
5. slash-command routing, help, and model/profile controls
6. artifact-aware renderers
7. Klaude Kode branding and UX polish pass

### Verification Per Step

- run `npm run check` after every shell change before commit/push
- verify the shell remains projection-only for the touched behavior
- manually exercise the updated UI flow against `cc-engine`
- verify the changed flow stays familiar while the surface presentation remains
  distinctly Klaude Kode
- when the step changes interaction behavior, verify transcript layout, prompt
  behavior, and help/discoverability together rather than in isolation

### Exit Criteria

- shell contains no session source of truth
- local interactive workflows reach parity for common usage
- the shell is recognizably rebranded rather than a visual clone
- prompt, transcript, status, permission, and help flows feel cohesive as one
  product experience rather than a collection of shell commands

### Current Status

- `complete` as of `2026-04-02`
- phase-close verification now includes the aggregate shell smoke suite:
  - `npm run smoke:tmux`
  - `npm run smoke:permissions`
  - `npm run smoke:oauth`
  - `npm run smoke:anthropic`
  - `npm run smoke:profiles`
  - `npm run smoke:models`
  - `npm run smoke:openrouter`
  - `npm run smoke:shell`

## 7. Phase 3: Provider Expansion

### Scope

- OpenRouter adapter
- separate OpenRouter profile management
- capability-matrix enforcement
- model listing and validation

### Deliverables

- `openrouter_api_key` login/profile flow
- provider switching by profile
- adapter-normalized tool calling and structured outputs

### Step Sequence

1. auth profile registry and validation
2. provider capability matrix
3. OpenRouter adapter
4. provider selection and model validation flows
5. replay/eval metadata export for provider reproducibility

### Verification Per Step

- test profile resolution and validation paths for changed providers
- verify Anthropic and OpenRouter behaviors stay isolated
- verify capability flags drive behavior instead of provider-specific branching

### Exit Criteria

- Anthropic and OpenRouter work side by side
- provider mismatch failures are typed and recoverable

### Current Status

- `complete` as of `2026-04-02`
- phase-close verification includes:
  - Anthropic OAuth progress and login UX smoke coverage
  - Anthropic API-key live-path smoke coverage
  - OpenRouter API-key live-path smoke coverage
  - profile switching and model-flow smoke coverage
  - capability handling and invalid-model validation coverage

## 8. Phase 4: Plugins, Hooks, MCP, and Marketplace Surfaces

### Scope

- Go plugin supervisor
- hook execution and hook event model
- Go MCP supervisor and MCP auth management
- tool/resource projection
- plugin manifest split between engine and shell contributions
- marketplace-facing plugin surfaces

### Deliverables

- plugin status model
- hook event model and hook execution runtime
- MCP status model
- reconnect and auth recovery
- plugin loading and marketplace-oriented manifest handling in the new runtime

### Step Sequence

1. plugin and hook runtime contracts
2. hook execution and hook event delivery
3. MCP supervisor lifecycle
4. MCP auth and reconnect handling
5. tool/resource projection
6. plugin manifest loading, marketplace metadata, and split responsibilities

### Verification Per Step

- run the narrowest meaningful engine tests for touched plugin/hook/MCP behavior
- verify auth recovery and reconnect flows when the step affects them
- confirm shell behavior is rendering structured engine state, not rebuilding it

### Exit Criteria

- MCP-heavy sessions are stable
- hook and plugin behavior is not TS-runtime-dependent
- plugin and marketplace-facing behavior are driven by typed manifests rather
  than shell ad hoc logic

## 9. Phase 5: Remote and Detached Sessions

### Scope

- local daemon mode
- remote viewer/control
- assistant viewer
- SSH and direct-connect transports
- background session support

### Deliverables

- attach/detach flows
- reconnect semantics
- remote permission round trips
- background session index and control

### Step Sequence

1. local daemon/session host mode
2. attach/detach and background session control
3. remote viewer/control transport
4. SSH/direct-connect transport
5. reconnect and permission round-trip hardening

### Verification Per Step

- verify the same event schema is used by every touched transport
- run reconnect/resume checks whenever session authority or transport changes
- manually exercise attach/detach flows before commit/push

### Exit Criteria

- all transport modes use the same event schema
- reconnect and resume scenarios meet stability gates

## 10. Phase 6: Harness Surface and Offline Evaluation

### Scope

- replay-pack export
- candidate validation
- offline replay and benchmark evaluation
- artifact indexing and reporting
- deterministic benchmark/replay packs

### Deliverables

- `export-replay-pack`
- `validate-candidate`
- `run-replay-eval`
- `run-benchmark-eval`
- run summaries, diffing, and frontier reporting

### Step Sequence

1. replay-pack schema and export path
2. candidate validation flow
3. offline replay evaluation
4. offline benchmark evaluation
5. artifact indexes and reporting commands
6. baseline benchmark/replay packs

### Verification Per Step

- verify replay artifacts are stable and queryable without shell state
- run eval commands end to end for the affected workflow
- verify score outputs and failure classes are machine-readable before commit/push

### Exit Criteria

- replay and benchmark artifacts are reproducible
- validation runs fail fast before expensive evaluation
- harness outputs are stable enough for external tooling to consume

## 11. Phase 7: Cutover and Import

### Scope

- config importer from current Claude Code
- session history importer
- compatibility shims for retained CLI flags and commands
- production packaging

### Deliverables

- opt-in migration command
- coexistence mode
- documentation and rollback guide

### Step Sequence

1. config importer
2. session history importer
3. retained CLI flag/command compatibility shims
4. coexistence packaging and docs

### Verification Per Step

- verify importers are repeatable and non-destructive
- run migration checks against sample legacy data before commit/push
- verify rollback steps remain accurate after each migration-related change

### Exit Criteria

- users can adopt without destroying the old runtime
- importer is repeatable and non-destructive

## 12. Phase 8: Distribution and Install Parity

### Scope

- install script parity
- package manager parity
- local update surfaces
- platform packaging expectations

### Deliverables

- supported install flows for macOS, Linux, and Windows
- documented update path
- version/reporting surface aligned with supported packaging

### Step Sequence

1. install surface inventory from the live upstream repo
2. script-based install path
3. package manager integration path
4. update/version reporting and docs

### Verification Per Step

- test the changed install or update path in the narrowest realistic environment
- verify version output and docs stay aligned after packaging changes
- keep install docs explicit about supported and unsupported platforms

### Exit Criteria

- a new user can install Klaude Kode through supported flows without reading the
  source
- update behavior is documented and tested for supported packaging paths

## 13. Rollout Gates

Progress to the next phase only when:

- parity tests for the phase pass
- benchmark gates hold
- failure-mode tests pass
- rollback path is documented
- the phase-ending step has been committed and pushed

## 14. Migration Guardrails

- keep storage roots separate until final cutover
- never rewrite current Claude Code configs in place
- do not move remote workflows before local parity is proven
- do not move MCP before builtin tool runtime is stable
- do not move TS shell logic deeper into Go until event contracts settle
- do not use the extracted source as the default parity reference when the live
  upstream repo already answers the question
- do not treat local verification without commit/push as a completed step
