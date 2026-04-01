package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/provider"
)

type nonStreamingAdapter struct{}

type limitedCapabilityAdapter struct{}

func (a *nonStreamingAdapter) Kind() contracts.ProviderKind {
	return contracts.ProviderAnthropic
}

func (a *nonStreamingAdapter) ListModels(_ context.Context, _ contracts.AuthProfile) ([]string, error) {
	return []string{"claude-sonnet-4-6"}, nil
}

func (a *nonStreamingAdapter) CountTokens(_ context.Context, _ contracts.AuthProfile, _ contracts.TokenCountRequest) (contracts.TokenCountResult, error) {
	return contracts.TokenCountResult{InputTokens: 1}, nil
}

func (a *nonStreamingAdapter) StreamCompletion(_ context.Context, _ contracts.AuthProfile, _ contracts.CompletionRequest) (<-chan contracts.ProviderEvent, error) {
	return nil, provider.ErrCompletionNotImplemented
}

func (a *nonStreamingAdapter) Complete(_ context.Context, _ contracts.AuthProfile, _ contracts.CompletionRequest) (contracts.CompletionResult, error) {
	return contracts.CompletionResult{
		Message: contracts.CanonicalMessage{
			Role:    "assistant",
			Content: "non-streaming fallback reply",
		},
	}, nil
}

func (a *nonStreamingAdapter) ValidateProfile(_ context.Context, _ contracts.AuthProfile) (contracts.ProfileValidationResult, error) {
	return contracts.ProfileValidationResult{Valid: true, Message: "profile is valid"}, nil
}

func (a *nonStreamingAdapter) Capabilities(_ context.Context, _ contracts.AuthProfile, _ string) (contracts.CapabilitySet, error) {
	return contracts.CapabilitySet{
		Streaming:         false,
		ToolCalling:       true,
		StructuredOutputs: true,
	}, nil
}

func (a *limitedCapabilityAdapter) Kind() contracts.ProviderKind {
	return contracts.ProviderAnthropic
}

func (a *limitedCapabilityAdapter) ListModels(_ context.Context, _ contracts.AuthProfile) ([]string, error) {
	return []string{"claude-sonnet-4-6"}, nil
}

func (a *limitedCapabilityAdapter) CountTokens(_ context.Context, _ contracts.AuthProfile, _ contracts.TokenCountRequest) (contracts.TokenCountResult, error) {
	return contracts.TokenCountResult{InputTokens: 1}, nil
}

func (a *limitedCapabilityAdapter) StreamCompletion(_ context.Context, _ contracts.AuthProfile, _ contracts.CompletionRequest) (<-chan contracts.ProviderEvent, error) {
	stream := make(chan contracts.ProviderEvent, 1)
	close(stream)
	return stream, nil
}

func (a *limitedCapabilityAdapter) Complete(_ context.Context, _ contracts.AuthProfile, _ contracts.CompletionRequest) (contracts.CompletionResult, error) {
	return contracts.CompletionResult{
		Message: contracts.CanonicalMessage{
			Role:    "assistant",
			Content: "limited capability reply",
		},
	}, nil
}

func (a *limitedCapabilityAdapter) ValidateProfile(_ context.Context, _ contracts.AuthProfile) (contracts.ProfileValidationResult, error) {
	return contracts.ProfileValidationResult{Valid: true, Message: "profile is valid"}, nil
}

func (a *limitedCapabilityAdapter) Capabilities(_ context.Context, _ contracts.AuthProfile, _ string) (contracts.CapabilitySet, error) {
	return contracts.CapabilitySet{
		Streaming:          true,
		ToolCalling:        false,
		StructuredOutputs:  false,
		DeferredToolSearch: false,
		ImageInput:         false,
		DocumentInput:      false,
	}, nil
}

func TestStartSessionRecordsAuthoritativeEventLog(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_start",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "anthropic-main",
		Model:     "claude-sonnet-4-6",
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

func TestStartSessionAppliesStoredDefaultProfileAndModel(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_defaults",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if handle.ProfileID != "anthropic-main" {
		t.Fatalf("expected stored default profile anthropic-main, got %s", handle.ProfileID)
	}
	if handle.Model != "claude-sonnet-4-6" {
		t.Fatalf("expected stored default model claude-sonnet-4-6, got %s", handle.Model)
	}
}

