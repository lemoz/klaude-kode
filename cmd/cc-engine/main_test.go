package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdossman/klaude-kode/internal/auth/anthropicoauth"
	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
	"github.com/cdossman/klaude-kode/internal/harness"
)

func TestRunJSONFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-prompt=hello world",
		"-session-id=test-json",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var got result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse json output: %v", err)
	}

	if got.Engine != "cc-engine" {
		t.Fatalf("expected engine cc-engine, got %q", got.Engine)
	}
	if got.Summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected closed status, got %s", got.Summary.Status)
	}
	if len(got.Events) != 10 {
		t.Fatalf("expected 10 events after headless close, got %d", len(got.Events))
	}
	if got.Events[len(got.Events)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got.Events[len(got.Events)-1].Kind)
	}
	if got.Events[4].Kind != contracts.EventKindAssistantDelta {
		t.Fatalf("expected assistant_delta in event stream, got %s", got.Events[4].Kind)
	}
	if got.Events[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message in event stream, got %s", got.Events[5].Kind)
	}
}

func TestRunTextFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=text",
		"-prompt=text run",
		"-session-id=test-text",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc-engine headless session") {
		t.Fatalf("expected text header, got %q", output)
	}
	if !strings.Contains(output, "status: closed") {
		t.Fatalf("expected closed status in text output, got %q", output)
	}
	if !strings.Contains(output, "closed_reason: headless_complete") {
		t.Fatalf("expected headless close reason, got %q", output)
	}
}

func TestRunEventsFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=events",
		"-prompt=event run",
		"-session-id=test-events",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 jsonl events, got %d", len(lines))
	}

	var last contracts.SessionEvent
	for _, line := range lines {
		var event contracts.SessionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("failed to parse event line %q: %v", line, err)
		}
		last = event
	}

	if last.Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final streamed event session_closed, got %s", last.Kind)
	}
}

func TestRunProfilesJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-profiles",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got profileCatalogResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse profile json output: %v", err)
	}
	if got.Engine != "cc-engine" {
		t.Fatalf("expected engine cc-engine, got %q", got.Engine)
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got.Profiles))
	}
	if got.Profiles[0].Profile.ID != "anthropic-main" {
		t.Fatalf("expected anthropic-main first, got %s", got.Profiles[0].Profile.ID)
	}
	if !got.Profiles[0].Validation.Valid {
		t.Fatalf("expected first profile to validate, got %#v", got.Profiles[0].Validation)
	}
	if !got.Profiles[0].Capabilities.Streaming || !got.Profiles[0].Capabilities.ToolCalling {
		t.Fatalf("expected profile capabilities in catalog, got %#v", got.Profiles[0].Capabilities)
	}
}

func TestRunModelsJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-models",
		"-profile-id=openrouter-main",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got modelCatalogResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse model catalog output: %v", err)
	}
	if got.ProfileID != "openrouter-main" {
		t.Fatalf("expected openrouter-main catalog, got %s", got.ProfileID)
	}
	if len(got.Models) == 0 {
		t.Fatalf("expected model list in catalog")
	}
	if !got.Capabilities.Streaming {
		t.Fatalf("expected streaming capability in catalog, got %#v", got.Capabilities)
	}
}

func TestRunStatusJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state-root")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{
		"-format=json",
		"-prompt=status seed",
		"-session-id=status-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("seed run returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-status",
		"-resume-session=status-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("status run returned error: %v", err)
	}

	var got sessionStatusResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse status json output: %v", err)
	}
	if got.Session != "status-target" {
		t.Fatalf("expected status-target session, got %s", got.Session)
	}
	if got.Summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected closed status, got %s", got.Summary.Status)
	}
	if got.Summary.EventCount == 0 {
		t.Fatalf("expected event count in summary, got %#v", got.Summary)
	}
}

func TestRunExportReplayPackJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state-root")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{
		"-format=json",
		"-prompt=replay seed",
		"-session-id=replay-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("seed run returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-export-replay-pack",
		"-resume-session=replay-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("export replay run returned error: %v", err)
	}

	var got contracts.ReplayPack
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse replay pack output: %v", err)
	}
	if got.Session.SessionID != "replay-target" {
		t.Fatalf("expected replay-target session, got %s", got.Session.SessionID)
	}
	if got.Summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected closed summary in replay pack, got %s", got.Summary.Status)
	}
	if len(got.Events) == 0 {
		t.Fatalf("expected replay pack events")
	}
}

func TestRunValidateCandidateJSON(t *testing.T) {
	root := createValidCandidateRoot(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"-format=json",
		"-validate-candidate",
		"-cwd=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got harness.CandidateValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse validation output: %v", err)
	}
	if !got.Valid {
		t.Fatalf("expected valid candidate result, got %#v", got)
	}
}

func TestRunReplayEvalJSON(t *testing.T) {
	candidateRoot := createValidCandidateRoot(t)
	replayPath := writeReplayPack(t, t.TempDir(), contracts.TerminalOutcomeSuccess)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"-format=json",
		"-run-replay-eval",
		"-cwd=" + candidateRoot,
		"-replay-path=" + replayPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got harness.EvalRun
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse replay eval output: %v", err)
	}
	if got.Status != harness.EvalRunStatusCompleted {
		t.Fatalf("expected completed replay eval, got %#v", got)
	}
	if got.Score != 1 {
		t.Fatalf("expected score 1, got %#v", got)
	}
	if got.ReplayPath != replayPath {
		t.Fatalf("expected replay path %s, got %s", replayPath, got.ReplayPath)
	}
}

func TestRunUpsertProfileMakesNewDefault(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-upsert-profile",
		"-profile-id=openrouter-alt",
		"-provider=openrouter",
		"-profile-kind=openrouter_api_key",
		"-display-name=OpenRouter Alt",
		"-default-model=openrouter/auto",
		"-credential-ref=env://OPENROUTER_API_KEY",
		"-make-default",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("upsert run returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-prompt=hello after profile save",
		"-session-id=profile-save-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("follow-up run returned error: %v", err)
	}

	var got result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse follow-up json output: %v", err)
	}
	if got.Session.ProfileID != "openrouter-alt" {
		t.Fatalf("expected saved default profile openrouter-alt, got %s", got.Session.ProfileID)
	}
	if got.Session.Model != "openrouter/auto" {
		t.Fatalf("expected saved default model openrouter/auto, got %s", got.Session.Model)
	}
}

func TestRunAnthropicOAuthLoginSavesDefaultProfile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	original := performAnthropicOAuthLogin
	t.Cleanup(func() {
		performAnthropicOAuthLogin = original
	})
	performAnthropicOAuthLogin = func(_ context.Context, opts anthropicoauth.LoginOptions) (anthropicoauth.LoginResult, error) {
		return anthropicoauth.LoginResult{
			AuthURL: "https://claude.com/cai/oauth/authorize",
			Profile: contracts.AuthProfile{
				ID:           opts.ProfileID,
				Kind:         contracts.AuthProfileAnthropicOAuth,
				Provider:     contracts.ProviderAnthropic,
				DisplayName:  opts.DisplayName,
				DefaultModel: opts.DefaultModel,
				Settings: map[string]string{
					"oauth_host":          "https://claude.ai",
					"account_scope":       "claude",
					"oauth_access_token":  "oauth-access",
					"oauth_refresh_token": "oauth-refresh",
					"api_base":            "https://api.anthropic.com",
				},
			},
		}, nil
	}

	err := run([]string{
		"-format=json",
		"-anthropic-oauth-login",
		"-profile-id=anthropic-main",
		"-display-name=Anthropic Main",
		"-default-model=claude-sonnet-4-6",
		"-make-default",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("oauth login run returned error: %v", err)
	}

	var catalog profileCatalogResult
	if err := json.Unmarshal(stdout.Bytes(), &catalog); err != nil {
		t.Fatalf("failed to parse oauth profile catalog: %v", err)
	}
	if len(catalog.Profiles) == 0 {
		t.Fatalf("expected at least one profile after oauth login")
	}
	if catalog.Profiles[0].Profile.ID != "anthropic-main" {
		t.Fatalf("expected anthropic-main profile after oauth login, got %s", catalog.Profiles[0].Profile.ID)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-prompt=hello oauth default",
		"-session-id=oauth-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("follow-up run returned error: %v", err)
	}

	var got result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse follow-up json output: %v", err)
	}
	if got.Session.ProfileID != "anthropic-main" {
		t.Fatalf("expected anthropic-main as saved default, got %s", got.Session.ProfileID)
	}
}

