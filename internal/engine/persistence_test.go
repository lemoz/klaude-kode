package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestFileBackedEnginePersistsReplayAndSessionIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "persisted-session",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeHeadless,
		ProfileID: "profile-a",
		Model:     "model-a",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "persist me",
			Source: contracts.MessageSourcePrint,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}
	if err := runtime.CloseSession(ctx, handle.SessionID, "done"); err != nil {
		t.Fatalf("CloseSession returned error: %v", err)
	}

	summaries, err := runtime.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 session summary, got %d", len(summaries))
	}
	if summaries[0].SessionID != handle.SessionID {
		t.Fatalf("expected indexed session %s, got %s", handle.SessionID, summaries[0].SessionID)
	}

	eventsPath := filepath.Join(root, "sessions", handle.SessionID, "events.jsonl")
	if _, err := os.Stat(eventsPath); err != nil {
		t.Fatalf("expected persisted events at %s: %v", eventsPath, err)
	}
	indexPath := filepath.Join(root, "state", "session-index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("expected persisted index at %s: %v", indexPath, err)
	}
}

func TestFileBackedEngineResumeAcrossInstances(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtimeA, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	handle, err := runtimeA.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "resume-session",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeHeadless,
		ProfileID: "profile-a",
		Model:     "model-a",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtimeA.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "resume me",
			Source: contracts.MessageSourcePrint,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}
	if err := runtimeA.CloseSession(ctx, handle.SessionID, "done"); err != nil {
		t.Fatalf("CloseSession returned error: %v", err)
	}

	runtimeB, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	resumed, err := runtimeB.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: handle.SessionID})
	if err != nil {
		t.Fatalf("ResumeSession returned error: %v", err)
	}
	if resumed.SessionID != handle.SessionID {
		t.Fatalf("expected resumed session %s, got %s", handle.SessionID, resumed.SessionID)
	}

	summary, err := runtimeB.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary returned error: %v", err)
	}
	if summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected resumed summary to be closed, got %s", summary.Status)
	}

	events, err := runtimeB.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 9 {
		t.Fatalf("expected 9 replay events, got %d", len(events))
	}

	stream, err := runtimeB.StreamEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("StreamEvents returned error: %v", err)
	}

	count := 0
	for range stream {
		count++
	}
	if count != 9 {
		t.Fatalf("expected resumed stream to replay 9 events, got %d", count)
	}
}

func TestFileBackedEngineResumesPendingPermissionRequests(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtimeA, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	handle, err := runtimeA.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "resume-pending-permission",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "profile-a",
		Model:     "model-a",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtimeA.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "tool:pwd",
			Source: contracts.MessageSourceInteractive,
			Metadata: map[string]string{
				"permission_mode": "ask",
			},
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	runtimeB, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	if _, err := runtimeB.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: handle.SessionID}); err != nil {
		t.Fatalf("ResumeSession returned error: %v", err)
	}

	if err := runtimeB.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindApprovePermission,
		Payload: contracts.SessionCommandPayload{
			RequestID: "perm_tool_turn_1_1",
		},
	}); err != nil {
		t.Fatalf("ApprovePermission returned error after resume: %v", err)
	}

	events, err := runtimeB.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 12 {
		t.Fatalf("expected 12 events after resumed permission approval, got %d", len(events))
	}
	if events[6].Kind != contracts.EventKindPermissionResolved {
		t.Fatalf("expected permission_resolved after resume, got %s", events[6].Kind)
	}
}
