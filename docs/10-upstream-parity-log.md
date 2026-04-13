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

## 2026-04-13

### Typed Plugin Hook Manifest Inspection

Reviewed upstream sources:

- [upstream README](https://github.com/anthropics/claude-code/blob/main/README.md)
- [upstream CHANGELOG](https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md)
- [upstream plugins README](https://github.com/anthropics/claude-code/blob/main/plugins/README.md)
- [upstream marketplace manifest](https://github.com/anthropics/claude-code/blob/main/.claude-plugin/marketplace.json)
- [upstream `security-guidance` hooks manifest](https://github.com/anthropics/claude-code/blob/main/plugins/security-guidance/hooks/hooks.json)
- [upstream `explanatory-output-style` hooks manifest](https://github.com/anthropics/claude-code/blob/main/plugins/explanatory-output-style/hooks/hooks.json)
- [Issue #10225: plugin `UserPromptSubmit` hooks register but never execute](https://github.com/anthropics/claude-code/issues/10225)
- [Issue #10871: plugin-registered hooks execute twice](https://github.com/anthropics/claude-code/issues/10871)
- [Issue #41943: `/reload-plugins` crashes when marketplace plugins use `hooks/hooks.json`](https://github.com/anthropics/claude-code/issues/41943)
- [PR #12756: manifest-driven plugin loading and missing hook declarations](https://github.com/anthropics/claude-code/pull/12756)

Key findings:

- The live upstream plugin surface treats `hooks/hooks.json` as a typed,
  user-visible contract rather than an opaque folder of scripts.
- Recent upstream fixes and bug reports show that correct hook file loading,
  event recognition, deduplication, and path handling are part of normal
  plugin behavior, not internal implementation details.
- The public bundled plugins use a small, stable hook-manifest shape with a
  top-level `hooks` object keyed by event names and matcher entries containing
  command hooks.

Local change:

- Klaude Kode plugin inspection now loads and validates the standard
  `hooks/hooks.json` file instead of only counting files under `hooks/`.
- `plugin_status` payloads and `cc`/`cc-engine` inspection output now project
  typed hook metadata through `hook_count` plus `hook_events`.
- Validation now reports malformed hook manifests, unsupported event names,
  missing commands, and missing `hooks/hooks.json` when a `hooks/` directory is
  present.

Local status: `partial`.

- Typed hook manifest inspection is now matched closely enough for plugin-state
  projection and validation.
- Hook execution wiring, plugin loader integration, deduplication safeguards,
  and marketplace/plugin runtime loading are still not started.

## 2026-04-09

### Marketplace Manifest Inspection Surface

Reviewed upstream sources:

- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream CHANGELOG](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)
- [upstream plugins README](https://raw.githubusercontent.com/anthropics/claude-code/main/plugins/README.md)
- [upstream marketplace manifest](https://raw.githubusercontent.com/anthropics/claude-code/main/.claude-plugin/marketplace.json)
- [Issue #9641: plugin manager misses installed plugins despite valid on-disk layout](https://github.com/anthropics/claude-code/issues/9641)
- [Issue #10364: marketplace installs skills to the wrong directory](https://github.com/anthropics/claude-code/issues/10364)

Key findings:

- The bundled upstream `.claude-plugin/marketplace.json` is now a public,
  versioned product surface with owner metadata plus per-plugin `source` and
  `category` fields.
- Recent upstream changelog entries include marketplace-facing fixes such as
  `claude plugin update` incorrectly reporting local-marketplace plugins as
  current when remote commits existed, which implies marketplace metadata is
  part of normal user workflows rather than hidden packaging state.
- Current upstream plugin issues still show discovery/install problems when
  marketplace metadata and local filesystem state diverge, so the new runtime
  needs a typed Go-owned marketplace contract before later `/plugin`
  browse/install work.

Local change:

- Klaude Kode now has a typed Go parser, validator, and inspector for
  `.claude-plugin/marketplace.json`, including owner metadata, per-plugin
  source/category fields, duplicate-name checks, and source-path validation.
- `cc-engine -inspect-marketplace` and `cc -inspect-marketplace` now expose the
  canonical marketplace inspection surface with validation issues and
  marketplace summary output.

Local status: `partial`.

- Marketplace manifest metadata is now a concrete Go-owned parity surface.
- Marketplace-driven plugin loading, install/update flows, and engine-emitted
  inventory events are still not started.

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

## 2026-04-06

### Plugin Root Validation and Hook Directory Recognition

Reviewed upstream sources:

- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream CHANGELOG](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)
- [upstream plugins README](https://raw.githubusercontent.com/anthropics/claude-code/main/plugins/README.md)
- [upstream marketplace manifest](https://raw.githubusercontent.com/anthropics/claude-code/main/.claude-plugin/marketplace.json)
- [upstream examples directory](https://github.com/anthropics/claude-code/tree/main/examples)
- [upstream scripts directory](https://github.com/anthropics/claude-code/tree/main/scripts)
- [Issue #9297: marketplace add hangs and validation catches malformed plugin manifest data](https://github.com/anthropics/claude-code/issues/9297)
- [Issue #9641: plugin manager misses installed plugins despite valid on-disk layout](https://github.com/anthropics/claude-code/issues/9641)

Key findings:

- The public upstream plugin docs treat `README.md` as part of the standard plugin root alongside `.claude-plugin/plugin.json`.
- The public plugin layout includes optional `hooks/`, so local discovery should recognize that directory as part of the typed plugin surface even before hook execution is wired through the loader.
- Recent upstream plugin issues show that validation quality and filesystem discovery accuracy are user-visible because `/plugin` flows depend on trustworthy on-disk structure, not shell-only assumptions.
- The upstream examples and scripts trees reinforce that plugin-related UX now spans docs, examples, validation, and install surfaces rather than being an internal-only implementation detail.

Local change:

- Klaude Kode plugin inspection now requires a root `README.md` file and records typed validation issues when it is missing.
- Plugin inspection now recognizes `hooks/` as a first-class plugin contribution root and counts discovered hook files for status projection.
- Malformed plugin contribution layouts such as nested `commands/` directories, non-markdown `agents/` entries, directory-shaped `.mcp.json`, or file-shaped `hooks/`/`skills/` paths now surface as typed validation issues instead of only failing later during loader work.

Local status: `partial`.

- Root validation and hook-directory recognition now better match the upstream plugin packaging contract.
- Engine-emitted plugin inventory events, marketplace ingestion, and full hook manifest loading are still not started.

## 2026-04-07

### Engine-Owned Plugin Inspection Surface

Reviewed upstream sources:

- [upstream README](https://raw.githubusercontent.com/anthropics/claude-code/main/README.md)
- [upstream CHANGELOG](https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md)
- [upstream plugins README](https://raw.githubusercontent.com/anthropics/claude-code/main/plugins/README.md)
- [upstream marketplace manifest](https://raw.githubusercontent.com/anthropics/claude-code/main/.claude-plugin/marketplace.json)
- [Issue #9641: plugin manager misses installed plugins despite valid on-disk layout](https://github.com/anthropics/claude-code/issues/9641)
- [Issue #10225: plugin UserPromptSubmit hooks are registered but never execute](https://github.com/anthropics/claude-code/issues/10225)
- [Issue #10871: plugin-registered hooks are executed twice with different PIDs](https://github.com/anthropics/claude-code/issues/10871)

Key findings:

- The live upstream plugin surface is not just a packaging format; it depends on trustworthy runtime inspection so installed plugins, hook registrations, and contribution inventory are visible to user-facing plugin flows.
- Recent upstream issues show two adjacent user-visible failure modes: plugin managers can miss valid installs, and plugin hook behavior can drift from the on-disk manifest state.
- That makes a typed Go-side inspection surface a prerequisite for later loader and `/plugin` work, because the shell should render plugin state rather than rediscover filesystem facts independently.

Local change:

- Klaude Kode now exposes `-inspect-plugin` in both `cc-engine` and `cc`, using the existing typed plugin inspector and canonical `plugin_status` payload.
- The new surface reports plugin identity, validity, contribution inventory, hook count, MCP presence, and detailed validation issues from the Go runtime.
- README and roadmap status now document plugin inspection as a completed Phase 4 atomic unit.

Local status: `partial`.

- Plugin inspection and status rendering now exist as a concrete Go-owned parity surface.
- Engine-emitted plugin inventory events, marketplace ingestion, and full hook manifest loading are still not started.
