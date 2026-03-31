package toolruntime

import (
	"context"
	"testing"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func TestBuiltinRuntimeListsEchoAndPwd(t *testing.T) {
	runtime := NewBuiltinRuntime()

	tools, err := runtime.ListTools(context.Background(), contracts.SessionContext{})
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[1].Name != "pwd" || !tools[1].RequiresPermission {
		t.Fatalf("expected pwd to require permission, got %#v", tools[1])
	}
}

func TestBuiltinRuntimeExecutesPwd(t *testing.T) {
	runtime := NewBuiltinRuntime()

	stream, err := runtime.ExecuteTool(context.Background(), contracts.SessionContext{
		CWD: "/tmp/project",
	}, contracts.ToolCall{
		ID:    "tool_1",
		Name:  "pwd",
		Input: map[string]any{},
	})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}

	events := collectToolEvents(stream)
	if len(events) != 2 {
		t.Fatalf("expected 2 tool events, got %d", len(events))
	}
	if events[0].Kind != contracts.ToolEventKindProgress {
		t.Fatalf("expected progress first, got %s", events[0].Kind)
	}
	if events[1].Output != "/tmp/project" {
		t.Fatalf("expected pwd output /tmp/project, got %q", events[1].Output)
	}
}

func TestParseInlineToolCall(t *testing.T) {
	call, ok := ParseInlineToolCall("tool:echo hello world")
	if !ok {
		t.Fatal("expected tool call to parse")
	}
	if call.Name != "echo" {
		t.Fatalf("expected echo tool, got %s", call.Name)
	}
	if got, _ := call.Input["text"].(string); got != "hello world" {
		t.Fatalf("expected echo text payload, got %#v", call.Input)
	}
}

func collectToolEvents(stream <-chan contracts.ToolEvent) []contracts.ToolEvent {
	var events []contracts.ToolEvent
	for event := range stream {
		events = append(events, event)
	}
	return events
}
