package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cdossman/klaude-kode/internal/auth/anthropicoauth"
	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/provider"
	"github.com/cdossman/klaude-kode/internal/toolruntime"
)

type Engine interface {
	StartSession(ctx context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error)
	SendCommand(ctx context.Context, sessionID string, cmd contracts.SessionCommand) error
	StreamEvents(ctx context.Context, sessionID string) (<-chan contracts.SessionEvent, error)
	ListEvents(ctx context.Context, sessionID string) ([]contracts.SessionEvent, error)
	ListSessions(ctx context.Context) ([]contracts.SessionSummary, error)
	ListProfiles(ctx context.Context) ([]contracts.ProfileStatus, error)
	SaveProfile(ctx context.Context, profile contracts.AuthProfile, makeDefault bool) (contracts.ProfileStatus, error)
	LogoutProfile(ctx context.Context, profileID string) (contracts.ProfileStatus, error)
	GetSessionSummary(ctx context.Context, sessionID string) (contracts.SessionSummary, error)
	ResumeSession(ctx context.Context, req contracts.ResumeSessionRequest) (contracts.SessionHandle, error)
	CloseSession(ctx context.Context, sessionID string, reason string) error
}

type InMemoryEngine struct {
	mu               sync.RWMutex
	sessions         map[string]*sessionRecord
	nextSubscriberID uint64
	store            sessionStore
	toolRuntime      toolruntime.Runtime
	providers        *provider.Registry
	profileStore     provider.ProfileStore
}

type sessionRecord struct {
	handle            contracts.SessionHandle
	summary           contracts.SessionSummary
	events            []contracts.SessionEvent
	subscribers       map[uint64]chan contracts.SessionEvent
	nextSequence      int64
	nextTurnID        int64
	closed            bool
	pendingPermission *pendingPermissionState
}

type recordSnapshot struct {
	handle            contracts.SessionHandle
	summary           contracts.SessionSummary
	nextSequence      int64
	nextTurnID        int64
	closed            bool
	events            []contracts.SessionEvent
	pendingPermission *pendingPermissionState
}

type pendingPermissionState struct {
	RequestID    string
	CommandID    string
	TurnID       string
	ToolCall     contracts.ToolCall
	PolicySource string
	Prompt       string
	Scope        string
}

type turnResult struct {
	Pending          bool
	Outcome          contracts.TerminalOutcome
	AssistantDeltas  []string
	AssistantMessage string
}

func NewInMemoryEngine() *InMemoryEngine {
	return &InMemoryEngine{
		sessions:     make(map[string]*sessionRecord),
		toolRuntime:  toolruntime.NewBuiltinRuntime(),
		providers:    provider.DefaultRegistry(),
		profileStore: provider.NewMemoryProfileStore(),
	}
}

func NewFileBackedEngine(root string) (*InMemoryEngine, error) {
	store, err := newFileSessionStore(root)
	if err != nil {
		return nil, err
	}
	profileStore, err := provider.NewFileProfileStore(root)
	if err != nil {
		return nil, err
	}
	return &InMemoryEngine{
		sessions:     make(map[string]*sessionRecord),
		store:        store,
		toolRuntime:  toolruntime.NewBuiltinRuntime(),
		providers:    provider.DefaultRegistry(),
		profileStore: profileStore,
	}, nil
}

