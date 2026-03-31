package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/harness"
)

func TestRunJSONFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-format=json",
		"-prompt=hello over cc",
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

	if got.Launcher != "cc" {
		t.Fatalf("expected launcher cc, got %q", got.Launcher)
	}
	if got.Transport != "local" {
		t.Fatalf("expected local transport, got %q", got.Transport)
	}
	if got.Session.Mode != contracts.SessionModeInteractive {
		t.Fatalf("expected interactive session mode, got %s", got.Session.Mode)
	}
	if got.Summary.Status != contracts.SessionStatusClosed {
		t.Fatalf("expected closed status, got %s", got.Summary.Status)
	}
	if len(got.Events) != 10 {
		t.Fatalf("expected 10 events after local close, got %d", len(got.Events))
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
	if !strings.Contains(output, "cc local session") {
		t.Fatalf("expected text header, got %q", output)
	}
	if !strings.Contains(output, "mode: interactive") {
		t.Fatalf("expected interactive mode in text output, got %q", output)
	}
	if !strings.Contains(output, "closed_reason: cc_complete") {
		t.Fatalf("expected local close reason, got %q", output)
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

func TestRunProfilesText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-profiles",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc configured profiles") {
		t.Fatalf("expected profile header, got %q", output)
	}
	if !strings.Contains(output, "anthropic-main") {
		t.Fatalf("expected anthropic-main in profile output, got %q", output)
	}
	if !strings.Contains(output, "openrouter-main") {
		t.Fatalf("expected openrouter-main in profile output, got %q", output)
	}
	if !strings.Contains(output, "capabilities: streaming") {
		t.Fatalf("expected capabilities in profile output, got %q", output)
	}
}

func TestRunModelsText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := filepath.Join(t.TempDir(), "state-root")

	err := run([]string{
		"-models",
		"-profile-id=openrouter-main",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc model catalog") {
		t.Fatalf("expected model catalog header, got %q", output)
	}
	if !strings.Contains(output, "profile: openrouter-main") {
		t.Fatalf("expected openrouter-main in model catalog, got %q", output)
	}
	if !strings.Contains(output, "models: openrouter/auto") {
		t.Fatalf("expected model list in catalog, got %q", output)
	}
}

func TestRunStatusText(t *testing.T) {
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
		"-status",
		"-resume-session=status-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("status run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc session status") {
		t.Fatalf("expected status header, got %q", output)
	}
	if !strings.Contains(output, "session: status-target") {
		t.Fatalf("expected session id in status output, got %q", output)
	}
	if !strings.Contains(output, "status: closed") {
		t.Fatalf("expected closed status in output, got %q", output)
	}
}

func TestRunExportReplayPackText(t *testing.T) {
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
		"-export-replay-pack",
		"-resume-session=replay-target",
		"-state-root=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("export replay run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc replay pack") {
		t.Fatalf("expected replay pack header, got %q", output)
	}
	if !strings.Contains(output, "session: replay-target") {
		t.Fatalf("expected session id in replay output, got %q", output)
	}
	if !strings.Contains(output, "events: 10") {
		t.Fatalf("expected event count in replay output, got %q", output)
	}
}

func TestRunValidateCandidateText(t *testing.T) {
	root := createValidCandidateRoot(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"-validate-candidate",
		"-cwd=" + root,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc candidate validation") {
		t.Fatalf("expected validation header, got %q", output)
	}
	if !strings.Contains(output, "valid: true") {
		t.Fatalf("expected valid candidate output, got %q", output)
	}
}

func TestRunReplayEvalText(t *testing.T) {
	candidateRoot := createValidCandidateRoot(t)
	replayPath := writeReplayPack(t, t.TempDir(), contracts.TerminalOutcomeSuccess)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run([]string{
		"-run-replay-eval",
		"-cwd=" + candidateRoot,
		"-replay-path=" + replayPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cc replay eval") {
		t.Fatalf("expected replay eval header, got %q", output)
	}
	if !strings.Contains(output, "status: completed") {
		t.Fatalf("expected completed status in replay eval output, got %q", output)
	}
	if !strings.Contains(output, "score: 1.00") {
		t.Fatalf("expected score in replay eval output, got %q", output)
	}

	artifactRoot := filepath.Join(candidateRoot, harness.DefaultArtifactDirName)
	runIDLine := strings.Split(strings.TrimSpace(output), "\n")[1]
	runID := strings.TrimPrefix(runIDLine, "run: ")
	if _, err := os.Stat(harness.EvalRunPath(artifactRoot, runID)); err != nil {
		t.Fatalf("expected persisted eval run artifact: %v", err)
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
