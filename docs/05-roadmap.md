# Phased Roadmap

## 1. Delivery Strategy

Use a phased roadmap, not a single cutover. Each phase has:

- concrete deliverables
- parity targets
- test gates
- rollback criteria

## 2. Phase 0: Inventory and Contracts

### Deliverables

- current-system parity inventory
- canonical command/event schema
- core Go interfaces
- profile and settings schema
- session storage layout

### Exit Criteria

- no subsystem boundary left implicit
- preserve/simplify/drop decisions approved
- engine and shell responsibilities are non-overlapping

## 3. Phase 1: Headless Core

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

### Exit Criteria

- headless turn loop passes parity tests
- event log is authoritative
- no Anthropic SDK types escape the adapter boundary

## 4. Phase 2: Thin TS Shell

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

### Exit Criteria

- shell contains no session source of truth
- local interactive workflows reach parity for common usage

## 5. Phase 3: Provider Expansion

### Scope

- OpenRouter adapter
- separate OpenRouter profile management
- capability-matrix enforcement
- model listing and validation

### Deliverables

- `openrouter_api_key` login/profile flow
- provider switching by profile
- adapter-normalized tool calling and structured outputs

### Exit Criteria

- Anthropic and OpenRouter work side by side
- provider mismatch failures are typed and recoverable

## 6. Phase 4: MCP and Plugins

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

### Exit Criteria

- MCP-heavy sessions are stable
- hook and plugin behavior is not TS-runtime-dependent

## 7. Phase 5: Remote and Detached Sessions

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

### Exit Criteria

- all transport modes use the same event schema
- reconnect and resume scenarios meet stability gates

## 8. Phase 6: Cutover and Import

### Scope

- config importer from current Claude Code
- session history importer
- compatibility shims for retained CLI flags and commands
- production packaging

### Deliverables

- opt-in migration command
- coexistence mode
- documentation and rollback guide

### Exit Criteria

- users can adopt without destroying the old runtime
- importer is repeatable and non-destructive

## 9. Rollout Gates

Progress to the next phase only when:

- parity tests for the phase pass
- benchmark gates hold
- failure-mode tests pass
- rollback path is documented

## 10. Migration Guardrails

- keep storage roots separate until final cutover
- never rewrite current Claude Code configs in place
- do not move remote workflows before local parity is proven
- do not move MCP before builtin tool runtime is stable
- do not move TS shell logic deeper into Go until event contracts settle

