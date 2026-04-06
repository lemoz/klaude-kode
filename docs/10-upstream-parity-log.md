# Upstream Parity Log

Track the live upstream Claude Code project as the primary parity reference.
Use this log to record what changed upstream, what we reviewed, and whether
Klaude Kode currently matches, partially matches, diverges, or has not started
that surface yet.

## Status Values

- `matched`
- `partial`
- `divergent`
- `not_started`

## 2026-04-02

### Roadmap Recalibration

Reviewed upstream sources:

- [anthropics/claude-code](https://github.com/anthropics/claude-code)
- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream CHANGELOG](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)
- [plugins directory](https://github.com/anthropics/claude-code/tree/main/plugins)
- [scripts directory](https://github.com/anthropics/claude-code/tree/main/scripts)
- [examples directory](https://github.com/anthropics/claude-code/tree/main/examples)

Key findings:

- plugins are a first-class product surface with commands, agents, hooks, and
  MCP integrations
- MCP and hooks are active product areas with ongoing changelog churn
- install and packaging flows are part of the current user-facing upstream
  surface

Current local parity read:

| Area | Upstream status | Local status | Notes |
| --- | --- | --- | --- |
| Local shell UX | active and user-facing | `partial` | strong shell baseline exists, but parity closure is still in progress |
| Providers/auth | active and user-facing | `partial` | Anthropic and OpenRouter work locally; more parity hardening remains |
| Plugins/hooks | first-class | `not_started` | roadmap now treats this as a major phase |
| MCP | first-class | `not_started` | only contract placeholders exist locally |
| Harness/eval | limited upstream emphasis | `divergent` | intentional Klaude Kode extension beyond upstream |
| Install/distribution | first-class | `not_started` | added as a later roadmap phase |

Next recommended atomic units:

1. update repo status docs after the next real Phase 2 or Phase 3 completion
2. add shell/provider smoke coverage that closes the remaining local UX tail
3. begin Phase 4 with plugin/hook/MCP contracts and status surfaces

### Model Selection Validation and Session Model UX

- Area: model-selection validation and session-local model UX.
- Upstream sources reviewed:
  - `README.md` in `anthropics/claude-code`
  - `CHANGELOG.md` in `anthropics/claude-code`
  - `plugins/` tree in `anthropics/claude-code`
  - `examples/` tree in `anthropics/claude-code`
  - `scripts/` tree in `anthropics/claude-code`
  - Issue `#2532` about `/model` selection behavior for subagents
- Relevant upstream notes:
  - Claude Code exposes `/model` as a first-class interactive control.
  - Recent upstream changelog entries continue hardening model-related UI
    surfaces, including fixes for `/model` selection headers and
    statusline/session model correctness across concurrent sessions.
  - The upstream repo still treats model choice as an interactive session
    concern rather than an ad hoc shell-only preference.
- Local change:
  - Klaude Kode now rejects invalid anthropic model changes during
    `update_session_setting` instead of accepting the setting and failing only
    on the next prompt.
  - OpenRouter custom-model behavior remains intact because provider validation
    still allows provider-specific custom IDs where supported.
- Local status: `partial`.
  - Validation timing now matches the expected engine-owned behavior more
    closely.
  - Richer `/model` UX parity such as dedicated selection views and broader
    session-surface affordances is still not started.

### Phase 2 and Phase 3 Local Parity Closure

Reviewed upstream sources:

- [anthropics/claude-code](https://github.com/anthropics/claude-code)
- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream CHANGELOG](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)

What was verified locally:

- interactive shell flow with help, prompt, grouped transcript, permissions,
  status, profile switching, model switching, and report surfaces
- Anthropic OAuth in-band progress UX and fallback URL visibility
- Anthropic API-key login and live provider turn coverage
- OpenRouter API-key login, profile switching, and custom-model live turn
  coverage
- aggregate shell smoke coverage across `tmux`, permissions, auth, profile,
  model, and provider paths

Updated local parity read:

| Area | Upstream status | Local status | Notes |
| --- | --- | --- | --- |
| Local shell UX | active and user-facing | `matched` | local interactive baseline is complete for the current roadmap scope |
| Providers/auth | active and user-facing | `matched` | Anthropic OAuth/API-key and OpenRouter API-key flows are complete for the current roadmap scope |
| Plugins/hooks | first-class | `not_started` | next execution phase |
| MCP | first-class | `not_started` | next execution phase after plugin/hook contracts |
| Harness/eval | limited upstream emphasis | `divergent` | intentional Klaude Kode extension beyond upstream |
| Install/distribution | first-class | `not_started` | later roadmap phase |

Next recommended atomic units:

1. add plugin manifest contracts and validation helpers
2. add hook event contracts and hook status surfaces
3. add engine-owned MCP lifecycle and status events before tool projection

## 2026-04-05

### Plugin Manifest Contracts and Status Projection

Reviewed upstream sources:

- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream plugins README](https://raw.githubusercontent.com/anthropics/claude-code/main/plugins/README.md)
- [upstream marketplace manifest](https://raw.githubusercontent.com/anthropics/claude-code/main/.claude-plugin/marketplace.json)
- [Issue #9641: plugin manager misses installed plugins](https://github.com/anthropics/claude-code/issues/9641)
- [Issue #9297: marketplace install/discovery hangs](https://github.com/anthropics/claude-code/issues/9297)

Key findings:

- The live upstream README now treats plugins as a first-class public feature of Claude Code.
- The upstream `plugins/README.md` describes a standard plugin layout with `.claude-plugin/plugin.json`, optional `commands/`, `agents/`, `skills/`, `hooks/`, and `.mcp.json`.
- The bundled upstream marketplace manifest carries per-plugin metadata such as `name`, `description`, `version`, `author`, `source`, and `category`.
- Recent plugin issues indicate that manifest completeness and discovery accuracy matter to the user-visible `/plugin` workflow, so local contracts should preserve versioned metadata and contribution inventory instead of deferring everything to ad hoc shell logic.

Local change:

- Klaude Kode now requires plugin `version` metadata in the local manifest contract.
- The plugin package can inspect a plugin root for `commands/`, `agents/`, `skills/`, and `.mcp.json` and project that into the canonical `plugin_status` payload.
- The canonical plugin status payload now carries `version` and `skills` alongside commands, agents, and MCP presence so later engine and shell work can stay projection-only.

Local status: `partial`.

- Manifest metadata and contribution discovery now cover the public upstream plugin layout more directly.
- Hook directory loading, marketplace ingestion, and engine-emitted plugin inventory/status events are still not started.