func (e *InMemoryEngine) StartSession(_ context.Context, req contracts.StartSessionRequest) (contracts.SessionHandle, error) {
	now := time.Now().UTC()
	if err := e.applyProfileDefaults(&req); err != nil {
		return contracts.SessionHandle{}, err
	}
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
	case contracts.CommandKindApprovePermission:
		return e.handlePermissionResolutionLocked(record, cmd, true)
	case contracts.CommandKindDenyPermission:
		return e.handlePermissionResolutionLocked(record, cmd, false)
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

func (e *InMemoryEngine) ListProfiles(ctx context.Context) ([]contracts.ProfileStatus, error) {
	profiles := []contracts.AuthProfile(nil)
	if e.profileStore != nil {
		list, err := e.profileStore.ListProfiles()
		if err != nil {
			return nil, err
		}
		profiles = list
	} else {
		profiles = []contracts.AuthProfile{provider.ResolveSessionProfile("", "")}
	}

	statuses := make([]contracts.ProfileStatus, 0, len(profiles))
	for _, profile := range profiles {
		status, err := e.buildProfileStatus(ctx, profile)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (e *InMemoryEngine) SaveProfile(ctx context.Context, profile contracts.AuthProfile, makeDefault bool) (contracts.ProfileStatus, error) {
	if e.profileStore == nil {
		return contracts.ProfileStatus{}, fmt.Errorf("profile store is unavailable")
	}

	status, err := e.buildProfileStatus(ctx, profile)
	if err != nil {
		return contracts.ProfileStatus{}, err
	}
	if !status.Validation.Valid {
		return contracts.ProfileStatus{}, fmt.Errorf(status.Validation.Message)
	}

	if err := e.profileStore.SaveProfile(profile); err != nil {
		return contracts.ProfileStatus{}, err
	}
	if makeDefault {
		if err := e.profileStore.SetDefaultProfile(profile.ID); err != nil {
			return contracts.ProfileStatus{}, err
		}
	}
	return status, nil
}

func (e *InMemoryEngine) LogoutProfile(ctx context.Context, profileID string) (contracts.ProfileStatus, error) {
	if e.profileStore == nil {
		return contracts.ProfileStatus{}, fmt.Errorf("profile store is unavailable")
	}

	profile, err := e.profileStore.GetProfile(strings.TrimSpace(profileID))
	if err != nil {
		return contracts.ProfileStatus{}, err
	}
	profile.Settings = clearedProfileAuthSettings(profile)

	if err := e.profileStore.SaveProfile(profile); err != nil {
		return contracts.ProfileStatus{}, err
	}
	return e.buildProfileStatus(ctx, profile)
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

	if _, err := e.appendEventLocked(record, contracts.EventKindLifecycle, contracts.SessionEventPayload{
		CommandID:     cmd.CommandID,
		TurnID:        turnID,
		LifecycleName: "turn_started",
	}); err != nil {
		return err
	}

	previousOutcome := record.summary.TerminalOutcome
	result, err := e.runTurnLocked(record, turnID, cmd.CommandID, text, cmd.Payload.Metadata)
	if err != nil {
		record.summary.TerminalOutcome = previousOutcome
		return err
	}
	if result.Pending {
		return nil
	}

	if err := e.finishTurnLocked(record, cmd.CommandID, turnID, result.Outcome, result.AssistantDeltas, result.AssistantMessage); err != nil {
		record.summary.TerminalOutcome = previousOutcome
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
		nextProfile, nextModel, err := e.resolveProfileSetting(record.handle.ProfileID, record.handle.Model, cmd.Payload.SettingValue)
		if err != nil {
			return err
		}
		record.handle.ProfileID = nextProfile
		record.summary.ProfileID = nextProfile
		record.handle.Model = nextModel
		record.summary.Model = nextModel
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

func (e *InMemoryEngine) finishTurnLocked(record *sessionRecord, commandID string, turnID string, outcome contracts.TerminalOutcome, _ []string, assistantMessage string) error {
	record.summary.TerminalOutcome = outcome

	if _, err := e.appendEventLocked(record, contracts.EventKindAssistantMessage, contracts.SessionEventPayload{
		CommandID: commandID,
		TurnID:    turnID,
		Message: &contracts.CanonicalMessage{
			Role:    "assistant",
			Content: assistantMessage,
		},
	}); err != nil {
		return err
	}
	if _, err := e.appendEventLocked(record, contracts.EventKindLifecycle, contracts.SessionEventPayload{
		CommandID:       commandID,
		TurnID:          turnID,
		LifecycleName:   "turn_completed",
		TerminalOutcome: outcome,
	}); err != nil {
		return err
	}

	record.summary.TurnCount++
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		CommandID: commandID,
		TurnID:    turnID,
		State:     e.nextStateSnapshotLocked(record),
	}); err != nil {
		record.summary.TurnCount--
		return err
	}
	return nil
}

func (e *InMemoryEngine) handlePermissionResolutionLocked(record *sessionRecord, cmd contracts.SessionCommand, allow bool) error {
	if record.pendingPermission == nil {
		return fmt.Errorf("no permission request is pending")
	}
	pending := copyPendingPermission(record.pendingPermission)

	requestID := strings.TrimSpace(cmd.Payload.RequestID)
	if requestID == "" {
		return fmt.Errorf("permission request_id is required")
	}
	if requestID != pending.RequestID {
		return fmt.Errorf("unknown pending permission request: %s", requestID)
	}

	previousOutcome := record.summary.TerminalOutcome
	result, err := e.resolvePendingPermissionLocked(record, cmd, allow)
	if err != nil {
		record.summary.TerminalOutcome = previousOutcome
		return err
	}

	if err := e.finishTurnLocked(record, cmd.CommandID, pending.TurnID, result.Outcome, result.AssistantDeltas, result.AssistantMessage); err != nil {
		record.summary.TerminalOutcome = previousOutcome
		return err
	}
	record.pendingPermission = nil
	return nil
}

func (e *InMemoryEngine) closeSessionLocked(record *sessionRecord, reason string) error {
	if record.closed {
		return nil
	}

	record.closed = true
	record.summary.Status = contracts.SessionStatusClosed
	record.summary.ClosedReason = reason
	previousPending := copyPendingPermission(record.pendingPermission)
	record.pendingPermission = nil

	if _, err := e.appendEventLocked(record, contracts.EventKindSessionState, contracts.SessionEventPayload{
		Reason: reason,
		State:  e.nextStateSnapshotLocked(record),
	}); err != nil {
		record.closed = false
		record.summary.Status = contracts.SessionStatusActive
		record.summary.ClosedReason = ""
		record.pendingPermission = previousPending
		return err
	}
	if _, err := e.appendEventLocked(record, contracts.EventKindSessionClosed, contracts.SessionEventPayload{
		Reason: reason,
	}); err != nil {
		record.closed = false
		record.summary.Status = contracts.SessionStatusActive
		record.summary.ClosedReason = ""
		record.pendingPermission = previousPending
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
		handle:            record.handle,
		summary:           record.summary,
		nextSequence:      record.nextSequence,
		nextTurnID:        record.nextTurnID,
		closed:            record.closed,
		events:            append([]contracts.SessionEvent(nil), record.events...),
		pendingPermission: copyPendingPermission(record.pendingPermission),
	}
}

func restoreRecord(record *sessionRecord, snapshot recordSnapshot) {
	record.handle = snapshot.handle
	record.summary = snapshot.summary
	record.nextSequence = snapshot.nextSequence
	record.nextTurnID = snapshot.nextTurnID
	record.closed = snapshot.closed
	record.events = append(record.events[:0], snapshot.events...)
	record.pendingPermission = copyPendingPermission(snapshot.pendingPermission)
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
		summary:           summary,
		events:            append([]contracts.SessionEvent(nil), events...),
		subscribers:       make(map[uint64]chan contracts.SessionEvent),
		nextSequence:      summary.LastSequence,
		nextTurnID:        nextTurnIDFromEvents(summary, events),
		closed:            summary.Status == contracts.SessionStatusClosed,
		pendingPermission: pendingPermissionFromEvents(events),
	}
}

func copyPendingPermission(pending *pendingPermissionState) *pendingPermissionState {
	if pending == nil {
		return nil
	}

	clonedCall := pending.ToolCall
	if pending.ToolCall.Input != nil {
		clonedCall.Input = make(map[string]any, len(pending.ToolCall.Input))
		for key, value := range pending.ToolCall.Input {
			clonedCall.Input[key] = value
		}
	}

	return &pendingPermissionState{
		RequestID:    pending.RequestID,
		CommandID:    pending.CommandID,
		TurnID:       pending.TurnID,
		ToolCall:     clonedCall,
		PolicySource: pending.PolicySource,
		Prompt:       pending.Prompt,
		Scope:        pending.Scope,
	}
}

func nextTurnIDFromEvents(summary contracts.SessionSummary, events []contracts.SessionEvent) int64 {
	maxTurnID := int64(summary.TurnCount)
	for _, event := range events {
		turnID := strings.TrimSpace(event.Payload.TurnID)
		if turnID == "" || !strings.HasPrefix(turnID, "turn_") {
			continue
		}
		var value int64
		if _, err := fmt.Sscanf(turnID, "turn_%d", &value); err == nil && value > maxTurnID {
			maxTurnID = value
		}
	}
	return maxTurnID
}

func pendingPermissionFromEvents(events []contracts.SessionEvent) *pendingPermissionState {
	toolCalls := make(map[string]pendingPermissionState)
	var pending *pendingPermissionState

	for _, event := range events {
		switch event.Kind {
		case contracts.EventKindToolCallRequested:
			if event.Payload.Tool == nil {
				continue
			}
			toolCalls[event.Payload.Tool.CallID] = pendingPermissionState{
				CommandID: event.Payload.CommandID,
				TurnID:    event.Payload.TurnID,
				ToolCall: contracts.ToolCall{
					ID:    event.Payload.Tool.CallID,
					Name:  event.Payload.Tool.Name,
					Input: copyToolInput(event.Payload.Tool.Input),
				},
			}
		case contracts.EventKindPermissionRequested:
			if event.Payload.Permission == nil {
				continue
			}
			call := toolCalls[event.Payload.Permission.ToolCallID]
			call.RequestID = event.Payload.Permission.RequestID
			call.PolicySource = event.Payload.Permission.PolicySource
			call.Prompt = event.Payload.Permission.Prompt
			call.Scope = event.Payload.Permission.Scope
			if call.CommandID == "" {
				call.CommandID = event.Payload.CommandID
			}
			if call.TurnID == "" {
				call.TurnID = event.Payload.TurnID
			}
			pending = &call
		case contracts.EventKindPermissionResolved:
			if pending == nil || event.Payload.Permission == nil {
				continue
			}
			if pending.RequestID == event.Payload.Permission.RequestID {
				pending = nil
			}
		case contracts.EventKindLifecycle:
			if event.Payload.LifecycleName == "turn_completed" {
				pending = nil
			}
		}
	}

	return copyPendingPermission(pending)
}

func copyToolInput(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
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

func scaffoldAssistantReply(model string, userText string) string {
	if strings.TrimSpace(model) == "" {
		model = "default-model"
	}
	return fmt.Sprintf(
		"Scaffold response from %s. Provider execution is not wired yet. Received: %s",
		model,
		userText,
	)
}

func (e *InMemoryEngine) runTurnLocked(record *sessionRecord, turnID string, commandID string, userText string, metadata map[string]string) (turnResult, error) {
	call, isToolCall := toolruntime.ParseInlineToolCall(userText)
	if !isToolCall {
		return e.executeProviderTurnLocked(record, turnID, commandID, userText)
	}

	call.ID = fmt.Sprintf("tool_%s_1", turnID)
	descriptor, err := e.lookupToolDescriptorLocked(record, call.Name)
	if err != nil {
		if emitErr := e.emitTurnFailureLocked(record, turnID, commandID, err, contracts.TerminalOutcomeToolFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeToolFailure,
			AssistantMessage: fmt.Sprintf("Tool %s failed: %v", call.Name, err),
		}, nil
	}

	if _, err := e.appendEventLocked(record, contracts.EventKindToolCallRequested, contracts.SessionEventPayload{
		CommandID: commandID,
		TurnID:    turnID,
		Tool: &contracts.ToolEventPayload{
			CallID:           call.ID,
			Name:             descriptor.Name,
			Input:            call.Input,
			ConcurrencyClass: descriptor.ConcurrencyClass,
		},
	}); err != nil {
		return turnResult{}, err
	}

	if descriptor.RequiresPermission {
		requestID := fmt.Sprintf("perm_%s", call.ID)
		pending := &pendingPermissionState{
			RequestID:    requestID,
			CommandID:    commandID,
			TurnID:       turnID,
			ToolCall:     call,
			PolicySource: "builtin_headless_policy",
			Prompt:       fmt.Sprintf("Allow %s to access %s?", descriptor.Name, descriptor.PermissionScope),
			Scope:        descriptor.PermissionScope,
		}

		record.pendingPermission = pending
		if _, err := e.appendEventLocked(record, contracts.EventKindPermissionRequested, contracts.SessionEventPayload{
			CommandID: commandID,
			TurnID:    turnID,
			Permission: &contracts.PermissionEventPayload{
				RequestID:    pending.RequestID,
				ToolCallID:   call.ID,
				PolicySource: pending.PolicySource,
				Prompt:       pending.Prompt,
				Scope:        pending.Scope,
			},
		}); err != nil {
			record.pendingPermission = nil
			return turnResult{}, err
		}

		if requiresInteractivePermission(metadata) {
			return turnResult{Pending: true}, nil
		}

		result, err := e.resolvePendingPermissionLocked(record, contracts.SessionCommand{
			CommandID: commandID,
			Payload: contracts.SessionCommandPayload{
				RequestID: pending.RequestID,
			},
		}, true)
		record.pendingPermission = nil
		return result, err
	}

	return e.executeToolCallLocked(record, turnID, commandID, call, descriptor)
}

func (e *InMemoryEngine) executeProviderTurnLocked(record *sessionRecord, turnID string, commandID string, userText string) (turnResult, error) {
	if e.providers == nil {
		return turnResult{
			Outcome:          contracts.TerminalOutcomeSuccess,
			AssistantMessage: scaffoldAssistantReply(record.handle.Model, userText),
		}, nil
	}

	profile, err := e.resolveProfile(record.handle.ProfileID, record.handle.Model)
	if err != nil {
		if emitErr := e.emitFailureLocked(record, turnID, commandID, contracts.FailureCategoryAuth, "profile_resolution_failed", err, contracts.TerminalOutcomeProviderFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeProviderFailure,
			AssistantMessage: fmt.Sprintf("Provider setup failed: %v", err),
		}, nil
	}
	validation, err := e.providers.ValidateProfile(context.Background(), profile)
	if err != nil {
		if emitErr := e.emitFailureLocked(record, turnID, commandID, contracts.FailureCategoryAuth, "profile_validation_failed", err, contracts.TerminalOutcomeProviderFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeProviderFailure,
			AssistantMessage: fmt.Sprintf("Provider setup failed: %v", err),
		}, nil
	}
	if !validation.Valid {
		err := fmt.Errorf(validation.Message)
		if emitErr := e.emitFailureLocked(record, turnID, commandID, contracts.FailureCategoryAuth, "invalid_profile", err, contracts.TerminalOutcomeProviderFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeProviderFailure,
			AssistantMessage: fmt.Sprintf("Provider setup failed: %s", validation.Message),
		}, nil
	}

	model := strings.TrimSpace(record.handle.Model)
	if model == "" {
		model = profile.DefaultModel
	}

	preflightProfile, err := e.maybeRefreshOAuthProfileLocked(context.Background(), profile, false)
	if err == nil {
		profile = preflightProfile
	}
	if err := e.providers.ValidateModel(context.Background(), profile, model); err != nil {
		category, code, outcome, retryable, assistantMessage := classifyProviderFailure(err)
		if emitErr := e.emitFailureDetailedLocked(record, turnID, commandID, category, code, err, outcome, retryable); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          outcome,
			AssistantMessage: assistantMessage,
		}, nil
	}

	completionRequest := contracts.CompletionRequest{
		TurnID: turnID,
		Model:  model,
		Messages: []contracts.CanonicalMessage{
			{
				Role:    "user",
				Content: userText,
			},
		},
	}

	assistantDeltas, completion, err := e.streamOrCompleteProviderTurn(record, turnID, commandID, profile, completionRequest)
	if shouldRetryOAuthCompletion(err, profile) {
		refreshedProfile, refreshErr := e.maybeRefreshOAuthProfileLocked(context.Background(), profile, true)
		if refreshErr == nil {
			profile = refreshedProfile
			assistantDeltas, completion, err = e.streamOrCompleteProviderTurn(record, turnID, commandID, profile, completionRequest)
		} else {
			err = &provider.Error{
				Code:      provider.ErrorCodeAuthFailed,
				Message:   fmt.Sprintf("oauth refresh failed: %v", refreshErr),
				Retryable: false,
			}
		}
	}
	if err != nil {
		category, code, outcome, retryable, assistantMessage := classifyProviderFailure(err)
		if emitErr := e.emitFailureDetailedLocked(record, turnID, commandID, category, code, err, outcome, retryable); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          outcome,
			AssistantMessage: assistantMessage,
		}, nil
	}

	message := strings.TrimSpace(completion.Message.Content)
	if message == "" {
		message = scaffoldAssistantReply(model, userText)
	}

	return turnResult{
		Outcome:          contracts.TerminalOutcomeSuccess,
		AssistantDeltas:  assistantDeltas,
		AssistantMessage: message,
	}, nil
}

func (e *InMemoryEngine) streamOrCompleteProviderTurn(record *sessionRecord, turnID string, commandID string, profile contracts.AuthProfile, req contracts.CompletionRequest) ([]string, contracts.CompletionResult, error) {
	stream, err := e.providers.StreamCompletion(context.Background(), profile, req)
	if err == nil && stream != nil {
		deltas, streamErr := e.consumeProviderStreamLocked(record, turnID, commandID, stream)
		if streamErr != nil {
			return nil, contracts.CompletionResult{}, streamErr
		}
		return deltas, contracts.CompletionResult{
			Message: contracts.CanonicalMessage{
				Role:    "assistant",
				Content: strings.Join(deltas, ""),
			},
		}, nil
	}
	if err != nil && !errors.Is(err, provider.ErrCompletionNotImplemented) {
		return nil, contracts.CompletionResult{}, err
	}

	completion, completeErr := e.providers.Complete(context.Background(), profile, req)
	return nil, completion, completeErr
}

func (e *InMemoryEngine) consumeProviderStreamLocked(record *sessionRecord, turnID string, commandID string, stream <-chan contracts.ProviderEvent) ([]string, error) {
	deltas := make([]string, 0, 1)
	for providerEvent := range stream {
		if providerEvent.Kind != "assistant_delta" {
			continue
		}
		text, _ := providerEvent.Payload["text"].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		deltas = append(deltas, text)
		if _, err := e.appendEventLocked(record, contracts.EventKindAssistantDelta, contracts.SessionEventPayload{
			CommandID: commandID,
			TurnID:    turnID,
			Message: &contracts.CanonicalMessage{
				Role:    "assistant",
				Content: text,
			},
		}); err != nil {
			return nil, err
		}
	}
	return deltas, nil
}

func shouldRetryOAuthCompletion(err error, profile contracts.AuthProfile) bool {
	if err == nil || profile.Kind != contracts.AuthProfileAnthropicOAuth {
		return false
	}
	providerErr := provider.AsError(err)
	return providerErr != nil && providerErr.Code == provider.ErrorCodeAuthFailed
}

func (e *InMemoryEngine) resolvePendingPermissionLocked(record *sessionRecord, cmd contracts.SessionCommand, allow bool) (turnResult, error) {
	pending := record.pendingPermission
	if pending == nil {
		return turnResult{}, fmt.Errorf("no permission request is pending")
	}

	resolution := "deny"
	actor := "user"
	if allow {
		resolution = "allow_once"
		if cmd.CommandID == pending.CommandID {
			actor = "auto"
		}
	}

	if _, err := e.appendEventLocked(record, contracts.EventKindPermissionResolved, contracts.SessionEventPayload{
		CommandID: cmd.CommandID,
		TurnID:    pending.TurnID,
		Permission: &contracts.PermissionEventPayload{
			RequestID:    pending.RequestID,
			ToolCallID:   pending.ToolCall.ID,
			PolicySource: pending.PolicySource,
			Scope:        pending.Scope,
			Resolution:   resolution,
			Actor:        actor,
		},
	}); err != nil {
		return turnResult{}, err
	}

	if !allow {
		err := fmt.Errorf("permission denied for tool %s", pending.ToolCall.Name)
		if emitErr := e.emitFailureLocked(record, pending.TurnID, cmd.CommandID, contracts.FailureCategoryPermission, "permission_denied", err, contracts.TerminalOutcomeTaskFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeTaskFailure,
			AssistantMessage: fmt.Sprintf("Permission denied for tool %s.", pending.ToolCall.Name),
		}, nil
	}

	descriptor, err := e.lookupToolDescriptorLocked(record, pending.ToolCall.Name)
	if err != nil {
		if emitErr := e.emitFailureLocked(record, pending.TurnID, cmd.CommandID, contracts.FailureCategoryTool, "tool_lookup_failed", err, contracts.TerminalOutcomeToolFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeToolFailure,
			AssistantMessage: fmt.Sprintf("Tool %s failed: %v", pending.ToolCall.Name, err),
		}, nil
	}

	return e.executeToolCallLocked(record, pending.TurnID, cmd.CommandID, pending.ToolCall, descriptor)
}

func (e *InMemoryEngine) executeToolCallLocked(record *sessionRecord, turnID string, commandID string, call contracts.ToolCall, descriptor contracts.ToolDescriptor) (turnResult, error) {
	sessionContext := contracts.SessionContext{
		SessionID: record.handle.SessionID,
		CWD:       record.handle.CWD,
		Mode:      record.handle.Mode,
		ProfileID: record.handle.ProfileID,
		Model:     record.handle.Model,
	}
	stream, err := e.toolRuntime.ExecuteTool(context.Background(), sessionContext, call)
	if err != nil {
		if emitErr := e.emitTurnFailureLocked(record, turnID, commandID, err, contracts.TerminalOutcomeToolFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeToolFailure,
			AssistantMessage: fmt.Sprintf("Tool %s failed: %v", call.Name, err),
		}, nil
	}

	resultSummary := ""
	output := ""
	failed := false
	for toolEvent := range stream {
		switch toolEvent.Kind {
		case contracts.ToolEventKindProgress:
			if _, err := e.appendEventLocked(record, contracts.EventKindToolCallProgress, contracts.SessionEventPayload{
				CommandID: commandID,
				TurnID:    turnID,
				Tool: &contracts.ToolEventPayload{
					CallID:           call.ID,
					Name:             call.Name,
					Input:            call.Input,
					ConcurrencyClass: descriptor.ConcurrencyClass,
					ProgressMessage:  toolEvent.Message,
				},
			}); err != nil {
				return turnResult{}, err
			}
		case contracts.ToolEventKindCompleted:
			resultSummary = toolEvent.ResultSummary
			output = toolEvent.Output
			failed = toolEvent.Failed
		}
	}

	if _, err := e.appendEventLocked(record, contracts.EventKindToolCallCompleted, contracts.SessionEventPayload{
		CommandID: commandID,
		TurnID:    turnID,
		Tool: &contracts.ToolEventPayload{
			CallID:           call.ID,
			Name:             call.Name,
			Input:            call.Input,
			ConcurrencyClass: descriptor.ConcurrencyClass,
			ResultSummary:    resultSummary,
			Output:           output,
			Failed:           failed,
		},
	}); err != nil {
		return turnResult{}, err
	}

	if failed {
		err := fmt.Errorf("%s reported failure", call.Name)
		if emitErr := e.emitTurnFailureLocked(record, turnID, commandID, err, contracts.TerminalOutcomeToolFailure); emitErr != nil {
			return turnResult{}, emitErr
		}
		return turnResult{
			Outcome:          contracts.TerminalOutcomeToolFailure,
			AssistantMessage: fmt.Sprintf("Tool %s failed: %s", call.Name, resultSummary),
		}, nil
	}

	return turnResult{
		Outcome:          contracts.TerminalOutcomeSuccess,
		AssistantMessage: fmt.Sprintf("Tool %s completed. %s Output: %s", call.Name, resultSummary, output),
	}, nil
}

func (e *InMemoryEngine) lookupToolDescriptorLocked(record *sessionRecord, name string) (contracts.ToolDescriptor, error) {
	tools, err := e.toolRuntime.ListTools(context.Background(), contracts.SessionContext{
		SessionID: record.handle.SessionID,
		CWD:       record.handle.CWD,
		Mode:      record.handle.Mode,
		ProfileID: record.handle.ProfileID,
		Model:     record.handle.Model,
	})
	if err != nil {
		return contracts.ToolDescriptor{}, err
	}
	for _, descriptor := range tools {
		if descriptor.Name == name {
			return descriptor, nil
		}
	}
	return contracts.ToolDescriptor{}, fmt.Errorf("unknown tool: %s", name)
}

func requiresInteractivePermission(metadata map[string]string) bool {
	if metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(metadata["permission_mode"]), "ask")
}

