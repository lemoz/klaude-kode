# Upstream Parity Log

## 2026-04-02

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
  - Recent upstream changelog entries continue hardening model-related UI surfaces, including fixes for `/model` selection headers and statusline/session model correctness across concurrent sessions.
  - The upstream repo still treats model choice as an interactive session concern rather than an ad hoc shell-only preference.
- Local change:
  - Klaude Kode now rejects invalid anthropic model changes during `update_session_setting` instead of accepting the setting and failing only on the next prompt.
  - OpenRouter custom-model behavior remains intact because provider validation still allows provider-specific custom IDs where supported.
- Local status: partial.
  - Validation timing now matches the expected engine-owned behavior more closely.
  - Richer `/model` UX parity such as dedicated selection views and broader session-surface affordances is still not started.
