package contracts

import "time"

const SchemaVersionV1 = "v1"

type SessionMode string

const (
	SessionModeInteractive SessionMode = "interactive"
	SessionModeHeadless    SessionMode = "headless"
	SessionModeRemote      SessionMode = "remote"
	SessionModeViewer      SessionMode = "viewer"
)

type SessionStatus string

const (
	SessionStatusActive SessionStatus = "active"
	SessionStatusClosed SessionStatus = "closed"
)

type ProviderKind string

const (
	ProviderAnthropic  ProviderKind = "anthropic"
	ProviderOpenRouter ProviderKind = "openrouter"
)

type AuthProfileKind string

const (
	AuthProfileAnthropicOAuth   AuthProfileKind = "anthropic_oauth"
	AuthProfileAnthropicAPIKey  AuthProfileKind = "anthropic_api_key"
	AuthProfileOpenRouterAPIKey AuthProfileKind = "openrouter_api_key"
)

type CommandKind string

const (
	CommandKindUserInput            CommandKind = "user_input"
	CommandKindCancelTurn           CommandKind = "cancel_turn"
	CommandKindApprovePermission    CommandKind = "approve_permission"
	CommandKindDenyPermission       CommandKind = "deny_permission"
	CommandKindUpdateSessionSetting CommandKind = "update_session_setting"
	CommandKindExecuteSlashCommand  CommandKind = "execute_slash_command"
	CommandKindAttachViewer         CommandKind = "attach_viewer"
	CommandKindDetachViewer         CommandKind = "detach_viewer"
	CommandKindRefreshMCP           CommandKind = "refresh_mcp"
	CommandKindCloseSession         CommandKind = "close_session"
)

type EventKind string

const (
	EventKindSessionStarted      EventKind = "session_started"
	EventKindLifecycle           EventKind = "lifecycle"
	EventKindUserMessageAccepted EventKind = "user_message_accepted"
	EventKindAssistantMessage    EventKind = "assistant_message"
	EventKindToolCallRequested   EventKind = "tool_call_requested"
	EventKindToolCallProgress    EventKind = "tool_call_progress"
	EventKindToolCallCompleted   EventKind = "tool_call_completed"
	EventKindPermissionRequested EventKind = "permission_requested"
	EventKindPermissionResolved  EventKind = "permission_resolved"
	EventKindSessionState        EventKind = "session_state"
	EventKindWarning             EventKind = "warning"
	EventKindFailure             EventKind = "failure"
	EventKindSessionClosed       EventKind = "session_closed"
)

type MessageSource string

const (
	MessageSourceInteractive MessageSource = "interactive"
	MessageSourcePrint       MessageSource = "print"
	MessageSourceRemote      MessageSource = "remote"
	MessageSourceReplay      MessageSource = "replay"
)

type TerminalOutcome string

const (
	TerminalOutcomeNone                TerminalOutcome = ""
	TerminalOutcomeSuccess             TerminalOutcome = "success"
	TerminalOutcomeTaskFailure         TerminalOutcome = "task_failure"
	TerminalOutcomeToolFailure         TerminalOutcome = "tool_failure"
	TerminalOutcomeProviderFailure     TerminalOutcome = "provider_failure"
	TerminalOutcomeBudgetExhausted     TerminalOutcome = "budget_exhausted"
	TerminalOutcomeValidationFailure   TerminalOutcome = "validation_failure"
	TerminalOutcomeEnvironmentMismatch TerminalOutcome = "environment_mismatch"
)

type FailureCategory string

const (
	FailureCategoryProvider   FailureCategory = "provider"
	FailureCategoryTool       FailureCategory = "tool"
	FailureCategoryPermission FailureCategory = "permission"
	FailureCategoryTransport  FailureCategory = "transport"
	FailureCategoryAuth       FailureCategory = "auth"
	FailureCategoryMCP        FailureCategory = "mcp"
	FailureCategoryReplay     FailureCategory = "replay"
)