func (e *InMemoryEngine) emitTurnFailureLocked(record *sessionRecord, turnID string, commandID string, cause error, outcome contracts.TerminalOutcome) error {
	return e.emitFailureLocked(record, turnID, commandID, contracts.FailureCategoryTool, "tool_execution_failed", cause, outcome)
}

func (e *InMemoryEngine) emitFailureLocked(record *sessionRecord, turnID string, commandID string, category contracts.FailureCategory, code string, cause error, outcome contracts.TerminalOutcome) error {
	return e.emitFailureDetailedLocked(record, turnID, commandID, category, code, cause, outcome, false)
}

func (e *InMemoryEngine) emitFailureDetailedLocked(record *sessionRecord, turnID string, commandID string, category contracts.FailureCategory, code string, cause error, outcome contracts.TerminalOutcome, retryable bool) error {
	if _, err := e.appendEventLocked(record, contracts.EventKindFailure, contracts.SessionEventPayload{
		CommandID: commandID,
		TurnID:    turnID,
		Failure: &contracts.FailurePayload{
			Category:  category,
			Code:      code,
			Message:   cause.Error(),
			Retryable: retryable,
		},
	}); err != nil {
		return err
	}
	record.summary.TerminalOutcome = outcome
	return nil
}

func (e *InMemoryEngine) applyProfileDefaults(req *contracts.StartSessionRequest) error {
	profile, err := e.resolveProfile(req.ProfileID, req.Model)
	if err != nil {
		if strings.TrimSpace(req.ProfileID) != "" && !provider.IsLegacyProfileID(req.ProfileID) {
			return err
		}
		return nil
	}

	if strings.TrimSpace(req.ProfileID) == "" || provider.IsLegacyProfileID(req.ProfileID) {
		req.ProfileID = profile.ID
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model = profile.DefaultModel
	}
	return nil
}

