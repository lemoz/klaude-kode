package harness

import (
	"bufio"
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

func EvalRunIndexPath(root string) string {
	return filepath.Join(root, DirIndexes, RunIndexFile)
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
	if err := appendEvalRunIndex(root, run); err != nil {
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

func ListIndexedEvalRuns(root string) ([]EvalRun, error) {
	file, err := os.Open(EvalRunIndexPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	runs := make([]EvalRun, 0, 16)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var run EvalRun
		if err := json.Unmarshal(line, &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

func appendEvalRunIndex(root string, run EvalRun) error {
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(EvalRunIndexPath(root), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
