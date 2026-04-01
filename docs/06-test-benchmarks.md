# Test and Benchmark Plan

## 1. Test Strategy

The blueprint is only complete if implementation can be validated against
scenario, failure, and performance gates.

## 1.1 Test Hygiene

Shell and tmux smoke tests must leave the environment clean.

- automated shell session ids and tmux sessions use the `kk-smoke-` prefix
- run `./scripts/cleanup-shell-smokes.sh` before and after shell/tmux smoke
  batches
- if a smoke command launches a long-lived process, add an `EXIT` trap that
  calls the cleanup script
- stale smoke terminals are a failed verification state, not a cosmetic issue

## 2. Scenario Coverage

### 2.1 Local Interactive REPL

Must test:

- start interactive session
- submit prompt and stream assistant deltas
- invoke builtin tools
- approve and deny permissions
- switch model and profile mid-session
- compact and continue long conversations
- rewind and resume session

### 2.2 Non-Interactive Modes

Must test:

- print text output
- JSON output
- stream event output
- structured output validation
- max-turn and budget enforcement

### 2.3 Anthropic Auth

Must test:

- OAuth login
- API-key profile
- token refresh
- auth expiry during session
- invalid credential recovery

### 2.4 OpenRouter

Must test:

- API-key profile registration
- model listing
- arbitrary custom model id selection
- tool calling through adapter normalization
- structured output fallback behavior
- unsupported capability early failure

### 2.5 Builtin Tools

Must test:

- file read, write, edit
- bash and PowerShell
- grep and glob
- web fetch and web search
- task tools
- send-message or subagent flows
- concurrency-safe vs serialized scheduling

### 2.6 MCP

Must test:

- server discovery and connection
- MCP auth flow
- tool execution
- resource listing and reading
- session-expired recovery
- auth failure recovery
- large-output artifact persistence

### 2.7 Remote and Attached Modes

Must test:

- daemon attach
- assistant viewer
- remote control
- SSH-backed session
- direct-connect session
- reconnect after transient disconnect
- viewer-only permission restrictions

### 2.8 Resume and Background Sessions

Must test:

- replay from event log
- background session listing
- attach to background session
- resume after interrupted engine
- replay with artifacts present

### 2.9 Config and Policy

Must test:

- user settings
- project settings
- session overrides
- CLI flag overrides
- managed policy deny and constrain behavior
- blocked project attempts to set routing or secrets

### 2.10 Telemetry and Operational Services

Must test:

- compaction boundaries are emitted and replay-safe
- retry and fallback decisions produce normalized lifecycle events
- budget exhaustion produces typed terminal events
- telemetry redaction removes secrets and sensitive payloads before export
- per-session diagnostic bundle can be produced from stored artifacts and logs

## 3. Failure-Mode Coverage

Required failure categories:

- provider auth expiry
- invalid model
- unsupported capability
- transport disconnect
- MCP session not found
- MCP auth failure
- tool timeout
- hook failure
- replay corruption
- missing artifact on replay
- resume state mismatch

Each failure test must assert:

- normalized error code
- user-facing message
- retryable or terminal classification
- no session corruption

## 4. Benchmarks

Use the current runtime as baseline on the same machine and network. Measure
warm and cold starts separately.

### 4.1 Startup

- interactive warm start to prompt-ready
  - target: 500 ms or less
- interactive cold start to prompt-ready
  - target: 900 ms or less
- headless startup overhead before first provider request
  - target: 150 ms or less

### 4.2 First Token

- local first-token latency on Anthropic
  - target: no regression beyond 5 percent vs current baseline
- local first-token latency on OpenRouter
  - target: within 15 percent of direct provider baseline excluding network

### 4.3 Tool Dispatch

- builtin tool scheduling overhead
  - target: 10 ms or less excluding tool execution time
- MCP tool scheduling overhead
  - target: 50 ms or less excluding network/server time

### 4.4 Memory

- shell + engine combined RSS after 1000-turn synthetic transcript
  - target: 350 MB or less
- engine-only RSS during headless run
  - target: 200 MB or less

### 4.5 Stability

- reconnect success after transient disconnect
  - target: 95 percent or higher across test matrix
- replay success on persisted sessions
  - target: 99 percent or higher for non-corrupt fixtures

## 5. Acceptance Fixtures

Create fixtures for:

- short conversational session
- long tool-heavy session
- MCP-heavy session
- remote attach session
- auth-expired session
- partial stream interrupted mid-turn

Fixtures must be replayable by the engine without calling live providers.

## 6. Release Gates

A phase cannot ship unless:

- scenario tests for that phase pass
- benchmark targets pass or have signed waiver
- failure-mode tests pass
- replay/resume integrity tests pass
