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
