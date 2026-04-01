package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestLoadReplayPackReadsReplayJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pack.json")

	expected := contracts.ReplayPack{
		SchemaVersion: contracts.SchemaVersionV1,
		ExportedAt:    time.Now().UTC(),
		Session: contracts.SessionHandle{
			SessionID: "pack-session",
			CWD:       "/tmp/project",
			Mode:      contracts.SessionModeHeadless,
			ProfileID: "anthropic-main",
			Model:     "claude-sonnet-4-6",
			CreatedAt: time.Now().UTC(),
		},
		Summary: contracts.SessionSummary{
			SessionID:  "pack-session",
			Status:     contracts.SessionStatusClosed,
			EventCount: 1,
		},
		Events: []contracts.SessionEvent{
			{SchemaVersion: contracts.SchemaVersionV1, SessionID: "pack-session", Sequence: 1, Kind: contracts.EventKindSessionClosed},
		},
	}

	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := LoadReplayPack(path)
	if err != nil {
		t.Fatalf("LoadReplayPack returned error: %v", err)
	}
	if got.Session.SessionID != expected.Session.SessionID {
		t.Fatalf("expected session %s, got %s", expected.Session.SessionID, got.Session.SessionID)
	}
	if len(got.Events) != 1 {
		t.Fatalf("expected one event, got %d", len(got.Events))
	}
}

func TestLoadBenchmarkPackReadsBenchmarkJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "benchmark.json")

	expected := BenchmarkPack{
		SchemaVersion: contracts.SchemaVersionV1,
		Name:          "baseline-pack",
		Description:   "baseline replay benchmark pack",
		Cases: []BenchmarkCase{
			{
				ID:         "case_one",
				ReplayPath: "replays/case-one.json",
				Weight:     1,
			},
		},
	}

	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := LoadBenchmarkPack(path)
	if err != nil {
		t.Fatalf("LoadBenchmarkPack returned error: %v", err)
	}
	if got.Name != expected.Name {
		t.Fatalf("expected pack name %s, got %s", expected.Name, got.Name)
	}
	if len(got.Cases) != 1 {
		t.Fatalf("expected one case, got %d", len(got.Cases))
	}
	if got.Cases[0].ReplayPath != expected.Cases[0].ReplayPath {
		t.Fatalf("expected replay path %s, got %s", expected.Cases[0].ReplayPath, got.Cases[0].ReplayPath)
	}
}