func (e *InMemoryEngine) resolveProfileSetting(currentProfileID string, currentModel string, nextProfileID string) (string, string, error) {
	profile, err := e.resolveProfile(nextProfileID, currentModel)
	if err != nil {
		return "", "", err
	}

	nextModel := currentModel
	currentProfile, currentErr := e.resolveProfile(currentProfileID, currentModel)
	if strings.TrimSpace(nextModel) == "" || (currentErr == nil && strings.TrimSpace(nextModel) == currentProfile.DefaultModel) {
		nextModel = profile.DefaultModel
	}
	return profile.ID, nextModel, nil
}

func (e *InMemoryEngine) resolveProfile(profileID string, model string) (contracts.AuthProfile, error) {
	if e.profileStore != nil {
		return e.profileStore.ResolveProfile(profileID, model)
	}
	return provider.ResolveSessionProfile(profileID, model), nil
}

func (e *InMemoryEngine) maybeRefreshOAuthProfileLocked(ctx context.Context, profile contracts.AuthProfile, force bool) (contracts.AuthProfile, error) {
	refreshed, changed, err := anthropicoauth.MaybeRefreshProfile(ctx, profile, force)
	if err != nil {
		return profile, err
	}
	if !changed {
		return profile, nil
	}
	if e.profileStore != nil {
		if err := e.profileStore.SaveProfile(refreshed); err != nil {
			return profile, err
		}
	}
	return refreshed, nil
}

