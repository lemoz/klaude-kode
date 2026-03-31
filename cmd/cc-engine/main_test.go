package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
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
	if len(got.Events) != 9 {
		t.Fatalf("expected 9 events after headless close, got %d", len(got.Events))
	}
	if got.Events[len(got.Events)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got.Events[len(got.Events)-1].Kind)
	}
	if got.Events[4].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message in event stream, got %s", got.Events[4].Kind)
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
	if len(lines) != 9 {
		t.Fatalf("expected 9 jsonl events, got %d", len(lines))
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
	if len(got.Events) != 9 {
		t.Fatalf("expected 9 replayed events, got %d", len(got.Events))
	}
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
	if len(lines) != 9 {
		t.Fatalf("expected 9 stdio events, got %d", len(lines))
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
	if got[4].Kind != contracts.EventKindAssistantMessage {
		t.Fatalf("expected assistant_message, got %s", got[4].Kind)
	}
	if got[len(got)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got[len(got)-1].Kind)
	}
	if got[len(got)-1].Payload.Reason != "stdio_test_complete" {
		t.Fatalf("expected stdio close reason, got %q", got[len(got)-1].Payload.Reason)
	}
}
