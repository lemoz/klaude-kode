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