func TestRunLogoutProfileClearsStoredAnthropicOAuthTokens(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	runtime, err := engine.NewFileBackedEngine(root)
	if err != nil {
		t.Fatalf("NewFileBackedEngine returned error: %v", err)
	}
	if _, err := runtime.SaveProfile(context.Background(), contracts.AuthProfile{
		ID:           "anthropic-main",
		Kind:         contracts.AuthProfileAnthropicOAuth,
		Provider:     contracts.ProviderAnthropic,
		DisplayName:  "Anthropic Main",
		DefaultModel: "claude-sonnet-4-6",
		Settings: map[string]string{
			"oauth_host":          "https://claude.ai",
			"account_scope":       "claude",
			"oauth_access_token":  "oauth-access",
			"oauth_refresh_token": "oauth-refresh",
			"oauth_expires_at":    "9999999999",
			"oauth_client_id":     "client-id",
		},
	}, false); err != nil {
		t.Fatalf("SaveProfile returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-logout-profile=anthropic-main",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("logout profile run returned error: %v", err)
	}

	var catalog profileCatalogResult
	if err := json.Unmarshal(stdout.Bytes(), &catalog); err != nil {
		t.Fatalf("failed to parse logout catalog: %v", err)
	}
	if len(catalog.Profiles) == 0 {
		t.Fatalf("expected profile catalog after logout")
	}
	if catalog.Profiles[0].Validation.Valid {
		t.Fatalf("expected logged out anthropic profile to be invalid until relogin")
	}
	if catalog.Profiles[0].Auth.State != contracts.ProfileAuthStateLoggedOut {
		t.Fatalf("expected logged out auth state, got %s", catalog.Profiles[0].Auth.State)
	}
}

func TestResumePersistedSession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state-root")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{
		"-format=json",
		"-prompt=resume seed",
		"-session-id=resume-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("initial run returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = run([]string{
		"-format=json",
		"-resume-session=resume-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("resume run returned error: %v", err)
	}

	var got result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse resumed json output: %v", err)
	}
	if got.Session.SessionID != "resume-target" {
		t.Fatalf("expected resumed session resume-target, got %s", got.Session.SessionID)
	}
	if got.Summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected resumed status closed, got %s", got.Summary.Status)
	}
	if len(got.Events) != 10 {
		t.Fatalf("expected 10 replayed events, got %d", len(got.Events))
	}
}

func createValidCandidateRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"cmd/cc", "cmd/cc-engine", "shell", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
	}
	for _, file := range []string{
		"cmd/cc/main.go",
		"cmd/cc-engine/main.go",
		"shell/package.json",
		"docs/05-roadmap.md",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("stub"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}
	return root
}

