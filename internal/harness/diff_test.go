package harness

import (
	"path/filepath"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestDiffPersistedEvalRunsReportsTopLevelAndCaseDeltas(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")

	left := EvalRun{
		ID:            "run_left",
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusFailed,
		Score:         0.5,
		CaseResults: []BenchmarkCaseResult{
			{
				ID:     "case_a",
				Status: EvalRunStatusCompleted,
				Score:  1,
			},
			{
				ID:     "case_b",
				Status: EvalRunStatusFailed,
				Score:  0,
				Failure: &EvalFailureSummary{
					Code:      "replay_terminal_outcome",
					Message:   "replay pack terminal outcome is task_failure",
					Retryable: false,
				},
			},
		},
		Failure: &EvalFailureSummary{
			Code:      "benchmark_cases_failed",
			Message:   "1 benchmark cases failed",
			Retryable: false,
		},
	}
	right := EvalRun{
		ID:            "run_right",
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusCompleted,
		Score:         1,
		CaseResults: []BenchmarkCaseResult{
			{
				ID:     "case_a",
				Status: EvalRunStatusCompleted,
				Score:  1,
			},
			{
				ID:     "case_b",
				Status: EvalRunStatusCompleted,
				Score:  1,
			},
		},
	}

	if _, err := PersistEvalRun(root, left); err != nil {
		t.Fatalf("PersistEvalRun left returned error: %v", err)
	}
	if _, err := PersistEvalRun(root, right); err != nil {
		t.Fatalf("PersistEvalRun right returned error: %v", err)
	}

	diff, err := DiffPersistedEvalRuns(root, left.ID, right.ID)
	if err != nil {
		t.Fatalf("DiffPersistedEvalRuns returned error: %v", err)
	}
	if diff.ScoreDelta != 0.5 {
		t.Fatalf("expected score delta 0.5, got %#v", diff)
	}
	if diff.LeftFailureCode != "benchmark_cases_failed" || diff.RightFailureCode != "" {
		t.Fatalf("expected failure code transition, got %#v", diff)
	}
	if len(diff.CaseDiffs) != 2 {
		t.Fatalf("expected 2 case diffs, got %#v", diff)
	}
	if diff.CaseDiffs[1].ID != "case_b" || diff.CaseDiffs[1].ScoreDelta != 1 {
		t.Fatalf("expected case_b to improve by 1, got %#v", diff.CaseDiffs[1])
	}
}
