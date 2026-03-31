package contracts

import "time"

type SessionMode string

const (
	SessionModeInteractive SessionMode = "interactive"
	SessionModeHeadless    SessionMode = "headless"
	SessionModeRemote      SessionMode = "remote"
	SessionModeViewer      SessionMode = "viewer"
)

type ProviderKind string

const (
	ProviderAnthropic ProviderKind = "anthropic"
	ProviderOpenRouter ProviderKind = "openrouter"
)

type AuthProfileKind string

const (
	AuthProfileAnthropicOAuth AuthProfileKind = "anthropic_oauth"
	AuthProfileAnthropicAPIKey AuthProfileKind = "anthropic_api_key"
	AuthProfileOpenRouterAPIKey AuthProfileKind = "openrouter_api_key"
)

type StartSessionRequest struct {
	SessionID string
	CWD       string
	Mode      SessionMode
	ProfileID string
	Model     string
}

type ResumeSessionRequest struct {
	SessionID string
}

type SessionHandle struct {
	SessionID string
	CWD       string
	Mode      SessionMode
}

type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

type SessionCommand struct {
	CommandID string
	Kind      string
	Payload   map[string]any
}

type SessionEvent struct {
	SchemaVersion string
	SessionID     string
	Sequence      int64
	Timestamp     time.Time
	Kind          string
	Payload       map[string]any
}

type CanonicalMessage struct {
	Role    string
	Content string
}

type CompletionRequest struct {
	TurnID       string
	Model        string
	Messages     []CanonicalMessage
	SystemPrompt []string
	ToolChoice   ToolChoice
}

type CompletionResult struct {
	Message CanonicalMessage
}

type TokenCountRequest struct {
	Model    string
	Messages []CanonicalMessage
}

type TokenCountResult struct {
	InputTokens int
}

type ProviderEvent struct {
	Kind    string
	Payload map[string]any
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
	ID           string
	Kind         AuthProfileKind
	Provider     ProviderKind
	DisplayName  string
	DefaultModel string
	Settings     map[string]string
}

type ProfileValidationResult struct {
	Valid   bool
	Message string
}

type ToolDescriptor struct {
	Name             string
	Description      string
	ConcurrencyClass string
}

type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

type ToolEvent struct {
	Kind    string
	Payload map[string]any
}

type ResourceSnapshot struct {
	Resources []string
}

type SessionContext struct {
	SessionID string
	CWD       string
	Mode      SessionMode
}

type TransportTarget struct {
	Kind string
	Addr string
}

