package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestRunJSONFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-format=json", "-prompt=hello world", "-session-id=test-json"}, &stdout, &stderr)
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
	if len(got.Events) != 6 {
		t.Fatalf("expected 6 events after headless close, got %d", len(got.Events))
	}
	if got.Events[len(got.Events)-1].Kind != contracts.EventKindSessionClosed {
		t.Fatalf("expected final event session_closed, got %s", got.Events[len(got.Events)-1].Kind)
	}
}

func TestRunTextFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"-format=text", "-prompt=text run", "-session-id=test-text"}, &stdout, &stderr)
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

	err := run([]string{"-format=events", "-prompt=event run", "-session-id=test-events"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 jsonl events, got %d", len(lines))
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
