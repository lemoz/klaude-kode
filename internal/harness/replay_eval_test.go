package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestRunReplayEvalReturnsCompletedRunForValidCandidateAndReplay(t *testing.T) {
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

	replayPath := filepath.Join(root, "replay.json")
	replay := contracts.ReplayPack{
		SchemaVersion: contracts.SchemaVersionV1,
		ExportedAt:    time.Now().UTC(),
		Session: contracts.SessionHandle{
			SessionID: "replay-session",
		},
		Summary: contracts.SessionSummary{
			SessionID:       "replay-session",
			Status:          contracts.SessionStatusClosed,
			TerminalOutcome: contracts.TerminalOutcomeSuccess,
		},
		Events: []contracts.SessionEvent{
			{SchemaVersion: contracts.SchemaVersionV1, SessionID: "replay-session", Sequence: 1, Kind: contracts.EventKindSessionClosed},
		},
	}
	encoded, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(replayPath, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	run, err := RunReplayEval(root, replayPath)
	if err != nil {
		t.Fatalf("RunReplayEval returned error: %v", err)
	}
	if run.Status != EvalRunStatusCompleted {
		t.Fatalf("expected completed eval run, got %#v", run)
	}
	if run.Kind != EvalRunKindReplay {
		t.Fatalf("expected replay eval kind, got %#v", run)
	}
	if run.Score != 1 {
		t.Fatalf("expected score 1, got %#v", run)
	}
}
