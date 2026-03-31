package harness

import (
	"encoding/json"
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

func DefaultArtifactRoot(candidateRoot string) string {
	return filepath.Join(candidateRoot, DefaultArtifactDirName)
}

func EvalRunDir(root string, runID string) string {
	return filepath.Join(root, DirRuns, runID)
}

func EvalRunPath(root string, runID string) string {
	return filepath.Join(EvalRunDir(root, runID), RunMetadataFile)
}

func PersistEvalRun(root string, run EvalRun) (string, error) {
	if err := EnsureArtifactRoot(root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(EvalRunDir(root, run.ID), 0o755); err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return "", err
	}

	path := EvalRunPath(root, run.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func LoadEvalRun(root string, runID string) (EvalRun, error) {
	data, err := os.ReadFile(EvalRunPath(root, runID))
	if err != nil {
		return EvalRun{}, err
	}

	var run EvalRun
	if err := json.Unmarshal(data, &run); err != nil {
		return EvalRun{}, err
	}
	return run, nil
}
