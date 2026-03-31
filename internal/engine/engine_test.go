package engine

import (
	"context"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestStartSessionRecordsAuthoritativeEventLog(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_start",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "profile-a",
		Model:     "model-a",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 initial events, got %d", len(events))
	}
	if events[0].Kind != contracts.EventKindSessionStarted {
		t.Fatalf("expected first event to be session_started, got %s", events[0].Kind)
	}
	if events[1].Kind != contracts.EventKindSessionState {
		t.Fatalf("expected second event to be session_state, got %s", events[1].Kind)
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("expected initial sequences 1 and 2, got %d and %d", events[0].Sequence, events[1].Sequence)
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary returned error: %v", err)
	}
	if summary.Status != contracts.SessionStatusActive {
		t.Fatalf("expected active session, got %s", summary.Status)
	}
	if summary.EventCount != 2 {
		t.Fatalf("expected summary event count 2, got %d", summary.EventCount)
	}
	if summary.LastSequence != 2 {
		t.Fatalf("expected last sequence 2, got %d", summary.LastSequence)
	}
}

func TestStreamEventsReplaysBacklogAndFutureEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := NewInMemoryEngine()
	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_stream",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	stream, err := runtime.StreamEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("StreamEvents returned error: %v", err)
	}

	first := nextEvent(t, stream)
	second := nextEvent(t, stream)
	if first.Kind != contracts.EventKindSessionStarted || second.Kind != contracts.EventKindSessionState {
		t.Fatalf("unexpected initial stream events: %s then %s", first.Kind, second.Kind)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello",
			Source: contracts.MessageSourceInteractive,
		},
	})
	if err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	third := nextEvent(t, stream)
	fourth := nextEvent(t, stream)
	fifth := nextEvent(t, stream)
	sixth := nextEvent(t, stream)
	seventh := nextEvent(t, stream)
	if third.Kind != contracts.EventKindUserMessageAccepted {
		t.Fatalf("expected user_message_accepted, got %s", third.Kind)
	}
	if fourth.Kind != contracts.EventKindLifecycle || fourth.Payload.LifecycleName != "turn_started" {
		t.Fatalf("expected lifecycle turn_started, got %s (%s)", fourth.Kind, fourth.Payload.LifecycleName)
	}
	if third.Payload.Message == nil || third.Payload.Message.Content != "hello" {
		t.Fatalf("unexpected message payload: %#v", third.Payload.Message)
	}
	if fifth.Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", fifth.Kind)
	}
	if sixth.Kind != contracts.EventKindLifecycle || sixth.Payload.LifecycleName != "turn_completed" {
		t.Fatalf("expected lifecycle turn_completed, got %s (%s)", sixth.Kind, sixth.Payload.LifecycleName)
	}
	if sixth.Payload.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		t.Fatalf("expected success terminal outcome, got %s", sixth.Payload.TerminalOutcome)
	}
	if seventh.Kind != contracts.EventKindSessionState {
		t.Fatalf("expected session_state after turn completion, got %s", seventh.Kind)
	}
	if seventh.Payload.State == nil || seventh.Payload.State.TurnCount != 1 {
		t.Fatalf("expected turn count 1 in state snapshot, got %#v", seventh.Payload.State)
	}
}

func TestCloseSessionPreservesReplayState(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_close",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeHeadless,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.CloseSession(ctx, handle.SessionID, "user_exit"); err != nil {
		t.Fatalf("CloseSession returned error: %v", err)
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary returned error: %v", err)
	}
	if summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected closed session, got %s", summary.Status)
	}
	if summary.ClosedReason != "user_exit" {
		t.Fatalf("expected closed reason user_exit, got %q", summary.ClosedReason)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 events after close, got %d", len(events))
	}
	if events[3].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", events[3].Kind)
	}

	resumed, err := runtime.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: handle.SessionID})
	if err != nil {
		t.Fatalf("ResumeSession returned error: %v", err)
	}
	if resumed.SessionID != handle.SessionID {
		t.Fatalf("expected resumed session %s, got %s", handle.SessionID, resumed.SessionID)
	}
}

func TestToolTurnEmitsPermissionAndToolLifecycle(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_tool",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeHeadless,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "tool:pwd",
			Source: contracts.MessageSourcePrint,
		},
	})
	if err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 12 {
		t.Fatalf("expected 12 events for tool turn, got %d", len(events))
	}

	if events[4].Kind != contracts.EventKindToolCallRequested {
		t.Fatalf("expected tool_call_requested, got %s", events[4].Kind)
	}
	if events[5].Kind != contracts.EventKindPermissionRequested {
		t.Fatalf("expected permission_requested, got %s", events[5].Kind)
	}
	if events[6].Kind != contracts.EventKindPermissionResolved {
		t.Fatalf("expected permission_resolved, got %s", events[6].Kind)
	}
	if events[7].Kind != contracts.EventKindToolCallProgress {
		t.Fatalf("expected tool_call_progress, got %s", events[7].Kind)
	}
	if events[8].Kind != contracts.EventKindToolCallCompleted {
		t.Fatalf("expected tool_call_completed, got %s", events[8].Kind)
	}
	if events[8].Payload.Tool == nil || events[8].Payload.Tool.Output != "/tmp/project" {
		t.Fatalf("expected pwd output /tmp/project, got %#v", events[8].Payload.Tool)
	}
	if events[9].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message after tool completion, got %s", events[9].Kind)
	}
	if events[10].Kind != contracts.EventKindLifecycle || events[10].Payload.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		t.Fatalf("expected successful turn_completed lifecycle, got %s (%s)", events[10].Kind, events[10].Payload.TerminalOutcome)
	}
}

func nextEvent(t *testing.T, stream <-chan contracts.SessionEvent) contracts.SessionEvent {
	t.Helper()

	select {
	case event := <-stream:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return contracts.SessionEvent{}
	}
}