func (e *InMemoryEngine) buildProfileStatus(ctx context.Context, profile contracts.AuthProfile) (contracts.ProfileStatus, error) {
	status := contracts.ProfileStatus{
		Profile: profile,
		Validation: contracts.ProfileValidationResult{
			Valid:   true,
			Message: "profile is valid",
		},
		Auth: describeProfileAuth(profile),
	}

	if e.providers != nil {
		validation, err := e.providers.ValidateProfile(ctx, profile)
		if err != nil {
			return contracts.ProfileStatus{}, err
		}
		status.Validation = validation

		models, err := e.providers.ListModels(ctx, profile)
		if err == nil {
			status.Models = models
		}
		if validation.Valid {
			model := strings.TrimSpace(profile.DefaultModel)
			if model == "" && len(status.Models) > 0 {
				model = status.Models[0]
			}
			capabilities, err := e.providers.Capabilities(ctx, profile, model)
			if err == nil {
				status.Capabilities = capabilities
			}
		}
	}
	return status, nil
}

func clearedProfileAuthSettings(profile contracts.AuthProfile) map[string]string {
	if len(profile.Settings) == 0 {
		return map[string]string{}
	}

	cleared := make(map[string]string, len(profile.Settings))
	for key, value := range profile.Settings {
		cleared[key] = value
	}
	for _, key := range []string{
		"credential_ref",
		"api_key",
		"access_token",
		"oauth_access_token",
		"oauth_refresh_token",
		"oauth_expires_at",
	} {
		delete(cleared, key)
	}
	return cleared
}

