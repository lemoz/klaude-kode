package transport

import (
	"context"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
)

func TestLocalTransportStreamsBacklogAndNewEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runtime := engine.NewInMemoryEngine()
	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "transport-session",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	transport := NewLocalTransport(runtime)
	if err := transport.Open(ctx, contracts.TransportTarget{
		Kind: "local",
		Addr: handle.SessionID,
	}); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	stream, err := transport.Events(ctx)
	if err != nil {
		t.Fatalf("Events returned error: %v", err)
	}

	if event := nextTransportEvent(t, stream); event.Kind != contracts.EventKindSessionStarted {
		t.Fatalf("expected session_started backlog event, got %s", event.Kind)
	}
	if event := nextTransportEvent(t, stream); event.Kind != contracts.EventKindSessionState {
		t.Fatalf("expected session_state backlog event, got %s", event.Kind)
	}

	if err := transport.Send(ctx, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello over transport",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if event := nextTransportEvent(t, stream); event.Kind != contracts.EventKindUserMessageAccepted {
		t.Fatalf("expected user_message_accepted, got %s", event.Kind)
	}
	if event := nextTransportEvent(t, stream); event.Kind != contracts.EventKindLifecycle {
		t.Fatalf("expected lifecycle event, got %s", event.Kind)
	}
}

func TestLocalTransportClosePreventsFurtherUse(t *testing.T) {
	ctx := context.Background()
	runtime := engine.NewInMemoryEngine()
	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "transport-close",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	transport := NewLocalTransport(runtime)
	if err := transport.Open(ctx, contracts.TransportTarget{Kind: "local", Addr: handle.SessionID}); err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := transport.Close(ctx); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := transport.Send(ctx, contracts.SessionCommand{Kind: contracts.CommandKindUserInput}); err == nil {
		t.Fatal("expected Send to fail after Close")
	}
}

func nextTransportEvent(t *testing.T, stream <-chan contracts.SessionEvent) contracts.SessionEvent {
	t.Helper()

	select {
	case event := <-stream:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transport event")
		return contracts.SessionEvent{}
	}
}
