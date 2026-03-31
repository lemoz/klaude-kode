package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCandidateRootAcceptsExpectedRepoShape(t *testing.T) {
	root := t.TempDir()

	for _, path := range []string{
		filepath.Join(root, "cmd", "cc"),
		filepath.Join(root, "cmd", "cc-engine"),
		filepath.Join(root, "shell"),
		filepath.Join(root, "docs"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
	}
	for _, file := range []string{
		filepath.Join(root, "cmd", "cc", "main.go"),
		filepath.Join(root, "cmd", "cc-engine", "main.go"),
		filepath.Join(root, "shell", "package.json"),
		filepath.Join(root, "docs", "05-roadmap.md"),
	} {
		if err := os.WriteFile(file, []byte("stub"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}

	result, err := ValidateCandidateRoot(root)
	if err != nil {
		t.Fatalf("ValidateCandidateRoot returned error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid candidate result, got %#v", result)
	}
	if result.Candidate.Root == "" {
		t.Fatalf("expected candidate root to be set")
	}
}

func TestValidateCandidateRootReportsMissingPaths(t *testing.T) {
	root := t.TempDir()

	result, err := ValidateCandidateRoot(root)
	if err != nil {
		t.Fatalf("ValidateCandidateRoot returned error: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid candidate result, got %#v", result)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("expected missing-path issues")
	}
}
