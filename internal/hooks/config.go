package hooks

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type EventName string

const (
	EventSessionStart     EventName = "session_start"
	EventSessionEnd       EventName = "session_end"
	EventPreToolUse       EventName = "pre_tool_use"
	EventPostToolUse      EventName = "post_tool_use"
	EventPermissionDenied EventName = "permission_denied"
)

var hookIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-_]*$`)

type Config struct {
	Hooks []Definition `json:"hooks"`
}

type Definition struct {
	ID             string    `json:"id"`
	Event          EventName `json:"event"`
	Command        string    `json:"command"`
	Shell          string    `json:"shell,omitempty"`
	TimeoutSeconds int       `json:"timeout_seconds,omitempty"`
	Enabled        bool      `json:"enabled"`
}

type ValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type BasePayload struct {
	Event     EventName             `json:"event"`
	SessionID string                `json:"session_id"`
	CWD       string                `json:"cwd"`
	Mode      contracts.SessionMode `json:"mode"`
	Timestamp time.Time             `json:"timestamp"`
}

type SessionStartPayload struct {
	BasePayload
	ProfileID string `json:"profile_id"`
	Model     string `json:"model"`
}

type SessionEndPayload struct {
	BasePayload
	Reason          string                    `json:"reason"`
	TerminalOutcome contracts.TerminalOutcome `json:"terminal_outcome"`
}

type ToolPayload struct {
	BasePayload
	TurnID     string `json:"turn_id"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
}

type PermissionDeniedPayload struct {
	BasePayload
	RequestID  string `json:"request_id"`
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name"`
	Reason     string `json:"reason"`
}

func ValidateConfig(config Config) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	seenIDs := make(map[string]struct{}, len(config.Hooks))

	for index, hook := range config.Hooks {
		prefix := fmt.Sprintf("hooks[%d]", index)

		id := strings.TrimSpace(hook.ID)
		if id == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".id",
				Message: "id is required",
			})
		} else {
			if !hookIDPattern.MatchString(id) {
				issues = append(issues, ValidationIssue{
					Field:   prefix + ".id",
					Message: "id must be lowercase letters, numbers, hyphens, or underscores",
				})
			}
			if _, exists := seenIDs[id]; exists {
				issues = append(issues, ValidationIssue{
					Field:   prefix + ".id",
					Message: "id must be unique",
				})
			} else {
				seenIDs[id] = struct{}{}
			}
		}

		if !isValidEvent(hook.Event) {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".event",
				Message: "event is required and must be a supported hook lifecycle event",
			})
		}

		if strings.TrimSpace(hook.Command) == "" {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".command",
				Message: "command is required",
			})
		}

		if hook.TimeoutSeconds < 0 {
			issues = append(issues, ValidationIssue{
				Field:   prefix + ".timeout_seconds",
				Message: "timeout must be zero or greater",
			})
		}
	}

	return issues
}

func isValidEvent(event EventName) bool {
	switch event {
	case EventSessionStart, EventSessionEnd, EventPreToolUse, EventPostToolUse, EventPermissionDenied:
		return true
	default:
		return false
	}
}
