package hooks

import (
	"context"
	"strings"
	"testing"
)

func TestLocalRunnerRunPassesJSONPayloadToHook(t *testing.T) {
	runner := NewLocalRunner()
	definition := Definition{
		ID:      "cat-payload",
		Event:   EventSessionStart,
		Command: "cat",
		Enabled: true,
	}

	result, err := runner.Run(context.Background(), definition, SessionStartPayload{
		BasePayload: BasePayload{
			Event:     EventSessionStart,
			SessionID: "sess-1",
		},
		ProfileID: "anthropic-main",
		Model:     "claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.State != "completed" {
		t.Fatalf("expected completed result, got %#v", result)
	}
	if !strings.Contains(result.Output, `"session_id":"sess-1"`) {
		t.Fatalf("expected JSON payload in hook output, got %q", result.Output)
	}
}

func TestLocalRunnerRunCapturesFailureExitCode(t *testing.T) {
	runner := NewLocalRunner()
	definition := Definition{
		ID:      "fail-hook",
		Event:   EventSessionEnd,
		Command: "exit 7",
		Enabled: true,
	}

	result, err := runner.Run(context.Background(), definition, SessionEndPayload{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.State != "failed" {
		t.Fatalf("expected failed result, got %#v", result)
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %#v", result)
	}
}

func TestLocalRunnerRunHonorsTimeout(t *testing.T) {
	runner := NewLocalRunner()
	definition := Definition{
		ID:             "slow-hook",
		Event:          EventSessionStart,
		Command:        "sleep 2",
		TimeoutSeconds: 1,
		Enabled:        true,
	}

	result, err := runner.Run(context.Background(), definition, SessionStartPayload{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.State != "timed_out" {
		t.Fatalf("expected timed_out result, got %#v", result)
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected timeout exit code -1, got %#v", result)
	}
}