func TestListProfilesReturnsStoredProfilesWithValidation(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	profiles, err := runtime.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles returned error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 stored profiles, got %d", len(profiles))
	}
	if profiles[0].Profile.ID != "anthropic-main" {
		t.Fatalf("expected anthropic-main first, got %s", profiles[0].Profile.ID)
	}
	if !profiles[0].Validation.Valid {
		t.Fatalf("expected first profile to validate, got %#v", profiles[0].Validation)
	}
	if profiles[0].Auth.State != contracts.ProfileAuthStateLoggedOut {
		t.Fatalf("expected seeded anthropic oauth profile to start logged_out, got %s", profiles[0].Auth.State)
	}
	if len(profiles[0].Models) == 0 {
		t.Fatalf("expected provider models for first profile")
	}
	if !profiles[0].Capabilities.Streaming || !profiles[0].Capabilities.ToolCalling {
		t.Fatalf("expected seeded anthropic profile capabilities, got %#v", profiles[0].Capabilities)
	}
}

func TestSaveProfilePersistsNewDefaultProfile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	status, err := runtime.SaveProfile(ctx, contracts.AuthProfile{
		ID:           "openrouter-alt",
		Kind:         contracts.AuthProfileOpenRouterAPIKey,
		Provider:     contracts.ProviderOpenRouter,
		DisplayName:  "OpenRouter Alt",
		DefaultModel: "openrouter/auto",
		Settings: map[string]string{
			"credential_ref": "env://OPENROUTER_API_KEY",
			"api_base":       "https://openrouter.ai/api/v1",
		},
	}, true)
	if err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}
	if !status.Validation.Valid {
		t.Fatalf("expected valid saved profile, got %#v", status.Validation)
	}

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_saved_default",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}
	if handle.ProfileID != "openrouter-alt" {
		t.Fatalf("expected saved default profile to resolve, got %s", handle.ProfileID)
	}
	if handle.Model != "openrouter/auto" {
		t.Fatalf("expected saved default model to resolve, got %s", handle.Model)
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
	eighth := nextEvent(t, stream)
	if third.Kind != contracts.EventKindUserMessageAccepted {
		t.Fatalf("expected user_message_accepted, got %s", third.Kind)
	}
	if fourth.Kind != contracts.EventKindLifecycle || fourth.Payload.LifecycleName != "turn_started" {
		t.Fatalf("expected lifecycle turn_started, got %s (%s)", fourth.Kind, fourth.Payload.LifecycleName)
	}
	if third.Payload.Message == nil || third.Payload.Message.Content != "hello" {
		t.Fatalf("unexpected message payload: %#v", third.Payload.Message)
	}
	if fifth.Kind != contracts.EventKindAssistantDelta {
		t.Fatalf("expected assistant_delta, got %s", fifth.Kind)
	}
	if fifth.Payload.Message == nil || !strings.Contains(fifth.Payload.Message.Content, "Anthropic response from") {
		t.Fatalf("expected provider-backed assistant delta, got %#v", fifth.Payload.Message)
	}
	if sixth.Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", sixth.Kind)
	}
	if seventh.Kind != contracts.EventKindLifecycle || seventh.Payload.LifecycleName != "turn_completed" {
		t.Fatalf("expected lifecycle turn_completed, got %s (%s)", seventh.Kind, seventh.Payload.LifecycleName)
	}
	if seventh.Payload.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		t.Fatalf("expected success terminal outcome, got %s", seventh.Payload.TerminalOutcome)
	}
	if eighth.Kind != contracts.EventKindSessionState {
		t.Fatalf("expected session_state after turn completion, got %s", eighth.Kind)
	}
	if eighth.Payload.State == nil || eighth.Payload.State.TurnCount != 1 {
		t.Fatalf("expected turn count 1 in state snapshot, got %#v", eighth.Payload.State)
	}
}

func TestNonToolTurnRoutesThroughOpenRouterWhenModelRequiresIt(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_openrouter",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		Model:     "openrouter/auto",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello from openrouter turn",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", events[5].Kind)
	}
	if events[5].Payload.Message == nil || !strings.Contains(events[5].Payload.Message.Content, "OpenRouter response from openrouter/auto") {
		t.Fatalf("expected openrouter-backed assistant message, got %#v", events[5].Payload.Message)
	}
}