func describeProfileAuth(profile contracts.AuthProfile) contracts.ProfileAuthStatus {
	switch profile.Kind {
	case contracts.AuthProfileAnthropicOAuth:
		accessToken := strings.TrimSpace(profile.Settings["oauth_access_token"])
		expiresAt := strings.TrimSpace(profile.Settings["oauth_expires_at"])
		canRefresh := strings.TrimSpace(profile.Settings["oauth_refresh_token"]) != ""
		if accessToken == "" {
			return contracts.ProfileAuthStatus{
				State:      contracts.ProfileAuthStateLoggedOut,
				ExpiresAt:  expiresAt,
				CanRefresh: canRefresh,
			}
		}
		if expiresAt == "" {
			return contracts.ProfileAuthStatus{
				State:      contracts.ProfileAuthStateAuthenticated,
				CanRefresh: canRefresh,
			}
		}
		unixSeconds, err := strconv.ParseInt(expiresAt, 10, 64)
		if err != nil {
			return contracts.ProfileAuthStatus{
				State:      contracts.ProfileAuthStateAuthenticated,
				ExpiresAt:  expiresAt,
				CanRefresh: canRefresh,
			}
		}
		expiration := time.Unix(unixSeconds, 0).UTC()
		if time.Now().UTC().After(expiration) {
			return contracts.ProfileAuthStatus{
				State:      contracts.ProfileAuthStateExpired,
				ExpiresAt:  expiration.Format(time.RFC3339),
				CanRefresh: canRefresh,
			}
		}
		state := contracts.ProfileAuthStateAuthenticated
		if anthropicoauth.ShouldRefresh(profile) {
			state = contracts.ProfileAuthStateExpiring
		}
		return contracts.ProfileAuthStatus{
			State:      state,
			ExpiresAt:  expiration.Format(time.RFC3339),
			CanRefresh: canRefresh,
		}
	default:
		if hasStoredCredential(profile) {
			return contracts.ProfileAuthStatus{
				State: contracts.ProfileAuthStateConfigured,
			}
		}
		return contracts.ProfileAuthStatus{
			State: contracts.ProfileAuthStateLoggedOut,
		}
	}
}

