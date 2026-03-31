package harness

import (
	"os"
	"path/filepath"
	"testing"
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
