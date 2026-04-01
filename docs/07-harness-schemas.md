# Harness Schemas

This document defines the file formats that the current harness surface reads
and writes.

The intent is stability for:

- offline replay evaluation
- offline benchmark evaluation
- external tooling that inspects `.klaude-harness/` artifacts

## Replay Pack

Replay packs are exported from persisted sessions with:

- `cc-engine -export-replay-pack`
- `cc -export-replay-pack`
- `/export-replay <path>` in the shell

Schema:

```json
{
  "schema_version": "v1",
  "exported_at": "2026-03-31T20:00:00Z",
  "session": {
    "session_id": "session-id",
    "cwd": "/absolute/candidate/root",
    "mode": "interactive",
    "profile_id": "anthropic-main",
    "model": "claude-sonnet-4-6",
    "created_at": "2026-03-31T19:58:00Z"
  },
  "summary": {
    "session_id": "session-id",
    "cwd": "/absolute/candidate/root",
    "mode": "interactive",
    "status": "closed",
    "profile_id": "anthropic-main",
    "model": "claude-sonnet-4-6",
    "created_at": "2026-03-31T19:58:00Z",
    "updated_at": "2026-03-31T20:00:00Z",
    "event_count": 12,
    "turn_count": 1,
    "last_sequence": 12,
    "last_event_kind": "session_closed",
    "closed_reason": "shell_exit",
    "terminal_outcome": "success"
  },
  "events": []
}
```

Required properties:

- `schema_version`
- `exported_at`
- `session`
- `summary`
- `events`

Contract notes:

- `events` is the canonical replay payload.
- `summary.terminal_outcome` is the value used by harness replay evaluation.
- replay packs are provider-neutral. Provider-specific payloads must already be
  normalized into canonical session events before export.
- replay packs are expected to be deterministic artifacts. They should not rely
  on live provider calls during eval.

## Benchmark Pack

Benchmark packs are loaded by:

- `cc-engine -run-benchmark-eval`
- `cc -run-benchmark-eval`
- `/run-benchmark <path>` in the shell

Schema:

```json
{
  "schema_version": "v1",
  "name": "mixed-basic",
  "description": "One passing replay and one failing replay.",
  "cases": [
    {
      "id": "case-pass",
      "replay_path": "../replays/pass-basic.json",
      "weight": 1
    },
    {
      "id": "case-fail",
      "replay_path": "../replays/fail-basic.json",
      "weight": 1
    }
  ]
}
```

Required properties:

- `schema_version`
- `name`
- `cases`

Per-case requirements:

- `id`
- `replay_path`
- `weight`

Contract notes:

- `replay_path` is resolved relative to the benchmark pack file location.
- `weight` contributes to aggregate benchmark score.
- benchmark packs are collections of replay packs. They do not embed provider
  credentials or live model instructions.
- current repository fixtures live under [benchmarks](/Users/cdossman/klaude-kode/benchmarks).

## Persisted Eval Run

Each replay or benchmark execution persists one eval run artifact at:

- `.klaude-harness/runs/<run-id>/run.json`

Schema shape:

```json
{
  "id": "run_1775000000000000000",
  "kind": "benchmark",
  "schema_version": "v1",
  "created_at": "2026-03-31T20:10:00Z",
  "candidate": {
    "schema_version": "v1",
    "created_at": "2026-03-31T20:09:00Z",
    "root": "/absolute/candidate/root",
    "engine_version": "cc-engine",
    "shell_version": "cc-shell",
    "default_profile_id": "anthropic-main",
    "default_model": "claude-sonnet-4-6"
  },
  "replay_path": "",
  "benchmark": {
    "name": "mixed-basic",
    "description": "One passing replay and one failing replay.",
    "path": "/absolute/path/to/mixed-basic.json",
    "case_count": 2
  },
  "status": "failed",
  "score": 0.5,
  "case_results": [],
  "failure": {
    "code": "benchmark_cases_failed",
    "message": "1 benchmark cases failed",
    "retryable": false
  }
}
```

Contract notes:

- `kind` is `replay` or `benchmark`.
- replay runs set `replay_path`; benchmark runs set `benchmark`.
- benchmark runs may include `case_results`; replay runs may leave
  `case_results` empty.
- `failure.code` is the stable classification surface used by summaries and
  diffs.

## Indexed Run Summary Data

The artifact store also maintains:

- `.klaude-harness/indexes/runs.jsonl`

That index is append-only and is the source for:

- run summaries
- frontier listings
- persisted run diffs

The JSONL format is intentionally simple so external tooling can inspect it
without linking against Klaude Kode internals.
