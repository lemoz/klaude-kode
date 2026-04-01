package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestRunBenchmarkEvalAggregatesReplayCases(t *testing.T) {
	candidateRoot := createBenchmarkCandidateRoot(t)
	benchmarkRoot := t.TempDir()

	successReplay := writeReplayPackFile(t, benchmarkRoot, "success.json", contracts.TerminalOutcomeSuccess)
	_ = writeReplayPackFile(t, benchmarkRoot, "failure.json", contracts.TerminalOutcomeTaskFailure)

	pack := BenchmarkPack{
		SchemaVersion: contracts.SchemaVersionV1,
		Name:          "baseline-pack",
		Description:   "baseline replay benchmark pack",
		Cases: []BenchmarkCase{
			{
				ID:         "case_success",
				ReplayPath: filepath.Base(successReplay),
				Weight:     1,
			},
			{
				ID:         "case_failure",
				ReplayPath: "failure.json",
				Weight:     1,
			},
		},
	}

	benchmarkPath := filepath.Join(benchmarkRoot, "benchmark.json")
	encoded, err := json.Marshal(pack)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(benchmarkPath, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	run, err := RunBenchmarkEval(candidateRoot, benchmarkPath)
	if err != nil {
		t.Fatalf("RunBenchmarkEval returned error: %v", err)
	}
	if run.Kind != EvalRunKindBenchmark {
		t.Fatalf("expected benchmark kind, got %#v", run)
	}
	if run.Benchmark == nil || run.Benchmark.Name != pack.Name {
		t.Fatalf("expected benchmark metadata %#v, got %#v", pack, run.Benchmark)
	}
	if len(run.CaseResults) != 2 {
		t.Fatalf("expected 2 case results, got %#v", run)
	}
	if run.Status != EvalRunStatusFailed {
		t.Fatalf("expected failed benchmark due to one failed case, got %#v", run)
	}
	if run.Score != 0.5 {
		t.Fatalf("expected weighted score 0.5, got %#v", run)
	}
	if run.Failure == nil || run.Failure.Code != "benchmark_cases_failed" {
		t.Fatalf("expected benchmark failure summary, got %#v", run)
	}
}

func createBenchmarkCandidateRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"cmd/cc", "cmd/cc-engine", "shell", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
	}
	for _, file := range []string{
		"cmd/cc/main.go",
		"cmd/cc-engine/main.go",
		"shell/package.json",
		"docs/05-roadmap.md",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("stub"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}
	return root
}

func writeReplayPackFile(t *testing.T, root string, name string, terminalOutcome contracts.TerminalOutcome) string {
	t.Helper()

	replayPath := filepath.Join(root, name)
	replay := contracts.ReplayPack{
		SchemaVersion: contracts.SchemaVersionV1,
		ExportedAt:    time.Now().UTC(),
		Session: contracts.SessionHandle{
			SessionID: "benchmark-session-" + name,
		},
		Summary: contracts.SessionSummary{
			SessionID:       "benchmark-session-" + name,
			Status:          contracts.SessionStatusClosed,
			TerminalOutcome: terminalOutcome,
		},
		Events: []contracts.SessionEvent{
			{
				SchemaVersion: contracts.SchemaVersionV1,
				SessionID:     "benchmark-session-" + name,
				Sequence:      1,
				Kind:          contracts.EventKindSessionClosed,
			},
		},
	}
	encoded, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(replayPath, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return replayPath
}
