package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type Engine interface {
	StartSession(ctx context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error)
	SendCommand(ctx context.Context, sessionID string, cmd contracts.SessionCommand) error
	StreamEvents(ctx context.Context, sessionID string) (<-chan contracts.SessionEvent, error)
	ListEvents(ctx context.Context, sessionID string) ([]contracts.SessionEvent, error)
	GetSessionSummary(ctx context.Context, sessionID string) (contracts.SessionSummary, error)
	ResumeSession(ctx context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error)
	CloseSession(ctx context.Context, sessionID string, reason string) error
}

type InMemoryEngine struct {
	mu               sync.RWMutex
	sessions         map[string]*sessionRecord
	nextSubscriberID uint64
}

type sessionRecord struct {
	handle       contracts.SessionHandle
	summary      contracts.SessionSummary
	events       []contracts.SessionEvent
	subscribers  map[uint64]chan contracts.SessionEvent
	nextSequence int64
	nextTurnID   int64
	closed       bool
}

func NewInMemoryEngine() *InMemoryEngine {
	return &InMemoryEngine{
		sessions: make(map[string]*sessionRecord),
	}
}

func (e *InMemoryEngine) StartSession(_ context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error) {
	now := time.Now().UTC()
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess_%d", now.UnixNano())
	}
	if req.CWD == "" {
		req.CWD = "."
	}
	if req.Mode == "" {
		req.Mode = contracts.SessionModeInteractive
	}

	handle := contracts.SessionHandle{
		SessionID: sessionID,
		CWD:       req.CWD,
		Mode:      req.Mode,
		ProfileID: req.ProfileID,
		Model:     req.Model,
		CreatedAt: now,
	}

	record := &sessionRecord{
		handle: handle,
		summary: contracts.SessionSummary{
			SessionID: sessionID,
			CWD:       req.CWD,
			Mode:      req.Mode,
			Status:    contracts.SessionStatusActive,
			ProfileID: req.ProfileID,
			Model:     req.Model,
			CreatedAt: now,
			UpdatedAt: now,
		},
		subscribers: make(map[uint64]chan contracts.SessionEvent),
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sessions[sessionID]; exists {
		return contracts.SessionHandle{}, fmt.Errorf("session already exists: %s", sessionID)
	}

	e.sessions[sessionID] = record
	e.appendEventLocked(record, contracts.EventKindSessionStarted, contracts.SessionEventPayload{
		State: e.nextStateSnapshotLocked(record),
	})
	e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		State: e.nextStateSnapshotLocked(record),
	})

	return handle, nil
}

func (e *InMemoryEngine) SendCommand(_ context.Context, sessionID string, cmd contracts.SessionCommand) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}
	if record.closed {
		return fmt.Errorf("session is closed: %s", sessionID)
	}

	cmd = normalizeCommand(cmd)
	if cmd.Kind == "" {
		return fmt.Errorf("command kind is required")
	}

	switch cmd.Kind {
	case contracts.CommandKindUserInput:
		return e.handleUserInputLocked(record, cmd)
	case contracts.CommandKindUpdateSessionSetting:
		return e.handleSettingUpdateLocked(record, cmd)
	case contracts.CommandKindCloseSession:
		return e.closeSessionLocked(record, cmd.Payload.Reason)
	default:
		return fmt.Errorf("unsupported command kind: %s", cmd.Kind)
	}
}

func (e *InMemoryEngine) StreamEvents(ctx context.Context, sessionID string) (<-chan contracts.SessionEvent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	backlog := append([]contracts.SessionEvent(nil), record.events...)
	bufferSize := len(backlog) + 8
	if bufferSize < 16 {
		bufferSize = 16
	}

	stream := make(chan contracts.SessionEvent, bufferSize)
	for _, event := range backlog {
		stream <- event
	}

	if record.closed {
		close(stream)
		return stream, nil
	}

	subscriberID := e.nextSubscriberID
	e.nextSubscriberID++
	record.subscribers[subscriberID] = stream

	go func() {
		<-ctx.Done()
		e.removeSubscriber(sessionID, subscriberID)
	}()

	return stream, nil
}

func (e *InMemoryEngine) ListEvents(_ context.Context, sessionID string) ([]contracts.SessionEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	return append([]contracts.SessionEvent(nil), record.events...), nil
}

func (e *InMemoryEngine) GetSessionSummary(_ context.Context, sessionID string) (contracts.SessionSummary, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return contracts.SessionSummary{}, fmt.Errorf("unknown session: %s", sessionID)
	}

	return record.summary, nil
}

func (e *InMemoryEngine) ResumeSession(_ context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.sessions[req.SessionID]
	if !ok {
		return contracts.SessionHandle{}, fmt.Errorf("unknown session: %s", req.SessionID)
	}

	return record.handle, nil
}