func TestProviderTurnFallsBackWhenStreamingCapabilityDisabled(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()
	runtime.providers = provider.NewRegistry(&nonStreamingAdapter{})

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_no_streaming",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "anthropic-main",
		Model:     "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello fallback",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[4].Kind != contracts.EventKindWarning {
		t.Fatalf("expected warning event when streaming is unsupported, got %s", events[4].Kind)
	}
	if !strings.Contains(events[4].Payload.Warning, "does not support streaming") {
		t.Fatalf("expected streaming fallback warning, got %q", events[4].Payload.Warning)
	}
	if events[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message after warning fallback, got %s", events[5].Kind)
	}
	if events[5].Payload.Message == nil || events[5].Payload.Message.Content != "non-streaming fallback reply" {
		t.Fatalf("expected fallback assistant reply, got %#v", events[5].Payload.Message)
	}
	for _, event := range events {
		if event.Kind == contracts.EventKindAssistantDelta {
			t.Fatalf("did not expect assistant_delta when streaming capability is disabled")
		}
	}
}

func TestUpdateSessionSettingChangesActiveModel(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_model_switch",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		Model:     "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUpdateSessionSetting,
		Payload: contracts.SessionCommandPayload{
			SettingKey:   "model",
			SettingValue: "claude-opus-4-6",
		},
	}); err != nil {
		t.Fatalf("UpdateSessionSetting returned error: %v", err)
	}
	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello after model switch",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary returned error: %v", err)
	}
	if summary.Model != "claude-opus-4-6" {
		t.Fatalf("expected updated model in summary, got %q", summary.Model)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	lastAssistant := contracts.CanonicalMessage{}
	for _, event := range events {
		if event.Kind == contracts.EventKindAssistantMessage && event.Payload.Message != nil {
			lastAssistant = *event.Payload.Message
		}
	}
	if !strings.Contains(lastAssistant.Content, "Anthropic response from claude-opus-4-6") {
		t.Fatalf("expected anthropic response after model switch, got %q", lastAssistant.Content)
	}
}

func TestProfileSwitchAdoptsStoredProfileDefaults(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_profile_switch_defaults",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUpdateSessionSetting,
		Payload: contracts.SessionCommandPayload{
			SettingKey:   "profile_id",
			SettingValue: "openrouter-main",
		},
	}); err != nil {
		t.Fatalf("UpdateSessionSetting returned error: %v", err)
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary returned error: %v", err)
	}
	if summary.ProfileID != "openrouter-main" {
		t.Fatalf("expected openrouter-main profile after switch, got %s", summary.ProfileID)
	}
	if summary.Model != "anthropic/claude-sonnet-4.5" {
		t.Fatalf("expected openrouter default model after profile switch, got %s", summary.Model)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello after profile switch",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	lastAssistant := contracts.CanonicalMessage{}
	for _, event := range events {
		if event.Kind == contracts.EventKindAssistantMessage && event.Payload.Message != nil {
			lastAssistant = *event.Payload.Message
		}
	}
	if !strings.Contains(lastAssistant.Content, "OpenRouter response from anthropic/claude-sonnet-4.5") {
		t.Fatalf("expected stored openrouter default model in assistant response, got %q", lastAssistant.Content)
	}
}

func TestInvalidAnthropicModelFailsBeforeProviderCall(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_invalid_model",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "anthropic-main",
		Model:     "claude-not-real",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello invalid model",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[4].Kind != contracts.EventKindFailure {
		t.Fatalf("expected failure event at sequence 5, got %s", events[4].Kind)
	}
	if events[4].Payload.Failure == nil || events[4].Payload.Failure.Code != "invalid_model" {
		t.Fatalf("expected invalid_model failure payload, got %#v", events[4].Payload.Failure)
	}
	if events[4].Payload.Failure.Category != contracts.FailureCategoryProvider {
		t.Fatalf("expected provider failure category, got %#v", events[4].Payload.Failure)
	}
	if events[6].Kind != contracts.EventKindLifecycle || events[6].Payload.TerminalOutcome != contracts.TerminalOutcomeValidationFailure {
		t.Fatalf("expected validation_failure turn completion, got %s (%s)", events[6].Kind, events[6].Payload.TerminalOutcome)
	}
	if events[7].Payload.State == nil || events[7].Payload.State.TerminalOutcome != contracts.TerminalOutcomeValidationFailure {
		t.Fatalf("expected validation failure in state snapshot, got %#v", events[7].Payload.State)
	}
}

