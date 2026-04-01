package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestEnsureArtifactRootCreatesRequiredDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")

	if err := EnsureArtifactRoot(root); err != nil {
		t.Fatalf("EnsureArtifactRoot returned error: %v", err)
	}

	for _, dir := range RequiredArtifactDirs() {
		fullPath := filepath.Join(root, dir)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", fullPath, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", fullPath)
		}
	}
}

func TestPersistEvalRunWritesRunArtifact(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	run := EvalRun{
		ID:            "run_test_1",
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusCompleted,
		Score:         1,
		Candidate: CandidateBundle{
			Root: filepath.Join(t.TempDir(), "candidate"),
		},
	}

	path, err := PersistEvalRun(root, run)
	if err != nil {
		t.Fatalf("PersistEvalRun returned error: %v", err)
	}
	if path != EvalRunPath(root, run.ID) {
		t.Fatalf("expected run path %s, got %s", EvalRunPath(root, run.ID), path)
	}

	loaded, err := LoadEvalRun(root, run.ID)
	if err != nil {
		t.Fatalf("LoadEvalRun returned error: %v", err)
	}
	if loaded.ID != run.ID {
		t.Fatalf("expected run id %s, got %s", run.ID, loaded.ID)
	}
	if loaded.Status != EvalRunStatusCompleted {
		t.Fatalf("expected persisted status %s, got %s", EvalRunStatusCompleted, loaded.Status)
	}
}

func TestPersistEvalRunAppendsRunIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	first := EvalRun{
		ID:            "run_test_1",
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusCompleted,
		Score:         1,
	}
	second := EvalRun{
		ID:            "run_test_2",
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusFailed,
		Score:         0,
		Failure: &EvalFailureSummary{
			Code:      "replay_terminal_outcome",
			Message:   "replay pack terminal outcome is task_failure",
			Retryable: false,
		},
	}

	if _, err := PersistEvalRun(root, first); err != nil {
		t.Fatalf("PersistEvalRun first returned error: %v", err)
	}
	if _, err := PersistEvalRun(root, second); err != nil {
		t.Fatalf("PersistEvalRun second returned error: %v", err)
	}

	runs, err := ListIndexedEvalRuns(root)
	if err != nil {
		t.Fatalf("ListIndexedEvalRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 indexed runs, got %d", len(runs))
	}
	if runs[0].ID != first.ID || runs[1].ID != second.ID {
		t.Fatalf("expected indexed runs in append order, got %#v", runs)
	}
	if runs[1].Failure == nil || runs[1].Failure.Code != second.Failure.Code {
		t.Fatalf("expected failure details in indexed run, got %#v", runs[1])
	}
}

func TestPersistEvalRunRoundTripsBenchmarkMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "artifacts")
	run := EvalRun{
		ID:            "benchmark_run_1",
		Kind:          EvalRunKindBenchmark,
		SchemaVersion: contracts.SchemaVersionV1,
		Status:        EvalRunStatusFailed,
		Score:         0.5,
		Benchmark: &BenchmarkRunMetadata{
			Name:        "baseline-pack",
			Description: "baseline replay benchmark pack",
			Path:        "/tmp/benchmark.json",
			CaseCount:   2,
		},
		CaseResults: []BenchmarkCaseResult{
			{
				ID:         "case_one",
				ReplayPath: "/tmp/replays/case-one.json",
				Weight:     1,
				Status:     EvalRunStatusCompleted,
				Score:      1,
			},
			{
				ID:         "case_two",
				ReplayPath: "/tmp/replays/case-two.json",
				Weight:     1,
				Status:     EvalRunStatusFailed,
				Score:      0,
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

	if _, err := PersistEvalRun(root, run); err != nil {
		t.Fatalf("PersistEvalRun returned error: %v", err)
	}

	loaded, err := LoadEvalRun(root, run.ID)
	if err != nil {
		t.Fatalf("LoadEvalRun returned error: %v", err)
	}
	if loaded.Kind != EvalRunKindBenchmark {
		t.Fatalf("expected benchmark run kind, got %#v", loaded)
	}
	if loaded.Benchmark == nil || loaded.Benchmark.Name != run.Benchmark.Name {
		t.Fatalf("expected benchmark metadata to round-trip, got %#v", loaded)
	}
	if len(loaded.CaseResults) != 2 {
		t.Fatalf("expected case results to round-trip, got %#v", loaded)
	}
	if loaded.CaseResults[1].Failure == nil || loaded.CaseResults[1].Failure.Code != "replay_terminal_outcome" {
		t.Fatalf("expected case failure to round-trip, got %#v", loaded)
	}
}