func hasStoredCredential(profile contracts.AuthProfile) bool {
	for _, key := range []string{"credential_ref", "api_key", "access_token"} {
		if strings.TrimSpace(profile.Settings[key]) != "" {
			return true
		}
	}
	return false
}

func classifyProviderFailure(err error) (contracts.FailureCategory, string, contracts.TerminalOutcome, bool, string) {
	providerErr := provider.AsError(err)
	if providerErr == nil {
		return contracts.FailureCategoryProvider, "provider_completion_failed", contracts.TerminalOutcomeProviderFailure, false, fmt.Sprintf("Provider execution failed: %v", err)
	}

	switch providerErr.Code {
	case provider.ErrorCodeInvalidModel:
		return contracts.FailureCategoryProvider, string(provider.ErrorCodeInvalidModel), contracts.TerminalOutcomeValidationFailure, providerErr.Retryable, fmt.Sprintf("Invalid model: %s", providerErr.Message)
	case provider.ErrorCodeAuthUnavailable, provider.ErrorCodeAuthFailed:
		return contracts.FailureCategoryAuth, string(providerErr.Code), contracts.TerminalOutcomeProviderFailure, providerErr.Retryable, fmt.Sprintf("Provider auth failed: %s", providerErr.Message)
	case provider.ErrorCodeProviderRequestFailed:
		fallthrough
	default:
		return contracts.FailureCategoryProvider, string(providerErr.Code), contracts.TerminalOutcomeProviderFailure, providerErr.Retryable, fmt.Sprintf("Provider execution failed: %s", providerErr.Message)
	}
}
