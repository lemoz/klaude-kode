package toolruntime

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type Runtime interface {
	ListTools(ctx context.Context, s contracts.SessionContext) ([]contracts.ToolDescriptor, error)
	ExecuteTool(ctx context.Context, s contracts.SessionContext, call contracts.ToolCall) (<-chan contracts.ToolEvent, error)
	ResolveResources(ctx context.Context, s contracts.SessionContext) (contracts.ResourceSnapshot, error)
}

type BuiltinRuntime struct{}

func NewBuiltinRuntime() *BuiltinRuntime {
	return &BuiltinRuntime{}
}

func (r *BuiltinRuntime) ListTools(_ context.Context, _ contracts.SessionContext) ([]contracts.ToolDescriptor, error) {
	return []contracts.ToolDescriptor{
		{
			Name:             "echo",
			Description:      "Echo text back into the turn loop",
			ConcurrencyClass: "read",
		},
		{
			Name:               "pwd",
			Description:        "Return the session working directory",
			ConcurrencyClass:   "read",
			RequiresPermission: true,
			PermissionScope:    "workspace",
		},
	}, nil
}

func (r *BuiltinRuntime) ExecuteTool(_ context.Context, s contracts.SessionContext, call contracts.ToolCall) (<-chan contracts.ToolEvent, error) {
	ch := make(chan contracts.ToolEvent, 4)

	switch call.Name {
	case "echo":
		text, _ := call.Input["text"].(string)
		ch <- contracts.ToolEvent{
			Kind:    contracts.ToolEventKindProgress,
			Message: "echoing input",
		}
		ch <- contracts.ToolEvent{
			Kind:          contracts.ToolEventKindCompleted,
			ResultSummary: "echo completed",
			Output:        text,
		}
	case "pwd":
		ch <- contracts.ToolEvent{
			Kind:    contracts.ToolEventKindProgress,
			Message: "reading working directory",
		}
		ch <- contracts.ToolEvent{
			Kind:          contracts.ToolEventKindCompleted,
			ResultSummary: "pwd completed",
			Output:        s.CWD,
		}
	default:
		close(ch)
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}

	close(ch)
	return ch, nil
}

func (r *BuiltinRuntime) ResolveResources(_ context.Context, s contracts.SessionContext) (contracts.ResourceSnapshot, error) {
	return contracts.ResourceSnapshot{
		Resources: []string{s.CWD},
	}, nil
}

func ParseInlineToolCall(text string) (contracts.ToolCall, bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "tool:") {
		return contracts.ToolCall{}, false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "tool:"))
	if rest == "" {
		return contracts.ToolCall{}, false
	}

	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return contracts.ToolCall{}, false
	}

	name := parts[0]
	args := parts[1:]
	call := contracts.ToolCall{
		Name:  name,
		Input: map[string]any{},
	}

	switch name {
	case "echo":
		call.Input["text"] = strings.Join(args, " ")
	case "pwd":
	default:
		call.Input["args"] = append([]string(nil), args...)
	}

	return call, true
}
