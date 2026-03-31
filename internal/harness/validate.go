package harness

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cdossman/klaude-kode/internal/provider"
)

func ValidateCandidateRoot(root string) (CandidateValidationResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return CandidateValidationResult{}, err
	}

	defaultProfile := provider.ResolveSessionProfile("", "")
	result := CandidateValidationResult{
		Valid: true,
		Candidate: CandidateBundle{
			SchemaVersion:    "v1",
			CreatedAt:        time.Now().UTC(),
			Root:             absRoot,
			EngineVersion:    "cc-engine",
			ShellVersion:     "cc-shell",
			DefaultProfileID: defaultProfile.ID,
			DefaultModel:     defaultProfile.DefaultModel,
		},
	}

	requiredPaths := []string{
		filepath.Join(absRoot, "cmd", "cc", "main.go"),
		filepath.Join(absRoot, "cmd", "cc-engine", "main.go"),
		filepath.Join(absRoot, "shell", "package.json"),
		filepath.Join(absRoot, "docs", "05-roadmap.md"),
	}
	for _, requiredPath := range requiredPaths {
		if _, err := os.Stat(requiredPath); err != nil {
			result.Valid = false
			result.Issues = append(result.Issues, "missing required path: "+requiredPath)
		}
	}

	return result, nil
}