func (e *InMemoryEngine) CloseSession(_ context.Context, sessionID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return fmt.Errorf("unknown session: %s", sessionID)
	}

	return e.closeSessionLocked(record, reason)
}

func (e *InMemoryEngine) handleUserInputLocked(record *sessionRecord, cmd contracts.SessionCommand) error {
	text := strings.TrimSpace(cmd.Payload.Text)
	if text == "" {
		return fmt.Errorf("user_input text is required")
	}

	record.nextTurnID++
	turnID := cmd.Payload.TurnID
	if turnID == "" {
		turnID = fmt.Sprintf("turn_%d", record.nextTurnID)
	}

	source := cmd.Payload.Source
	if source == "" {
		source = contracts.MessageSourceInteractive
	}

	e.appendEventLocked(record, contracts.EventKindUserMessageAccepted, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		TurnID:    turnID,
		Source:    source,
		Message: &contracts.CanonicalMessage{
			Role:    "user",
			Content: text,
		},
	})

	record.summary.TurnCount++
	e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		TurnID:    turnID,
		State:     e.nextStateSnapshotLocked(record),
	})
	return nil
}

func (e *InMemoryEngine) handleSettingUpdateLocked(record *sessionRecord, cmd contracts.SessionCommand) error {
	switch cmd.Payload.SettingKey {
	case "model":
		record.handle.Model = cmd.Payload.SettingValue
		record.summary.Model = cmd.Payload.SettingValue
	case "profile_id":
		record.handle.ProfileID = cmd.Payload.SettingValue
		record.summary.ProfileID = cmd.Payload.SettingValue
	default:
		return fmt.Errorf("unsupported session setting: %s", cmd.Payload.SettingKey)
	}

	e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		State:     e.nextStateSnapshotLocked(record),
	})
	return nil
}

func (e *InMemoryEngine) closeSessionLocked(record *sessionRecord, reason string) error {
	if record.closed {
		return nil
	}

	record.closed = true
	record.summary.Status = contracts.SessionStatusClosed
	record.summary.ClosedReason = reason

	e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		Reason: reason,
		State:  e.nextStateSnapshotLocked(record),
	})
	e.appendEventLocked(record, contracts.EventKindSessionClosed, contracts.SessionEventPayload{
		Reason: reason,
	})

	for id, stream := range record.subscribers {
		close(stream)
		delete(record.subscribers, id)
	}

	return nil
}

func (e *InMemoryEngine) appendEventLocked(record *sessionRecord, kind contracts.EventKind, payload contracts.SessionEventPayload) contracts.SessionEvent {
	event := contracts.SessionEvent{
		SchemaVersion: contracts.SchemaVersionV1,
		SessionID:     record.handle.SessionID,
		Sequence:      record.nextSequence + 1,
		Timestamp:     time.Now().UTC(),
		Kind:          kind,
		Payload:       payload,
	}

	record.events = append(record.events, event)
	record.nextSequence = event.Sequence
	record.summary.EventCount = len(record.events)
	record.summary.LastSequence = event.Sequence
	record.summary.LastEventKind = kind
	record.summary.UpdatedAt = event.Timestamp

	for id, stream := range record.subscribers {
		select {
		case stream <- event:
		default:
			close(stream)
			delete(record.subscribers, id)
		}
	}

	return event
}

func (e *InMemoryEngine) nextStateSnapshotLocked(record *sessionRecord) *contracts.SessionStateSnapshot {
	return &contracts.SessionStateSnapshot{
		CWD:             record.summary.CWD,
		Mode:            record.summary.Mode,
		Status:          record.summary.Status,
		ProfileID:       record.summary.ProfileID,
		Model:           record.summary.Model,
		EventCount:      len(record.events) + 1,
		TurnCount:       record.summary.TurnCount,
		LastSequence:    record.nextSequence + 1,
		ClosedReason:    record.summary.ClosedReason,
		TerminalOutcome: record.summary.TerminalOutcome,
	}
}

func (e *InMemoryEngine) removeSubscriber(sessionID string, subscriberID uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.sessions[sessionID]
	if !ok {
		return
	}

	stream, ok := record.subscribers[subscriberID]
	if !ok {
		return
	}

	delete(record.subscribers, subscriberID)
	close(stream)
}

func normalizeCommand(cmd contracts.SessionCommand) contracts.SessionCommand {
	if cmd.SchemaVersion == "" {
		cmd.SchemaVersion = contracts.SchemaVersionV1
	}
	if cmd.CommandID == "" {
		cmd.CommandID = fmt.Sprintf("cmd_%d", time.Now().UTC().UnixNano())
	}
	if cmd.Timestamp.IsZero() {
		cmd.Timestamp = time.Now().UTC()
	}
	return cmd
}
