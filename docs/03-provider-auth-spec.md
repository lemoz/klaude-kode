# Provider and Auth Specification

## 1. Summary

The rewrite supports these provider and auth profile kinds from day one:

- `anthropic_oauth`
- `anthropic_api_key`
- `openrouter_api_key`

Provider identity and auth identity are explicit. OpenRouter is never modeled
as Anthropic with a custom base URL.

## 2. Provider Kinds

Chosen provider enum:

```ts
type ProviderKind =
  | "anthropic"
  | "openrouter"
```

Cloud-provider passthroughs such as Bedrock, Vertex, and Foundry are deferred
to later adapters and are not part of the initial first-class blueprint target.
The design leaves room for them but does not require day-one implementation.

## 3. AuthProfile Schema

Profiles live in `~/.claude-next/profiles/profiles.json`.

Chosen schema:

```json
{
  "profiles": [
    {
      "id": "anthropic-main",
      "kind": "anthropic_oauth",
      "display_name": "Anthropic Main",
      "provider": "anthropic",
      "default_model": "claude-sonnet-4-6",
      "settings": {
        "account_scope": "claude",
        "oauth_host": "https://claude.ai"
      }
    },
    {
      "id": "openrouter-main",
      "kind": "openrouter_api_key",
      "display_name": "OpenRouter Main",
      "provider": "openrouter",
      "default_model": "anthropic/claude-sonnet-4.5",
      "settings": {
        "api_base": "https://openrouter.ai/api/v1",
        "app_name": "Claude Code Next",
        "http_referer": "https://local.cli"
      }
    }
  ],
  "default_profile_id": "anthropic-main"
}
```

Secrets are stored separately in secure storage or OS keychain. `profiles.json`
contains references and non-secret configuration only.

## 4. Credential Storage

### 4.1 Anthropic OAuth

- access token and refresh token in secure storage
- profile record stores metadata only
- engine owns refresh and expiry handling

### 4.2 Anthropic API Key

- API key in secure storage
- profile record stores model defaults and account labeling only

### 4.3 OpenRouter API Key

- API key in secure storage
- profile stores base URL, app metadata headers, and default model

## 5. Profile Resolution Rules

Chosen resolution order:

1. CLI `--profile`
2. session override from `/profile`
3. project default profile
4. user default profile

Managed policy may deny a resolved profile or constrain allowed provider kinds,
but it does not silently swap one profile for another.

## 6. Model Resolution Rules

Chosen order:

1. CLI `--model`
2. session override from `/model`
3. project default model
4. profile default model
5. provider default model

Provider default models:

- Anthropic
  - default interactive: `claude-sonnet-4-6`
  - high-reasoning fallback: `claude-opus-4-6`
- OpenRouter
  - default interactive: `anthropic/claude-sonnet-4.5`
  - high-reasoning fallback: `anthropic/claude-opus-4.1`

The engine accepts arbitrary model ids. Validation is adapter-owned:

- if provider supports model listing, validate against list
- if provider does not, allow explicit model ids and validate on first request
- preserve original case for custom ids

## 7. Provider Capability Matrix

Chosen v1 matrix:

| Capability | Anthropic OAuth | Anthropic API Key | OpenRouter API Key |
| --- | --- | --- | --- |
| Streaming completion | yes | yes | yes |
| Non-stream completion | yes | yes | yes |
| Tool calling | yes | yes | yes, adapter-normalized |
| Structured outputs | yes | yes | yes when model advertises support, else client validation fallback |
| Token counting | exact API count | exact API count | estimate by adapter unless exact count endpoint exists |
| Prompt caching | yes | yes | no in v1 |
| Reasoning/thinking controls | native | native | mapped best effort, else disabled |
| Deferred tool references/tool search | yes | yes | no in v1 |
| Image input | yes | yes | yes when model supports multimodal |
| Document input | yes | yes | text extraction fallback in v1 |
| Model listing | yes | yes | yes |
| Auth refresh | OAuth refresh | n/a | n/a |

## 8. ProviderAdapter Translation Policy

### 8.1 Canonical Request

The engine builds one provider-neutral request:

```go
type CompletionRequest struct {
    TurnID           string
    Model            string
    Messages         []CanonicalMessage
    SystemPrompt     []string
    Tools            []ToolDescriptor
    ToolChoice       ToolChoice
    OutputSchema     *JSONSchema
    Reasoning        ReasoningConfig
    Budget           BudgetConfig
    Attachments      []AttachmentRef
}
```

### 8.2 Anthropic

Anthropic adapter maps canonical requests to Anthropic-native fields:

- native tool schemas
- native structured outputs
- native reasoning/thinking
- native prompt caching

### 8.3 OpenRouter

OpenRouter adapter maps canonical requests conservatively:

- tool descriptors normalized to OpenAI-compatible tool schema
- structured outputs passed where supported, otherwise validated client-side
- reasoning config mapped to provider-specific request fields only when model
  capability says supported
- no prompt-caching assumptions
- no `tool_reference` or deferred-tool reference support in v1

## 9. Auth UX

Chosen UX model: separate provider profiles.

### 9.1 User Commands

- `/profile`
  - switch active profile for the session
- `/login anthropic`
  - create or refresh Anthropic OAuth or API-key profile
- `/login openrouter`
  - register an OpenRouter API-key profile
- `/profiles`
  - list profiles and defaults

### 9.2 CLI Flags

- `--profile <id>`
- `--model <id>`
- `--provider anthropic|openrouter`
  - optional convenience flag that resolves to the default profile for that
    provider if `--profile` is absent

If both `--profile` and `--provider` are set, `--profile` wins.

## 10. Config and Project-Scope Policy

Project settings may define:

- default profile id
- default model
- session behavior defaults
- allow/deny lists for tools and MCP servers

Project settings may not define:

- raw credentials
- auth host
- provider base URL
- routing overrides for protected profiles

This prevents hostile repos from hijacking provider/auth state.

## 11. Failure Modes

### 11.1 Auth Expiry

- engine emits `AuthStatusEvent(needs_login|expired)`
- active turn fails with a normalized retryable auth error
- shell renders a provider-specific recovery action

### 11.2 Invalid Model

- adapter returns normalized `invalid_model`
- shell suggests model switch within the active profile
- session remains usable

### 11.3 Capability Mismatch

Examples:

- structured output requested on unsupported model
- tool calling requested on unsupported model
- reasoning config requested on unsupported model

Chosen behavior:

- engine checks capability matrix before provider call
- fail early with typed error instead of leaking provider-specific 400s

### 11.4 Misconfiguration

- bad OpenRouter key
- bad Anthropic OAuth refresh token
- conflicting project default profile and managed policy

Chosen behavior:

- `AuthStatusEvent(invalid)`
- no silent fallback to another provider
- user must explicitly switch or repair the active profile

