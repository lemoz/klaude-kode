package engine

import (
	"context"
	"errors"
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
	ListSessions(ctx context.Context) ([]contracts.SessionSummary, error)
	GetSessionSummary(ctx context.Context, sessionID string) (contracts.SessionSummary, error)
	ResumeSession(ctx context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error)
	CloseSession(ctx context.Context, sessionID string, reason string) error
}

type InMemoryEngine struct {
	mu               sync.RWMutex
	sessions         map[string]*sessionRecord
	nextSubscriberID uint64
	store            sessionStore
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

type recordSnapshot struct {
	handle       contracts.SessionHandle
	summary      contracts.SessionSummary
	nextSequence int64
	nextTurnID   int64
	closed       bool
	events       []contracts.SessionEvent
}

func NewInMemoryEngine() *InMemoryEngine {
	return &InMemoryEngine{
		sessions: make(map[string]*sessionRecord),
	}
}

func NewFileBackedEngine(root string) (*InMemoryEngine, error) {
	store, err := newFileSessionStore(root)
	if err != nil {
		return nil, err
	}
	return &InMemoryEngine{
		sessions: make(map[string]*sessionRecord),
		store:    store,
	}, nil
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
	if e.store != nil {
		exists, err := e.store.SessionExists(sessionID)
		if err != nil {
			return contracts.SessionHandle{}, err
		}
		if exists {
			return contracts.SessionHandle{}, fmt.Errorf("session already exists: %s", sessionID)
		}
	}

	e.sessions[sessionID] = record
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionStarted, contracts.SessionEventPayload{
		State: e.nextStateSnapshotLocked(record),
	}); err != nil {
		delete(e.sessions, sessionID)
		return contracts.SessionHandle{}, err
	}
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		State: e.nextStateSnapshotLocked(record),
	}); err != nil {
		delete(e.sessions, sessionID)
		return contracts.SessionHandle{}, err
	}

	return handle, nil
}

func (e *InMemoryEngine) SendCommand(_ context.Context, sessionID string, cmd contracts.SessionCommand) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, err := e.getSessionLocked(sessionID)
	if err != nil {
		return err
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

	record, err := e.getSessionLocked(sessionID)
	if err != nil {
		return nil, err
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
	e.mu.Lock()
	defer e.mu.Unlock()

	record, err := e.getSessionLocked(sessionID)
	if err != nil {
		return nil, err
	}

	return append([]contracts.SessionEvent(nil), record.events...), nil
}

func (e *InMemoryEngine) ListSessions(_ context.Context) ([]contracts.SessionSummary, error) {
	e.mu.RLock()
	if e.store == nil {
		summaries := make([]contracts.SessionSummary, 0, len(e.sessions))
		for _, record := range e.sessions {
			summaries = append(summaries, record.summary)
		}
		e.mu.RUnlock()
		return summaries, nil
	}
	e.mu.RUnlock()

	return e.store.ListSummaries()
}

func (e *InMemoryEngine) GetSessionSummary(_ context.Context, sessionID string) (contracts.SessionSummary, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, err := e.getSessionLocked(sessionID)
	if err != nil {
		return contracts.SessionSummary{}, err
	}

	return record.summary, nil
}

func (e *InMemoryEngine) ResumeSession(_ context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, err := e.getSessionLocked(req.SessionID)
	if err != nil {
		return contracts.SessionHandle{}, err
	}

	return record.handle, nil
}

func (e *InMemoryEngine) CloseSession(_ context.Context, sessionID string, reason string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, err := e.getSessionLocked(sessionID)
	if err != nil {
		return err
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

	if _, err := e.appendEventLocked(record, contracts.EventKindUserMessageAccepted, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		TurnID:    turnID,
		Source:    source,
		Message: &contracts.CanonicalMessage{
			Role:    "user",
			Content: text,
		},
	}); err != nil {
		return err
	}

	record.summary.TurnCount++
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		TurnID:    turnID,
		State:     e.nextStateSnapshotLocked(record),
	}); err != nil {
		record.summary.TurnCount--
		return err
	}
	return nil
}

