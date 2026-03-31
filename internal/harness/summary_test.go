package harness

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestSummarizeIndexedEvalRunsAggregatesRunIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	first := EvalRun{
		ID:            "run_one",
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Unix(100, 0).UTC(),
		Status:        EvalRunStatusCompleted,
		Score:         1,
	}
	second := EvalRun{
		ID:            "run_two",
		SchemaVersion: contracts.SchemaVersionV1,
		CreatedAt:     time.Unix(200, 0).UTC(),
		Status:        EvalRunStatusFailed,
		Score:         0,
		Failure: &EvalFailureSummary{
			Code:      "invalid_replay_pack",
			Message:   "replay pack is missing required session data",
			Retryable: false,
		},
	}

	if _, err := PersistEvalRun(root, first); err != nil {
		t.Fatalf("PersistEvalRun first returned error: %v", err)
	}
	if _, err := PersistEvalRun(root, second); err != nil {
		t.Fatalf("PersistEvalRun second returned error: %v", err)
	}

	summary, err := SummarizeIndexedEvalRuns(root)
	if err != nil {
		t.Fatalf("SummarizeIndexedEvalRuns returned error: %v", err)
	}
	if summary.TotalRuns != 2 {
		t.Fatalf("expected 2 runs, got %#v", summary)
	}
	if summary.Completed != 1 || summary.Failed != 1 {
		t.Fatalf("expected 1 completed and 1 failed run, got %#v", summary)
	}
	if summary.AverageScore != 0.5 {
		t.Fatalf("expected average score 0.5, got %#v", summary)
	}
	if summary.LatestRunID != second.ID || summary.LatestStatus != second.Status {
		t.Fatalf("expected latest run %s/%s, got %#v", second.ID, second.Status, summary)
	}
	if summary.FailureCodes["invalid_replay_pack"] != 1 {
		t.Fatalf("expected failure code count, got %#v", summary.FailureCodes)
	}
}