type StartSessionRequest struct {
	SessionID string      `json:"session_id"`
	CWD       string      `json:"cwd"`
	Mode      SessionMode `json:"mode"`
	ProfileID string      `json:"profile_id"`
	Model     string      `json:"model"`
	Title     string      `json:"title"`
}

type ResumeSessionRequest struct {
	SessionID string `json:"session_id"`
}

type SessionHandle struct {
	SessionID string      `json:"session_id"`
	CWD       string      `json:"cwd"`
	Mode      SessionMode `json:"mode"`
	ProfileID string      `json:"profile_id"`
	Model     string      `json:"model"`
	CreatedAt time.Time   `json:"created_at"`
}

type SessionSummary struct {
	SessionID       string          `json:"session_id"`
	CWD             string          `json:"cwd"`
	Mode            SessionMode     `json:"mode"`
	Status          SessionStatus   `json:"status"`
	ProfileID       string          `json:"profile_id"`
	Model           string          `json:"model"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	EventCount      int             `json:"event_count"`
	TurnCount       int             `json:"turn_count"`
	LastSequence    int64           `json:"last_sequence"`
	LastEventKind   EventKind       `json:"last_event_kind"`
	ClosedReason    string          `json:"closed_reason"`
	TerminalOutcome TerminalOutcome `json:"terminal_outcome"`
}

type SessionStateSnapshot struct {
	CWD             string          `json:"cwd"`
	Mode            SessionMode     `json:"mode"`
	Status          SessionStatus   `json:"status"`
	ProfileID       string          `json:"profile_id"`
	Model           string          `json:"model"`
	EventCount      int             `json:"event_count"`
	TurnCount       int             `json:"turn_count"`
	LastSequence    int64           `json:"last_sequence"`
	ClosedReason    string          `json:"closed_reason"`
	TerminalOutcome TerminalOutcome `json:"terminal_outcome"`
}

type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

type SessionCommand struct {
	SchemaVersion string                `json:"schema_version"`
	CommandID     string                `json:"command_id"`
	Kind          CommandKind           `json:"kind"`
	Timestamp     time.Time             `json:"timestamp"`
	Payload       SessionCommandPayload `json:"payload"`
}

type SessionCommandPayload struct {
	TurnID       string            `json:"turn_id"`
	RequestID    string            `json:"request_id"`
	Source       MessageSource     `json:"source"`
	Text         string            `json:"text"`
	Name         string            `json:"name"`
	Args         []string          `json:"args"`
	SettingKey   string            `json:"setting_key"`
	SettingValue string            `json:"setting_value"`
	Reason       string            `json:"reason"`
	Metadata     map[string]string `json:"metadata"`
}

type SessionEvent struct {
	SchemaVersion string              `json:"schema_version"`
	SessionID     string              `json:"session_id"`
	Sequence      int64               `json:"sequence"`
	Timestamp     time.Time           `json:"timestamp"`
	Kind          EventKind           `json:"kind"`
	Payload       SessionEventPayload `json:"payload"`
}

type SessionEventPayload struct {
	CommandID       string                  `json:"command_id"`
	TurnID          string                  `json:"turn_id"`
	Source          MessageSource           `json:"source"`
	Message         *CanonicalMessage       `json:"message"`
	State           *SessionStateSnapshot   `json:"state"`
	Tool            *ToolEventPayload       `json:"tool"`
	Permission      *PermissionEventPayload `json:"permission"`
	LifecycleName   string                  `json:"lifecycle_name"`
	TerminalOutcome TerminalOutcome         `json:"terminal_outcome"`
	Warning         string                  `json:"warning"`
	Failure         *FailurePayload         `json:"failure"`
	Reason          string                  `json:"reason"`
}

type ToolEventPayload struct {
	CallID           string         `json:"call_id"`
	Name             string         `json:"name"`
	Input            map[string]any `json:"input"`
	ConcurrencyClass string         `json:"concurrency_class"`
	ProgressMessage  string         `json:"progress_message"`
	ResultSummary    string         `json:"result_summary"`
	Output           string         `json:"output"`
	Failed           bool           `json:"failed"`
}

type PermissionEventPayload struct {
	RequestID    string `json:"request_id"`
	ToolCallID   string `json:"tool_call_id"`
	PolicySource string `json:"policy_source"`
	Prompt       string `json:"prompt"`
	Scope        string `json:"scope"`
	Resolution   string `json:"resolution"`
	Actor        string `json:"actor"`
}

type FailurePayload struct {
	Category  FailureCategory `json:"category"`
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Retryable bool            `json:"retryable"`
}

type CanonicalMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	TurnID       string             `json:"turn_id"`
	Model        string             `json:"model"`
	Messages     []CanonicalMessage `json:"messages"`
	SystemPrompt []string           `json:"system_prompt"`
	ToolChoice   ToolChoice         `json:"tool_choice"`
}

type CompletionResult struct {
	Message CanonicalMessage `json:"message"`
}

type TokenCountRequest struct {
	Model    string             `json:"model"`
	Messages []CanonicalMessage `json:"messages"`
}

type TokenCountResult struct {
	InputTokens int `json:"input_tokens"`
}

type ProviderEvent struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

type CapabilitySet struct {
	Streaming          bool
	ToolCalling        bool
	StructuredOutputs  bool
	TokenCounting      bool
	PromptCaching      bool
	ReasoningControls  bool
	DeferredToolSearch bool
	ImageInput         bool
	DocumentInput      bool
}

type AuthProfile struct {
	ID           string            `json:"id"`
	Kind         AuthProfileKind   `json:"kind"`
	Provider     ProviderKind      `json:"provider"`
	DisplayName  string            `json:"display_name"`
	DefaultModel string            `json:"default_model"`
	Settings     map[string]string `json:"settings"`
}

type ProfileValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

type ProfileAuthState string

const (
	ProfileAuthStateConfigured     ProfileAuthState = "configured"
	ProfileAuthStateAuthenticated  ProfileAuthState = "authenticated"
	ProfileAuthStateExpiring       ProfileAuthState = "expiring"
	ProfileAuthStateExpired        ProfileAuthState = "expired"
	ProfileAuthStateLoggedOut      ProfileAuthState = "logged_out"
)

type ProfileAuthStatus struct {
	State      ProfileAuthState `json:"state"`
	ExpiresAt  string           `json:"expires_at"`
	CanRefresh bool             `json:"can_refresh"`
}

type ProfileStatus struct {
	Profile    AuthProfile             `json:"profile"`
	Validation ProfileValidationResult `json:"validation"`
	Models     []string                `json:"models"`
	Auth       ProfileAuthStatus       `json:"auth"`
}

type ToolDescriptor struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	ConcurrencyClass   string `json:"concurrency_class"`
	RequiresPermission bool   `json:"requires_permission"`
	PermissionScope    string `json:"permission_scope"`
}

type ToolEventKind string

const (
	ToolEventKindProgress  ToolEventKind = "progress"
	ToolEventKindCompleted ToolEventKind = "completed"
)

type ToolCall struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type ToolEvent struct {
	Kind          ToolEventKind `json:"kind"`
	Message       string        `json:"message"`
	ResultSummary string        `json:"result_summary"`
	Output        string        `json:"output"`
	Failed        bool          `json:"failed"`
}

type ResourceSnapshot struct {
	Resources []string `json:"resources"`
}

type SessionContext struct {
	SessionID string      `json:"session_id"`
	CWD       string      `json:"cwd"`
	Mode      SessionMode `json:"mode"`
	ProfileID string      `json:"profile_id"`
	Model     string      `json:"model"`
}

type TransportTarget struct {
	Kind string `json:"kind"`
	Addr string `json:"addr"`
}
