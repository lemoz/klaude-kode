# Compatibility Matrix

## 1. Policy

Compatibility target is behavioral parity. Preserve what users experience,
simplify what is internally accidental, and drop what is internal-only or
operationally specific to Anthropic.

UI/UX parity policy:

- preserve the recognizable interaction model and workflow shape
- do not require exact visual reproduction
- apply a distinct Klaude Kode rebrand in copy, naming, and styling
- treat transcript layout, prompt/status context, permission timing, and help
  discoverability as first-class parity surfaces

## 2. Preserve

| Area | Current Anchor | Decision | Notes |
| --- | --- | --- | --- |
| Local interactive REPL | `src/screens/REPL.tsx` | preserve | Same core UX: prompt, streaming output, tools, permissions, resume |
| Transcript and status context | `src/components/App.tsx`, `src/screens/REPL.tsx` | preserve | Prompt rhythm, streamed output shape, visible session context, terminal state cues |
| Headless print/json/stream modes | `src/main.tsx`, `src/cli/print.ts` | preserve | Same user-facing modes with engine-native event streaming |
| Turn loop semantics | `src/query.ts` | preserve | Compaction, tool loop, retries, budget, turn terminality |
| Builtin tools | `src/tools` | preserve | Same behavioral tool surface, reorganized under `ToolRuntime` |
| MCP tools/resources/auth | `src/services/mcp/client.ts` | preserve | First-class subsystem under engine |
| Permissions UX | `src/components/permissions/*` | preserve | Allow, deny, session-scoped approval, worker-safe behavior |
| Resume/session history | `src/history.ts`, resume flows in `src/main.tsx` | preserve | Replay-driven instead of ad hoc restore state |
| Remote/assistant/direct-connect/SSH | `src/main.tsx`, `src/remote/*` | preserve | Same user concepts, unified transport model |
| Slash commands | `src/commands.ts` | preserve | Major user-facing commands remain |
| Help and discoverability | `src/commands/help`, command/status surfaces | preserve | Common workflows must stay learnable without reading source or hidden docs |
| Hooks/skills/plugins | `src/commands/hooks`, `src/skills`, `src/plugins` | preserve | Same top-level concepts with cleaner manifest/runtime split |
| Config precedence and managed policy | `src/utils/managedEnv.ts`, `src/services/policyLimits` | preserve | Same user expectations, simpler implementation |
| Telemetry, retries, compaction, budgets | `src/query.ts`, `src/services/api/*`, `src/services/analytics/*` | preserve | Same behaviorally visible outcomes, moved behind engine services |

## 3. Simplify

| Area | Current Shape | New Shape |
| --- | --- | --- |
| Boot path | `main.tsx` multiplexes everything | launcher plus engine modes plus thin shell |
| App state | giant UI-owned state tree | engine-owned session state plus shell view state |
| Provider routing | env-heavy and Anthropic-centric | explicit provider profiles and adapter contracts |
| Tool orchestration | mixed runtime and UI concerns | engine-owned scheduling and permission lifecycle |
| Session restore | mixed snapshots, logs, and transport-specific state | append-only replay log plus reducer rebuild |
| MCP integration | large single client integration file | supervisor, auth manager, projection layer |
| Command registration | large mutable list gated by many feature flags | declarative command registry and capability-driven enablement |
| Feature flags | heavy runtime branching | narrow rollout flags around whole subsystems only |
| Model behavior | many hardcoded first-party assumptions | capability matrix checked by engine |

## 4. Drop

These are explicitly not required for parity:

| Area | Reason |
| --- | --- |
| Internal Anthropic-only commands such as `ant-trace`, `backfill-sessions`, `bughunter`, `good-claude`, `mock-limits` | internal operational value only |
| Ant-only rollout hooks and hidden staging behaviors | not part of end-user contract |
| Anthropic SDK block types as core runtime types | implementation detail to be removed |
| Environment-variable quirks whose only purpose was rollout compatibility | replace with explicit profile and policy configuration |
| UI-level ownership of session truth | replaced with engine-owned state |
| Exact Claude Code visual styling | replaced with Klaude Kode branding over similar workflow patterns |
| Pixel-perfect recreation of Claude Code spacing, tokens, or micro-layout | preserve workflow feel instead of exact visual duplication |

## 5. Slash Command Policy

### 5.1 Keep

Keep these concept groups behaviorally:

- session: `resume`, `rewind`, `clear`, `export`, `session`
- model and output: `model`, `effort`, `output-style`, `statusline`
- tools and permissions: `permissions`, `plan`, `tasks`, `memory`, `mcp`
- integration: `plugin`, `skills`, `hooks`, `ide`
- remote and transport: `teleport`, `remote-env`, `remote-setup`, `bridge`
- workflow: `review`, `diff`, `files`, `doctor`, `help`, `usage`

### 5.2 Collapse

Collapse low-value or overlapping commands into a smaller set when possible:

- multiple internal diagnostics commands
- legacy aliases that only exist for migration
- provider-specific escape hatches replaced by explicit profile control

## 6. Tool Policy

### 6.1 Preserve

- file read, write, edit
- bash and PowerShell
- grep and glob
- skill invocation
- task tools
- agent/send-message tools
- MCP resource tools
- web fetch and web search

### 6.2 Simplify

- concurrency safety becomes a first-class tool property in Go
- shell-specific rendering of tool progress becomes a projection of structured
  tool events
- large outputs move to artifact references

## 7. Config Compatibility

### 7.1 Preserve

- user-level settings
- project-level settings
- managed policy ceilings
- model and session defaults
- explicit flag overrides

### 7.2 Simplify

- routing and auth-sensitive settings become profile-driven
- project scope cannot redefine provider routing or secrets
- config import from current Claude Code is one-time and explicit

## 8. Remote and Resume Compatibility

Preserve behavior:

- attach to running remote session
- viewer-style assistant mode
- reconnect and resume after transport failure
- background or detached sessions
- remote permission prompt handling

Simplify implementation:

- all remote paths use the same event protocol
- replay and transport attach use the same session reducers

## 9. Compatibility Exit Criteria

Behavioral parity is considered reached when:

- the common local workflows are usable without reference to the old runtime
- headless print/json/stream users can switch without scripting breakage
- tool and permission behavior matches user expectations
- MCP-heavy sessions work without the old TS runtime
- remote, SSH, direct-connect, and viewer flows operate on the new transport
- importing config/history from the current system succeeds without destructive
  migration
- the shell feels familiar in flow while remaining visibly and verbally its own
