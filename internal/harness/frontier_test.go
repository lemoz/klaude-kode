package harness

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestListFrontierSortsByScoreThenRecency(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")

	olderHighScore := EvalRun{
		ID:            "older_high",
		Kind:          EvalRunKindReplay,
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Unix(100, 0).UTC(),
		Status:        EvalRunStatusCompleted,
		Score:         1,
	}
	newerHighScore := EvalRun{
		ID:            "newer_high",
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Unix(200, 0).UTC(),
		Status:        EvalRunStatusCompleted,
		Score:         1,
		Benchmark: &BenchmarkRunMetadata{
			Name: "baseline-pack",
		},
	}
	lowerScore := EvalRun{
		ID:            "lower_score",
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Unix(300, 0).UTC(),
		Status:        EvalRunStatusFailed,
		Score:         0.5,
		Failure: &EvalFailureSummary{
			Code:      "benchmark_cases_failed",
			Message:   "1 benchmark cases failed",
			Retryable: false,
		},
	}

	for _, run := range []EvalRun{olderHighScore, newerHighScore, lowerScore} {
		if _, err := PersistEvalRun(root, run); err != nil {
			t.Fatalf("PersistEvalRun returned error: %v", err)
		}
	}

	entries, err := ListFrontier(root, 2)
	if err != nil {
		t.Fatalf("ListFrontier returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 frontier entries, got %#v", entries)
	}
	if entries[0].RunID != newerHighScore.ID {
		t.Fatalf("expected newer high score first, got %#v", entries)
	}
	if entries[1].RunID != olderHighScore.ID {
		t.Fatalf("expected older high score second, got %#v", entries)
	}
	if entries[0].Benchmark != "baseline-pack" {
		t.Fatalf("expected benchmark metadata on frontier entry, got %#v", entries[0])
	}
}
