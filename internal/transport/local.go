package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
)

type LocalTransport struct {
	mu        sync.RWMutex
	engine    engine.Engine
	sessionID string
	closed    bool
}

func NewLocalTransport(runtime engine.Engine) *LocalTransport {
	return &LocalTransport{engine: runtime}
}

func (t *LocalTransport) Open(_ context.Context, target contracts.TransportTarget) error {
	if target.Kind != "local" {
		return fmt.Errorf("unsupported transport target kind: %s", target.Kind)
	}
	if target.Addr == "" {
		return fmt.Errorf("transport target addr is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = target.Addr
	t.closed = false
	return nil
}

func (t *LocalTransport) Send(ctx context.Context, cmd contracts.SessionCommand) error {
	sessionID, err := t.requireSession()
	if err != nil {
		return err
	}
	return t.engine.SendCommand(ctx, sessionID, cmd)
}

func (t *LocalTransport) Events(ctx context.Context) (<-chan contracts.SessionEvent, error) {
	sessionID, err := t.requireSession()
	if err != nil {
		return nil, err
	}
	return t.engine.StreamEvents(ctx, sessionID)
}

func (t *LocalTransport) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessionID = ""
	t.closed = true
	return nil
}

func (t *LocalTransport) requireSession() (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.engine == nil {
		return "", fmt.Errorf("transport engine is not configured")
	}
	if t.closed || t.sessionID == "" {
		return "", fmt.Errorf("transport is not open")
	}
	return t.sessionID, nil
}
