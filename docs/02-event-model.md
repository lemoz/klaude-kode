# Canonical Event Model

## 1. Purpose

The rewrite uses one canonical command/event model across:

- local shell to local engine
- headless mode
- local daemon attach
- remote viewer/control
- SSH/direct-connect sessions
- replay logs

The event model replaces today’s mixture of Anthropic SDK blocks, shell state
mutations, and transport-specific callbacks.

## 2. Envelope

Every message is wrapped in a versioned envelope:

```json
{
  "schema_version": "v1",
  "session_id": "sess_123",
  "sequence": 418,
  "timestamp": "2026-03-31T14:30:12.123Z",
  "payload": {}
}
```

Rules:

- `sequence` is strictly increasing per session.
- replay order is envelope order.
- all commands and events are idempotent by `(session_id, sequence)` when
  replayed or retried.

## 3. SessionCommand Union

### 3.1 Commands

```ts
type SessionCommand =
  | StartSessionCommand
  | UserInputCommand
  | CancelTurnCommand
  | ApprovePermissionCommand
  | DenyPermissionCommand
  | UpdateSessionSettingCommand
  | ExecuteSlashCommand
  | AttachViewerCommand
  | DetachViewerCommand
  | ResumeSessionCommand
  | RefreshMcpCommand
  | CloseSessionCommand
```

### 3.2 Chosen Semantics

- `StartSessionCommand`
  - starts a session with cwd, transport role, profile, model defaults, and
    mode flags
- `UserInputCommand`
  - normalized user message or structured print-mode input
- `CancelTurnCommand`
  - requests cancellation of the active provider turn and any cancellable tool
    subtree
- `ApprovePermissionCommand` and `DenyPermissionCommand`
  - satisfy a specific permission request id
- `UpdateSessionSettingCommand`
  - mutates session-only settings such as model, effort, or output mode
- `ExecuteSlashCommand`
  - executes a command already normalized by the shell router
- `AttachViewerCommand`
  - adds a read-only observer to a session
- `ResumeSessionCommand`
  - rebuilds session runtime state from replay storage
- `RefreshMcpCommand`
  - reconnects MCP servers or reloads server inventory
- `CloseSessionCommand`
  - ends the session and flushes state

## 4. SessionEvent Union

```ts
type SessionEvent =
  | SessionStartedEvent
  | LifecycleEvent
  | UserMessageAcceptedEvent
  | AssistantDeltaEvent
  | AssistantMessageEvent
  | ToolCallRequestedEvent
  | ToolCallProgressEvent
  | ToolCallCompletedEvent
  | PermissionRequestedEvent
  | PermissionResolvedEvent
  | AuthStatusEvent
  | McpStatusEvent
  | RemoteConnectionEvent
  | SessionStateEvent
  | ArtifactCreatedEvent
  | WarningEvent
  | FailureEvent
  | SessionClosedEvent
```

## 5. Required Event Payloads

### 5.1 User and Assistant

- `UserMessageAcceptedEvent`
  - normalized content
  - source `interactive | print | remote | replay`
  - turn id
- `AssistantDeltaEvent`
  - incremental text delta
  - optional reasoning delta
  - provider message id
  - turn id
- `AssistantMessageEvent`
  - final normalized assistant message
  - usage summary
  - provider request ids
  - cache or reasoning metadata

### 5.2 Tool Lifecycle

- `ToolCallRequestedEvent`
  - tool call id
  - tool name
  - normalized input
  - concurrency class
- `ToolCallProgressEvent`
  - progress phase
  - optional progress text
  - optional structured counters
- `ToolCallCompletedEvent`
  - final status
  - result summary
  - artifact references when output is large

### 5.3 Permissions

- `PermissionRequestedEvent`
  - permission request id
  - tool call id
  - policy source
  - prompt text
  - requested scope
- `PermissionResolvedEvent`
  - request id
  - resolved behavior `allow | deny | allow_once | allow_session`
  - actor `user | policy | auto`

### 5.4 Auth and MCP

- `AuthStatusEvent`
  - profile id
  - provider kind
  - status `ready | refreshing | expired | invalid | needs_login`
  - human-readable message
- `McpStatusEvent`
  - server id
  - status `connecting | ready | needs_auth | failed | closed`
  - optional auth dependency

### 5.5 Remote Connectivity

- `RemoteConnectionEvent`
  - target
  - status `connecting | connected | reconnecting | disconnected`
  - reconnect attempt count

### 5.6 Failures

- `FailureEvent`
  - category `provider | tool | permission | transport | auth | mcp | replay`
  - normalized code
  - user-facing message
  - retryable flag

## 6. Lifecycle Semantics

- a turn starts when the engine accepts user input
- all provider, tool, and permission work is attached to that turn id
- a turn is terminal only after:
  - provider completion emitted
  - all spawned tool work is resolved
  - post-turn bookkeeping is persisted

The shell does not infer terminality from transport closure or missing deltas.

## 7. Replay Semantics

- replay source of truth is `events.jsonl`
- engine reconstructs session state by reducing events
- shell replay mode is read-only and renders from event projections
- non-deterministic external results are persisted as artifacts and referenced
  by event id

## 8. Artifact Handling

Large outputs are externalized:

- patch or diff blobs
- large file reads
- screenshots
- MCP binary output
- verbose tool stderr

Artifact reference:

```json
{
  "artifact_id": "art_123",
  "kind": "tool_output",
  "mime_type": "text/plain",
  "sha256": "..."
}
```

## 9. Idempotency Rules

- retries of command delivery must include the same `command_id`
- permission resolutions are idempotent by request id
- tool completion emits exactly one terminal event per tool call id
- replay never re-executes tools or provider calls

## 10. Event Reduction Model

The engine owns reducers for:

- session summary
- current message transcript
- active tool calls
- permission queue
- MCP status board
- auth status board
- background task board

The shell consumes reduced snapshots and event deltas. It does not reimplement
session logic.

