package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type Engine interface {
	StartSession(ctx context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error)
	SendCommand(ctx context.Context, sessionID string, cmd contracts.SessionCommand) error
	StreamEvents(ctx context.Context, sessionID string) (<-chan contracts.SessionEvent, error)
	ResumeSession(ctx context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error)
	CloseSession(ctx context.Context, sessionID string, reason string) error
}

type InMemoryEngine struct {
	mu       sync.RWMutex
	sessions map[string]contracts.SessionHandle
	events   map[string]chan contracts.SessionEvent
}

func NewInMemoryEngine() *InMemoryEngine {
	return &InMemoryEngine{
		sessions: make(map[string]contracts.SessionHandle),
		events:   make(map[string]chan contracts.SessionEvent),
	}
}

func (e *InMemoryEngine) StartSession(_ context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error) {
	if req.SessionID == "" {
		return contracts.SessionHandle{}, fmt.Errorf("session id is required")
	}

	handle := contracts.SessionHandle{
		SessionID: req.SessionID,
		CWD:       req.CWD,
		Mode:      req.Mode,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.sessions[req.SessionID] = handle
	if _, ok := e.events[req.SessionID]; !ok {
		e.events[req.SessionID] = make(chan contracts.SessionEvent, 16)
	}

	e.events[req.SessionID] <- contracts.SessionEvent{
		SchemaVersion: "v1",
		SessionID:     req.SessionID,
		Sequence:      1,
		Kind:          "session_started",
		Payload: map[string]any{
			"cwd":  req.CWD,
			"mode": req.Mode,
		},
	}

	return handle, nil
}

func (e *InMemoryEngine) SendCommand(_ context.Context, sessionID string, cmd contracts.SessionCommand) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ch, ok := e.events[sessionID]
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}

	ch <- contracts.SessionEvent{
		SchemaVersion: "v1",
		SessionID:     sessionID,
		Sequence:      2,
		Kind:          "command_received",
		Payload: map[string]any{
			"command_id": cmd.CommandID,
			"kind":       cmd.Kind,
		},
	}
	return nil
}

func (e *InMemoryEngine) StreamEvents(_ context.Context, sessionID string) (<-chan contracts.SessionEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ch, ok := e.events[sessionID]
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	return ch, nil
}

func (e *InMemoryEngine) ResumeSession(_ context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	handle, ok := e.sessions[req.SessionID]
	if !ok {
		return contracts.SessionHandle{}, fmt.Errorf("unknown session: %s", req.SessionID)
	}

	return handle, nil
}

func (e *InMemoryEngine) CloseSession(_ context.Context, sessionID string, _ string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ch, ok := e.events[sessionID]
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}

	ch <- contracts.SessionEvent{
		SchemaVersion: "v1",
		SessionID:     sessionID,
		Sequence:      3,
		Kind:          "session_closed",
		Payload:       map[string]any{},
	}
	close(ch)
	delete(e.events, sessionID)
	delete(e.sessions, sessionID)
	return nil
}

