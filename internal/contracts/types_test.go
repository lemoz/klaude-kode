package contracts

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionEventMarshalsHookAndPluginPayloads(t *testing.T) {
	event := SessionEvent{
		SchemaVersion: SchemaVersionV1,
		SessionID:     "session-test",
		Sequence:      1,
		Timestamp:     time.Unix(1, 0).UTC(),
		Kind:          EventKindHookLifecycle,
		Payload: SessionEventPayload{
			Hook: &HookEventPayload{
				HookID:    "hook-1",
				EventName: "session_start",
				Command:   "echo start",
				State:     "completed",
				ExitCode:  0,
				Message:   "hook completed",
			},
			Plugin: &PluginStatusPayload{
				PluginID:   "example-plugin",
				Name:       "Example Plugin",
				Version:    "1.2.0",
				Loaded:     true,
				Valid:      true,
				Commands:   []string{"review"},
				Agents:     []string{"frontend"},
				Skills:     []string{"deploy"},
				HookCount:  2,
				MCPServers: 1,
			},
		},
	}

	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded SessionEvent
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if decoded.Payload.Hook == nil || decoded.Payload.Hook.HookID != "hook-1" {
		t.Fatalf("expected hook payload to survive round-trip, got %#v", decoded.Payload.Hook)
	}

	if decoded.Payload.Plugin == nil || decoded.Payload.Plugin.PluginID != "example-plugin" {
		t.Fatalf("expected plugin payload to survive round-trip, got %#v", decoded.Payload.Plugin)
	}

	if decoded.Payload.Plugin.Version != "1.2.0" || len(decoded.Payload.Plugin.Skills) != 1 || decoded.Payload.Plugin.Skills[0] != "deploy" {
		t.Fatalf("expected plugin metadata to survive round-trip, got %#v", decoded.Payload.Plugin)
	}
}