func TestOpenRouterCustomModelRemainsUsable(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_openrouter_custom",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "openrouter-main",
		Model:     "my/custom-model",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello custom openrouter",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", events[5].Kind)
	}
	if events[5].Payload.Message == nil || !strings.Contains(events[5].Payload.Message.Content, "OpenRouter response from my/custom-model") {
		t.Fatalf("expected custom openrouter model response, got %#v", events[5].Payload.Message)
	}
}

func TestProviderTurnFailsWhenRequestedCapabilityIsUnsupported(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name     string
		metadata map[string]string
	}{
		{name: "tool calling", metadata: map[string]string{"tool_choice": "auto"}},
		{name: "structured outputs", metadata: map[string]string{"structured_output": "true"}},
		{name: "deferred tool search", metadata: map[string]string{"deferred_tool_search": "true"}},
		{name: "image input", metadata: map[string]string{"input_kind": "image"}},
		{name: "document input", metadata: map[string]string{"input_kind": "document"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewInMemoryEngine()
			runtime.providers = provider.NewRegistry(&limitedCapabilityAdapter{})

			handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
				SessionID: "sess_capability_" + strings.ReplaceAll(tc.name, " ", "_"),
				CWD:       "/tmp/project",
				Mode:      contracts.SessionModeInteractive,
				ProfileID: "anthropic-main",
				Model:     "claude-sonnet-4-6",
			})
			if err != nil {
				t.Fatalf("StartSession returned error: %v", err)
			}

			if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
				Kind: contracts.CommandKindUserInput,
				Payload: contracts.SessionCommandPayload{
					Text:     "hello capability gate",
					Source:   contracts.MessageSourceInteractive,
					Metadata: tc.metadata,
				},
			}); err != nil {
				t.Fatalf("SendCommand returned error: %v", err)
			}

			events, err := runtime.ListEvents(ctx, handle.SessionID)
			if err != nil {
				t.Fatalf("ListEvents returned error: %v", err)
			}
			if events[4].Kind != contracts.EventKindFailure {
				t.Fatalf("expected failure event at sequence 5, got %s", events[4].Kind)
			}
			if events[4].Payload.Failure == nil || events[4].Payload.Failure.Code != "capability_mismatch" {
				t.Fatalf("expected capability_mismatch failure payload, got %#v", events[4].Payload.Failure)
			}
			if events[6].Kind != contracts.EventKindLifecycle || events[6].Payload.TerminalOutcome != contracts.TerminalOutcomeValidationFailure {
				t.Fatalf("expected validation_failure turn completion, got %s (%s)", events[6].Kind, events[6].Payload.TerminalOutcome)
			}
			if events[7].Payload.State == nil || events[7].Payload.State.TerminalOutcome != contracts.TerminalOutcomeValidationFailure {
				t.Fatalf("expected validation failure in state snapshot, got %#v", events[7].Payload.State)
			}
		})
	}
}

func TestProviderTurnWarnsAndFallsBackWhenCapabilityFallbackIsAllowed(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_capability_fallback",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "openrouter-main",
		Model:     "openrouter/auto",
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello fallback capability",
			Source: contracts.MessageSourceInteractive,
			Metadata: map[string]string{
				"deferred_tool_search":     "true",
				"allow_capability_fallback": "true",
			},
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[4].Kind != contracts.EventKindWarning {
		t.Fatalf("expected warning event at sequence 5, got %s", events[4].Kind)
	}
	if !strings.Contains(events[4].Payload.Warning, "continuing with capability fallback") {
		t.Fatalf("expected capability fallback warning, got %q", events[4].Payload.Warning)
	}
	if events[5].Kind != contracts.EventKindAssistantDelta {
		t.Fatalf("expected assistant delta after fallback warning, got %s", events[5].Kind)
	}
	if events[6].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant message after fallback warning, got %s", events[6].Kind)
	}
	if events[8].Kind != contracts.EventKindSessionState || events[8].Payload.State == nil || events[8].Payload.State.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		t.Fatalf("expected success state snapshot after fallback, got %#v", events[8].Payload.State)
	}
}

func TestMissingEnvCredentialProducesAuthFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	if _, err := runtime.SaveProfile(ctx, contracts.AuthProfile{
		ID:           "anthropic-live-missing",
		Kind:         contracts.AuthProfileAnthropicAPIKey,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Live Missing",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"credential_ref": "env://ANTHROPIC_TEST_KEY_MISSING",
			"api_base":       "https://api.anthropic.com",
		},
	}, true); err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_missing_auth",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello missing auth",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[4].Kind != contracts.EventKindFailure {
		t.Fatalf("expected failure event at sequence 5, got %s", events[4].Kind)
	}
	if events[4].Payload.Failure == nil || events[4].Payload.Failure.Category != contracts.FailureCategoryAuth {
		t.Fatalf("expected auth failure payload, got %#v", events[4].Payload.Failure)
	}
	if events[4].Payload.Failure.Code != "auth_unavailable" {
		t.Fatalf("expected auth_unavailable code, got %#v", events[4].Payload.Failure)
	}
	if events[4].Payload.Failure.Retryable {
		t.Fatalf("expected auth_unavailable to be non-retryable")
	}
	if events[6].Kind != contracts.EventKindLifecycle || events[6].Payload.TerminalOutcome != contracts.TerminalOutcomeProviderFailure {
		t.Fatalf("expected provider_failure outcome, got %s (%s)", events[6].Kind, events[6].Payload.TerminalOutcome)
	}
}

