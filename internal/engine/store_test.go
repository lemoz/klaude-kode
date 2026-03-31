package engine

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestFileSessionStoreConcurrentUpsertSummaryAcrossInstances(t *testing.T) {
	root := t.TempDir()

	storeA, err := newFileSessionStore(root)
	if err != nil {
		t.Fatalf("newFileSessionStore returned error: %v", err)
	}
	storeB, err := newFileSessionStore(root)
	if err != nil {
		t.Fatalf("newFileSessionStore returned error: %v", err)
	}

	var wg sync.WaitGroup
	errorsCh := make(chan error, 64)

	runWriter := func(prefix string, store *fileSessionStore) {
		defer wg.Done()
		for index := 0; index < 25; index++ {
			summary := contracts.SessionSummary{
				SessionID:     fmt.Sprintf("%s-%d", prefix, index),
				CWD:           "/tmp/project",
				Mode:          contracts.SessionModeInteractive,
				Status:        contracts.SessionStatusActive,
				ProfileID:     "anthropic-main",
				Model:         "claude-sonnet-4-6",
				CreatedAt:     time.Unix(int64(index), 0).UTC(),
				UpdatedAt:     time.Unix(int64(index)+1, 0).UTC(),
				EventCount:    index + 1,
				TurnCount:     index,
				LastSequence:  int64(index + 1),
				LastEventKind: contracts.EventKindSessionState,
			}
			if err := store.UpsertSummary(summary); err != nil {
				errorsCh <- err
				return
			}
		}
	}

	wg.Add(2)
	go runWriter("store-a", storeA)
	go runWriter("store-b", storeB)
	wg.Wait()
	close(errorsCh)

	for err := range errorsCh {
		if err != nil {
			t.Fatalf("expected concurrent upsert summaries to succeed, got %v", err)
		}
	}

	summaries, err := storeA.ListSummaries()
	if err != nil {
		t.Fatalf("ListSummaries returned error: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatalf("expected at least one summary after concurrent upserts")
	}
}
