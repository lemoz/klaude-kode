package harness

import (
	"os"
	"path/filepath"
)

func EnsureArtifactRoot(root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	for _, dir := range RequiredArtifactDirs() {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}