func (e *InMemoryEngine) handleSettingUpdateLocked(record *sessionRecord, cmd contracts.SessionCommand) error {
	previousHandle := record.handle
	previousSummary := record.summary

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

	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		State:     e.nextStateSnapshotLocked(record),
	}); err != nil {
		record.handle = previousHandle
		record.summary = previousSummary
		return err
	}
	return nil
}

func (e *InMemoryEngine) closeSessionLocked(record *sessionRecord, reason string) error {
	if record.closed {
		return nil
	}

	record.closed = true
	record.summary.Status = contracts.SessionStatusClosed
	record.summary.ClosedReason = reason

	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		Reason: reason,
		State:  e.nextStateSnapshotLocked(record),
	}); err != nil {
		record.closed = false
		record.summary.Status = contracts.SessionStatusActive
		record.summary.ClosedReason = ""
		return err
	}
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionClosed, contracts.SessionEventPayload{
		Reason: reason,
	}); err != nil {
		record.closed = false
		record.summary.Status = contracts.SessionStatusActive
		record.summary.ClosedReason = ""
		return err
	}

	for id, stream := range record.subscribers {
		close(stream)
		delete(record.subscribers, id)
	}

	return nil
}

func (e *InMemoryEngine) appendEventLocked(record *sessionRecord, kind contracts.EventKind, payload contracts.SessionEventPayload) (contracts.SessionEvent, error) {
	snapshot := snapshotRecord(record)

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

	if err := e.persistRecordLocked(record, event); err != nil {
		restoreRecord(record, snapshot)
		return contracts.SessionEvent{}, err
	}

	for id, stream := range record.subscribers {
		select {
		case stream <- event:
		default:
			close(stream)
			delete(record.subscribers, id)
		}
	}

	return event, nil
}

func (e *InMemoryEngine) persistRecordLocked(record *sessionRecord, event contracts.SessionEvent) error {
	if e.store == nil {
		return nil
	}
	if err := e.store.AppendEvent(event); err != nil {
		return err
	}
	if err := e.store.UpsertSummary(record.summary); err != nil {
		return err
	}
	return nil
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

func (e *InMemoryEngine) getSessionLocked(sessionID string) (*sessionRecord, error) {
	if record, ok := e.sessions[sessionID]; ok {
		return record, nil
	}
	if e.store == nil {
		return nil, fmt.Errorf("unknown session: %s", sessionID)
	}

	summary, err := e.store.LoadSummary(sessionID)
	if err != nil {
		if errors.Is(err, errSessionNotFound) {
			return nil, fmt.Errorf("unknown session: %s", sessionID)
		}
		return nil, err
	}
	events, err := e.store.LoadEvents(sessionID)
	if err != nil {
		if errors.Is(err, errSessionNotFound) {
			return nil, fmt.Errorf("unknown session: %s", sessionID)
		}
		return nil, err
	}

	record := recordFromPersisted(summary, events)
	e.sessions[sessionID] = record
	return record, nil
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

func snapshotRecord(record *sessionRecord) recordSnapshot {
	return recordSnapshot{
		handle:       record.handle,
		summary:      record.summary,
		nextSequence: record.nextSequence,
		nextTurnID:   record.nextTurnID,
		closed:       record.closed,
		events:       append([]contracts.SessionEvent(nil), record.events...),
	}
}

func restoreRecord(record *sessionRecord, snapshot recordSnapshot) {
	record.handle = snapshot.handle
	record.summary = snapshot.summary
	record.nextSequence = snapshot.nextSequence
	record.nextTurnID = snapshot.nextTurnID
	record.closed = snapshot.closed
	record.events = append(record.events[:0], snapshot.events...)
}

func recordFromPersisted(summary contracts.SessionSummary, events []contracts.SessionEvent) *sessionRecord {
	return &sessionRecord{
		handle: contracts.SessionHandle{
			SessionID: summary.SessionID,
			CWD:       summary.CWD,
			Mode:      summary.Mode,
			ProfileID: summary.ProfileID,
			Model:     summary.Model,
			CreatedAt: summary.CreatedAt,
		},
		summary:      summary,
		events:       append([]contracts.SessionEvent(nil), events...),
		subscribers:  make(map[uint64]chan contracts.SessionEvent),
		nextSequence: summary.LastSequence,
		nextTurnID:   int64(summary.TurnCount),
		closed:       summary.Status == contracts.SessionStatusClosed,
	}
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
