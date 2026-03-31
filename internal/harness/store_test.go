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