func TestAnthropicOAuthExpiredTokenRefreshesAndPersists(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	var sawBearer string
	var refreshCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth/token":
			refreshCalls++
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"refreshed-access","refresh_token":"refreshed-refresh","expires_in":7200,"scope":"user:profile user:inference"}`))
		case "/v1/messages":
			sawBearer = r.Header.Get("authorization")
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"oauth refreshed reply"}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	if _, err := runtime.SaveProfile(ctx, contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Main",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_access_token":  "expired-access",
			"oauth_refresh_token": "refresh-token",
			"oauth_expires_at":    strconv.FormatInt(time.Now().UTC().Add(-time.Minute).Unix(), 10),
			"oauth_scopes":        "user:profile user:inference",
			"oauth_token_url":     server.URL + "/v1/oauth/token",
			"oauth_client_id":     "client-123",
			"oauth_host":          "https://claude.ai",
			"account_scope":       "claude",
			"api_base":            server.URL,
		},
	}, true); err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_oauth_refresh",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello oauth refresh",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	if refreshCalls != 1 {
		t.Fatalf("expected one refresh call, got %d", refreshCalls)
	}
	if sawBearer != "Bearer refreshed-access" {
		t.Fatalf("expected refreshed bearer token on message request, got %q", sawBearer)
	}

	store, err := provider.NewFileProfileStore(root)
	if err != nil {
		t.Fatalf("NewFileProfileStore returned error: %v", err)
	}
	profile, err := store.GetProfile("anthropic-main")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}
	if profile.Settings["oauth_access_token"] != "refreshed-access" {
		t.Fatalf("expected persisted refreshed access token, got %q", profile.Settings["oauth_access_token"])
	}
}

func TestAnthropicOAuthAuthFailureRefreshesAndRetriesOnce(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	var refreshCalls int
	var messageCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth/token":
			refreshCalls++
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"second-access","refresh_token":"second-refresh","expires_in":7200,"scope":"user:profile user:inference"}`))
		case "/v1/messages":
			messageCalls++
			if messageCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"oauth token expired"}}`))
				return
			}
			if got := r.Header.Get("authorization"); got != "Bearer second-access" {
				t.Fatalf("expected retried request to use refreshed token, got %q", got)
			}
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"oauth retry reply"}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}

	if _, err := runtime.SaveProfile(ctx, contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Main",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_access_token":  "first-access",
			"oauth_refresh_token": "refresh-token",
			"oauth_expires_at":    strconv.FormatInt(time.Now().UTC().Add(30*time.Minute).Unix(), 10),
			"oauth_scopes":        "user:profile user:inference",
			"oauth_token_url":     server.URL + "/v1/oauth/token",
			"oauth_client_id":     "client-123",
			"oauth_host":          "https://claude.ai",
			"account_scope":       "claude",
			"api_base":            server.URL,
		},
	}, true); err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_oauth_retry",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if err := runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "hello oauth retry",
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	if refreshCalls != 1 {
		t.Fatalf("expected one refresh call after auth failure, got %d", refreshCalls)
	}
	if messageCalls != 2 {
		t.Fatalf("expected two message attempts, got %d", messageCalls)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if events[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", events[5].Kind)
	}
	if events[5].Payload.Message == nil || !strings.Contains(events[5].Payload.Message.Content, "oauth retry reply") {
		t.Fatalf("expected assistant message after oauth retry, got %#v", events[5].Payload.Message)
	}
}

func TestListProfilesMarksExpiringAnthropicOAuthProfile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	runtime, err := NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}
	if _, err := runtime.SaveProfile(ctx, contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Main",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_access_token":  "access-token",
			"oauth_refresh_token": "refresh-token",
			"oauth_expires_at":    strconv.FormatInt(time.Now().UTC().Add(4*time.Minute).Unix(), 10),
			"oauth_host":          "https://claude.ai",
			"account_scope":       "claude",
		},
	}, false); err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	profiles, err := runtime.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles returned error: %v", err)
	}
	if profiles[0].Auth.State != contracts.ProfileAuthStateExpiring {
		t.Fatalf("expected expiring auth state, got %s", profiles[0].Auth.State)
	}
	if !profiles[0].Auth.CanRefresh {
		t.Fatalf("expected expiring profile to report can_refresh")
	}
	if profiles[0].Auth.ExpiresAt == "" {
		t.Fatalf("expected expiring profile to expose expires_at")
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

func TestToolTurnWaitsForInteractivePermissionResolution(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_permission_wait",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "tool:pwd",
			Source: contracts.MessageSourceInteractive,
			Metadata: map[string]string{
				"permission_mode": "ask",
			},
		},
	})
	if err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("expected 6 events before permission resolution, got %d", len(events))
	}
	if events[5].Kind != contracts.EventKindPermissionRequested {
		t.Fatalf("expected permission_requested, got %s", events[5].Kind)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindApprovePermission,
		Payload: contracts.SessionCommandPayload{
			RequestID: "perm_tool_turn_1_1",
		},
	})
	if err != nil {
		t.Fatalf("ApprovePermission returned error: %v", err)
	}

	events, err = runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 12 {
		t.Fatalf("expected 12 events after permission approval, got %d", len(events))
	}
	if events[6].Kind != contracts.EventKindPermissionResolved {
		t.Fatalf("expected permission_resolved after approval, got %s", events[6].Kind)
	}
	if events[9].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message after approval flow, got %s", events[9].Kind)
	}
	if events[10].Payload.TerminalOutcome != contracts.TerminalOutcomeSuccess {
		t.Fatalf("expected success outcome after approval, got %s", events[10].Payload.TerminalOutcome)
	}
}

func TestDenyPermissionCompletesTurnWithFailure(t *testing.T) {
	ctx := context.Background()
	runtime := NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "sess_permission_deny",
		CWD:       "/tmp/project",
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   "tool:pwd",
			Source: contracts.MessageSourceInteractive,
			Metadata: map[string]string{
				"permission_mode": "ask",
			},
		},
	})
	if err != nil {
		t.Fatalf("SendCommand returned error: %v", err)
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindDenyPermission,
		Payload: contracts.SessionCommandPayload{
			RequestID: "perm_tool_turn_1_1",
		},
	})
	if err != nil {
		t.Fatalf("DenyPermission returned error: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 11 {
		t.Fatalf("expected 11 events after permission denial, got %d", len(events))
	}
	if events[6].Kind != contracts.EventKindPermissionResolved {
		t.Fatalf("expected permission_resolved, got %s", events[6].Kind)
	}
	if events[6].Payload.Permission == nil || events[6].Payload.Permission.Resolution != "deny" {
		t.Fatalf("expected deny resolution, got %#v", events[6].Payload.Permission)
	}
	if events[7].Kind != contracts.EventKindFailure {
		t.Fatalf("expected failure event after denial, got %s", events[7].Kind)
	}
	if events[7].Payload.Failure == nil || events[7].Payload.Failure.Category != contracts.FailureCategoryPermission {
		t.Fatalf("expected permission failure payload, got %#v", events[7].Payload.Failure)
	}
	if events[10].Kind != contracts.EventKindSessionState || events[10].Payload.State == nil || events[10].Payload.State.TerminalOutcome != contracts.TerminalOutcomeTaskFailure {
		t.Fatalf("expected task failure state after denial, got %#v", events[10].Payload.State)
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