func writeReplayPack(t *testing.T, root string, terminalOutcome contracts.TerminalOutcome) string {
	t.Helper()

	replayPath := filepath.Join(root, "replay.json")
	replay := contracts.ReplayPack{
		SchemaVersion: contracts.SchemaVersionV1,
		Session: contracts.SessionHandle{
			SessionID: "replay-session",
		},
		Summary: contracts.SessionSummary{
			SessionID:       "replay-session",
			Status:          contracts.SessionStatusClosed,
			TerminalOutcome: terminalOutcome,
		},
		Events: []contracts.SessionEvent{
			{
				SchemaVersion: contracts.SchemaVersionV1,
				SessionID:     "replay-session",
				Sequence:      1,
				Kind:          contracts.EventKindSessionClosed,
			},
		},
	}
	encoded, err := json.Marshal(replay)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(replayPath, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return replayPath
}

func TestRunToolPromptIncludesPermissionAndToolEvents(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-prompt=tool:pwd",
		"-session-id=tool-run",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	var got result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse tool json output: %v", err)
	}
	if len(got.Events) != 14 {
		t.Fatalf("expected 14 events for tool prompt, got %d", len(got.Events))
	}
	if got.Events[5].Kind != contracts.EventKindPermissionRequested {
		t.Fatalf("expected permission_requested, got %s", got.Events[5].Kind)
	}
	if got.Events[8].Kind != contracts.EventKindToolCallCompleted {
		t.Fatalf("expected tool_call_completed, got %s", got.Events[8].Kind)
	}
	if got.Events[8].Payload.Tool == nil || got.Events[8].Payload.Tool.Output == "" {
		t.Fatalf("expected tool output in completed event, got %#v", got.Events[8].Payload.Tool)
	}
	if got.Events[len(got.Events)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got.Events[len(got.Events)-1].Kind)
	}
}

func TestRunStdioTransportStreamsInteractiveSession(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	stdin := strings.NewReader(strings.Join([]string{
		`{"kind":"user_input","payload":{"text":"hello over stdio"}}`,
		`{"kind":"close_session","payload":{"reason":"stdio_test_complete"}}`,
		"",
	}, "\n"))

	err := runWithInput([]string{
		"-transport=stdio",
		"-format=events",
		"-session-id=stdio-session",
		"-state-root=" + root,
	}, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithInput returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 stdio events, got %d", len(lines))
	}

	var got []contracts.SessionEvent
	for _, line := range lines {
		var event contracts.SessionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("failed to parse stdio event line %q: %v", line, err)
		}
		got = append(got, event)
	}

	if got[0].Kind != contracts.EventKindSessionStarted {
		t.Fatalf("expected first event session_started, got %s", got[0].Kind)
	}
	if got[2].Kind != contracts.EventKindUserMessageAccepted {
		t.Fatalf("expected user_message_accepted, got %s", got[2].Kind)
	}
	if got[4].Kind != contracts.EventKindAssistantDelta {
		t.Fatalf("expected assistant_delta, got %s", got[4].Kind)
	}
	if got[5].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", got[5].Kind)
	}
	if got[len(got)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got[len(got)-1].Kind)
	}
	if got[len(got)-1].Payload.Reason != "stdio_test_complete" {
		t.Fatalf("expected stdio close reason, got %q", got[len(got)-1].Payload.Reason)
	}
}

func TestRunStdioTransportSupportsPermissionApproval(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	stdin := strings.NewReader(strings.Join([]string{
		`{"kind":"user_input","payload":{"text":"tool:pwd","metadata":{"permission_mode":"ask"}}}`,
		`{"kind":"approve_permission","payload":{"request_id":"perm_tool_turn_1_1"}}`,
		`{"kind":"close_session","payload":{"reason":"stdio_permission_complete"}}`,
		"",
	}, "\n"))

	err := runWithInput([]string{
		"-transport=stdio",
		"-format=events",
		"-session-id=stdio-permission",
		"-state-root=" + root,
	}, stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWithInput returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 14 {
		t.Fatalf("expected 14 stdio events with permission approval, got %d", len(lines))
	}

	var got []contracts.SessionEvent
	for _, line := range lines {
		var event contracts.SessionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("failed to parse stdio event line %q: %v", line, err)
		}
		got = append(got, event)
	}

	if got[5].Kind != contracts.EventKindPermissionRequested {
		t.Fatalf("expected permission_requested, got %s", got[5].Kind)
	}
	if got[6].Kind != contracts.EventKindPermissionResolved {
		t.Fatalf("expected permission_resolved, got %s", got[6].Kind)
	}
	if got[6].Payload.Permission == nil || got[6].Payload.Permission.Actor != "user" {
		t.Fatalf("expected user approval actor, got %#v", got[6].Payload.Permission)
	}
	if got[9].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message after approval, got %s", got[9].Kind)
	}
}
