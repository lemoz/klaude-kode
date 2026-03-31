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

## 3. Phase 0: Inventory and Contracts

### Deliverables

- current-system parity inventory
- canonical command/event schema
- core Go interfaces
- profile and settings schema
- session storage layout

### Step Sequence

1. parity inventory from extracted Claude Code source
2. canonical command/event schema
3. profile/settings/config precedence rules
4. session storage and artifact layout

### Verification Per Step

- confirm the targeted contract is explicit and non-overlapping
- update or add schema/docs fixtures where applicable
- run repo checks affected by the step before commit/push

### Exit Criteria

- no subsystem boundary left implicit
- preserve/simplify/drop decisions approved
- engine and shell responsibilities are non-overlapping

## 4. Phase 1: Headless Core

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

## 5. Phase 2: Thin TS Shell

### Scope

- TS/Ink shell wired to `cc-engine`
- interactive prompt and streaming transcript
- permission dialogs
- basic slash-command routing
- model/profile switching

### Deliverables

- interactive local REPL
- shell projections of engine state
- artifact-aware rendering for diffs and large outputs

### Step Sequence

1. engine transport client in the shell
2. interactive prompt and streaming transcript
3. permission dialogs
4. slash-command routing and model/profile controls
5. artifact-aware renderers

### Verification Per Step

- run `npm run check` after every shell change before commit/push
- verify the shell remains projection-only for the touched behavior
- manually exercise the updated UI flow against `cc-engine`

### Exit Criteria

- shell contains no session source of truth
- local interactive workflows reach parity for common usage

## 6. Phase 3: Provider Expansion

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

## 7. Phase 4: MCP and Plugins

### Scope

- Go MCP supervisor
- MCP auth management
- tool/resource projection
- engine-owned hooks
- plugin manifest split between engine and shell contributions

### Deliverables

- MCP status model
- reconnect and auth recovery
- plugin loading and hook execution in the new runtime

### Step Sequence

1. MCP supervisor lifecycle
2. MCP auth and reconnect handling
3. tool/resource projection
4. hook execution
5. plugin manifest loading and split responsibilities

### Verification Per Step

- run the narrowest meaningful engine tests for touched MCP/plugin behavior
- verify auth recovery and reconnect flows when the step affects them
- confirm shell behavior is rendering structured engine state, not rebuilding it

### Exit Criteria

- MCP-heavy sessions are stable
- hook and plugin behavior is not TS-runtime-dependent

## 8. Phase 5: Remote and Detached Sessions

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

## 9. Phase 6: Harness Surface and Offline Evaluation

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

## 10. Phase 7: Cutover and Import

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

## 11. Rollout Gates

Progress to the next phase only when:

- parity tests for the phase pass
- benchmark gates hold
- failure-mode tests pass
- rollback path is documented
- the phase-ending step has been committed and pushed

## 12. Migration Guardrails

- keep storage roots separate until final cutover
- never rewrite current Claude Code configs in place
- do not move remote workflows before local parity is proven
- do not move MCP before builtin tool runtime is stable
- do not move TS shell logic deeper into Go until event contracts settle
- do not treat local verification without commit/push as a completed step
