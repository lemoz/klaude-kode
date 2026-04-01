# Harness Workflows

This document describes the operator-facing harness flows that are implemented
today.

These workflows are intentionally offline-first:

- validate a candidate root
- run replay evals
- run benchmark evals
- inspect persisted runs
- diff runs
- rank the current frontier

The harness surface is meant to support external evaluation and future outer
loops without changing the core runtime contracts.

## Candidate Root Expectations

Current candidate validation is lightweight. A candidate root is considered
valid when it contains these paths:

- `cmd/cc/main.go`
- `cmd/cc-engine/main.go`
- `shell/package.json`
- `docs/05-roadmap.md`

Validation commands:

```bash
go run ./cmd/cc-engine -validate-candidate -cwd=/path/to/candidate
go run ./cmd/cc -validate-candidate -cwd=/path/to/candidate
```

Shell:

```text
/validate-candidate
```

## Replay Eval Flow

Use replay eval when you want to score one replay pack against one candidate
root.

CLI:

```bash
go run ./cmd/cc-engine \
  -run-replay-eval \
  -cwd=/path/to/candidate \
  -replay-path=/path/to/replay.json

go run ./cmd/cc \
  -run-replay-eval \
  -cwd=/path/to/candidate \
  -replay-path=/path/to/replay.json
```

Shell:

```text
/run-replay /absolute/path/to/replay.json
```

Output:

- one persisted run artifact
- one appended run index entry
- a score, status, and optional failure summary

## Benchmark Eval Flow

Use benchmark eval when you want to score a candidate against a collection of
replay cases.

CLI:

```bash
go run ./cmd/cc-engine \
  -run-benchmark-eval \
  -cwd=/path/to/candidate \
  -benchmark-path=/path/to/benchmark.json

go run ./cmd/cc \
  -run-benchmark-eval \
  -cwd=/path/to/candidate \
  -benchmark-path=/path/to/benchmark.json
```

Shell:

```text
/run-benchmark /absolute/path/to/benchmark.json
```

Current benchmark behavior:

- weighted score aggregation
- benchmark-level success or failure
- per-case results persisted on the run artifact
- stable failure code `benchmark_cases_failed` when one or more cases fail

## Artifact Layout

Harness output for a candidate root is stored under:

- `.klaude-harness/`

Layout:

```text
.klaude-harness/
  indexes/
    runs.jsonl
  runs/
    <run-id>/
      run.json
```

The current implementation does not yet persist richer reports, derived
frontier snapshots, or external evaluator metadata. The directory contract is
intentionally small for now.

## Run Summary

Summaries provide a quick view over persisted indexed runs for one candidate
root.

CLI:

```bash
go run ./cmd/cc-engine -summarize-runs -cwd=/path/to/candidate
go run ./cmd/cc -summarize-runs -cwd=/path/to/candidate
```

Shell:

```text
/summarize-runs
```

Current summary data:

- total run count
- completed count
- failed count
- average score
- latest run id and status
- aggregated failure-code counts

## Show Run

Use run inspection when you need the full persisted artifact for one run.

CLI:

```bash
go run ./cmd/cc-engine -show-run -cwd=/path/to/candidate -run-id=<run-id>
go run ./cmd/cc -show-run -cwd=/path/to/candidate -run-id=<run-id>
```

Shell:

```text
/show-run <run-id>
```

This is the best command for:

- confirming which replay or benchmark pack was used
- checking benchmark case counts
- checking failure code and retryability
- capturing a run id for later diff/frontier work

## Diff Runs

Diff compares two persisted runs by score, status, failure codes, and case
results.

CLI:

```bash
go run ./cmd/cc-engine \
  -diff-runs \
  -cwd=/path/to/candidate \
  -left-run-id=<left> \
  -right-run-id=<right>

go run ./cmd/cc \
  -diff-runs \
  -cwd=/path/to/candidate \
  -left-run-id=<left> \
  -right-run-id=<right>
```

Shell:

```text
/diff-runs <left-run-id> <right-run-id>
```

Current diff surface includes:

- left/right run ids
- left/right kinds
- left/right statuses
- left/right scores
- score delta
- left/right failure codes
- per-case diffs for benchmark case ids

## Frontier Listing

Frontier is a ranked view over persisted runs for one candidate root.

CLI:

```bash
go run ./cmd/cc-engine -list-frontier -cwd=/path/to/candidate -frontier-limit=5
go run ./cmd/cc -list-frontier -cwd=/path/to/candidate -frontier-limit=5
```

Shell:

```text
/list-frontier
/list-frontier 5
```

Current ranking rule:

1. higher score first
2. newer run first when scores tie

Each frontier entry exposes:

- run id
- kind
- status
- score
- benchmark name when present
- failure code when present

## Repository Fixtures

The repository includes baseline fixtures under
[benchmarks](/Users/cdossman/klaude-kode/benchmarks):

- [pass-basic.json](/Users/cdossman/klaude-kode/benchmarks/replays/pass-basic.json)
- [fail-basic.json](/Users/cdossman/klaude-kode/benchmarks/replays/fail-basic.json)
- [mixed-basic.json](/Users/cdossman/klaude-kode/benchmarks/packs/mixed-basic.json)

These are the current smoke-test fixtures for:

- replay eval
- benchmark eval
- frontier listing
- run diffing

## Current Boundaries

Implemented now:

- local candidate validation
- replay eval
- benchmark eval
- persisted run storage
- summary, show-run, diff, and frontier inspection
- CLI and shell access to all of the above

Not implemented yet:

- richer report generation
- candidate bundles beyond current root validation
- benchmark sharding
- external evaluator hooks
- proposer/frontier mutation loops
- sanitized internal trace ingestion
