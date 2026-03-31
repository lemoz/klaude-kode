package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cdossman/klaude-kode/internal/auth/anthropicoauth"
	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
	"github.com/cdossman/klaude-kode/internal/provider"
	"github.com/cdossman/klaude-kode/internal/transport"
)

type outputFormat string

const (
	outputFormatJSON   outputFormat = "json"
	outputFormatText   outputFormat = "text"
	outputFormatEvents outputFormat = "events"
)

type config struct {
	Transport           string
	Format              outputFormat
	ListProfiles        bool
	ListModels          bool
	ShowStatus          bool
	ExportReplayPack    bool
	UpsertProfile       bool
	AnthropicOAuthLogin bool
	LogoutProfileID     string
	Prompt              string
	SessionID           string
	ResumeSessionID     string
	CWD                 string
	ProfileID           string
	Model               string
	ProfileProvider     string
	ProfileKind         string
	DisplayName         string
	DefaultModel        string
	CredentialRef       string
	APIBase             string
	OAuthHost           string
	AccountScope        string
	OAuthOpenBrowser    bool
	MakeDefault         bool
	StateRoot           string
}

type result struct {
	Engine  string                   `json:"engine"`
	Session contracts.SessionHandle  `json:"session"`
	Summary contracts.SessionSummary `json:"summary"`
	Events  []contracts.SessionEvent `json:"events"`
}

type profileCatalogResult struct {
	Engine   string                    `json:"engine"`
	Profiles []contracts.ProfileStatus `json:"profiles"`
}

type modelCatalogResult struct {
	Engine       string                  `json:"engine"`
	ProfileID    string                  `json:"profile_id"`
	DefaultModel string                  `json:"default_model"`
	Models       []string                `json:"models"`
	Capabilities contracts.CapabilitySet `json:"capabilities"`
}

type sessionStatusResult struct {
	Engine  string                   `json:"engine"`
	Session string                   `json:"session"`
	Summary contracts.SessionSummary `json:"summary"`
}

var performAnthropicOAuthLogin = anthropicoauth.PerformLogin

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	return runWithInput(args, os.Stdin, stdout, stderr)
}

func runWithInput(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	cfg, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}

	runtime, err := engine.NewFileBackedEngine(cfg.StateRoot)
	if err != nil {
		return err
	}

	ctx := context.Background()

	if cfg.ListProfiles {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-profiles does not support -format=events")
		}
		return renderProfileCatalog(ctx, runtime, cfg.Format, stdout)
	}
	if cfg.ListModels {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-models does not support -format=events")
		}
		return renderModelCatalog(ctx, runtime, cfg, stdout)
	}
	if cfg.ShowStatus {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-status does not support -format=events")
		}
		return renderSessionStatus(ctx, runtime, cfg, stdout)
	}
	if cfg.ExportReplayPack {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-export-replay-pack does not support -format=events")
		}
		return renderReplayPack(ctx, runtime, cfg, stdout)
	}
	if cfg.UpsertProfile {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-upsert-profile does not support -format=events")
		}
		return upsertProfileAndRenderCatalog(ctx, runtime, cfg, stdout)
	}
	if cfg.AnthropicOAuthLogin {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-anthropic-oauth-login does not support -format=events")
		}
		return loginAnthropicOAuthAndRenderCatalog(ctx, runtime, cfg, stdout, stderr)
	}
	if cfg.LogoutProfileID != "" {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-logout-profile does not support -format=events")
		}
		return logoutProfileAndRenderCatalog(ctx, runtime, cfg, stdout)
	}

	if cfg.Transport == "stdio" {
		if cfg.Format != outputFormatEvents {
			return fmt.Errorf("stdio transport requires -format=events")
		}
		return runStdioSession(ctx, runtime, cfg, stdin, stdout)
	}

	if cfg.ResumeSessionID != "" {
		return renderPersistedSession(ctx, runtime, cfg, stdout)
	}
	if cfg.Format == outputFormatEvents {
		return streamEvents(ctx, runtime, cfg, stdout)
	}

	sessionResult, err := executeHeadlessSession(ctx, runtime, cfg)
	if err != nil {
		return err
	}

	return renderResult(stdout, cfg.Format, sessionResult)
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("cc-engine", flag.ContinueOnError)
	fs.SetOutput(stderr)

	transportValue := fs.String("transport", "headless", "engine transport: headless, stdio")
	formatValue := fs.String("format", string(outputFormatJSON), "output format: json, text, events")
	listProfilesValue := fs.Bool("profiles", false, "list configured auth profiles and exit")
	listModelsValue := fs.Bool("models", false, "list available models for the selected or default profile and exit")
	showStatusValue := fs.Bool("status", false, "show summary for an existing session and exit")
	exportReplayPackValue := fs.Bool("export-replay-pack", false, "export a replay pack for an existing session and exit")
	upsertProfileValue := fs.Bool("upsert-profile", false, "create or update a stored auth profile and exit")
	anthropicOAuthLoginValue := fs.Bool("anthropic-oauth-login", false, "log in with Anthropic OAuth and save the resulting profile")
	logoutProfileValue := fs.String("logout-profile", "", "clear stored auth from the specified profile and exit")
	promptValue := fs.String("prompt", "bootstrap hello from cc-engine", "prompt to submit to the session")
	sessionIDValue := fs.String("session-id", "engine-bootstrap", "session identifier")
	resumeSessionValue := fs.String("resume-session", "", "load and render a persisted session")
	cwdValue := fs.String("cwd", mustGetwd(), "session working directory")
	profileIDValue := fs.String("profile-id", "", "active auth profile id")
	modelValue := fs.String("model", "", "active model id")
	profileProviderValue := fs.String("provider", "", "profile provider kind for -upsert-profile")
	profileKindValue := fs.String("profile-kind", "", "profile auth kind for -upsert-profile")
	displayNameValue := fs.String("display-name", "", "display name for -upsert-profile")
	defaultModelValue := fs.String("default-model", "", "default model for -upsert-profile")
	credentialRefValue := fs.String("credential-ref", "", "credential reference for -upsert-profile")
	apiBaseValue := fs.String("api-base", "", "provider API base for -upsert-profile")
	oauthHostValue := fs.String("oauth-host", "", "oauth host for anthropic_oauth profiles")
	accountScopeValue := fs.String("account-scope", "", "account scope for anthropic_oauth profiles")
	oauthOpenBrowserValue := fs.Bool("oauth-open-browser", true, "open the Anthropic OAuth URL in a browser")
	makeDefaultValue := fs.Bool("make-default", false, "set the upserted profile as the default profile")
	stateRootValue := fs.String("state-root", engine.DefaultStateRoot(), "engine state root")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	format := outputFormat(strings.ToLower(strings.TrimSpace(*formatValue)))
	switch format {
	case outputFormatJSON, outputFormatText, outputFormatEvents:
	default:
		return config{}, fmt.Errorf("unsupported format %q", *formatValue)
	}

	transportMode := strings.ToLower(strings.TrimSpace(*transportValue))
	switch transportMode {
	case "headless", "stdio":
	default:
		return config{}, fmt.Errorf("unsupported transport %q", *transportValue)
	}

	return config{
		Transport:           transportMode,
		Format:              format,
		ListProfiles:        *listProfilesValue,
		ListModels:          *listModelsValue,
		ShowStatus:          *showStatusValue,
		ExportReplayPack:    *exportReplayPackValue,
		UpsertProfile:       *upsertProfileValue,
		AnthropicOAuthLogin: *anthropicOAuthLoginValue,
		LogoutProfileID:     strings.TrimSpace(*logoutProfileValue),
		Prompt:              strings.TrimSpace(*promptValue),
		SessionID:           strings.TrimSpace(*sessionIDValue),
		ResumeSessionID:     strings.TrimSpace(*resumeSessionValue),
		CWD:                 strings.TrimSpace(*cwdValue),
		ProfileID:           strings.TrimSpace(*profileIDValue),
		Model:               strings.TrimSpace(*modelValue),
		ProfileProvider:     strings.TrimSpace(*profileProviderValue),
		ProfileKind:         strings.TrimSpace(*profileKindValue),
		DisplayName:         strings.TrimSpace(*displayNameValue),
		DefaultModel:        strings.TrimSpace(*defaultModelValue),
		CredentialRef:       strings.TrimSpace(*credentialRefValue),
		APIBase:             strings.TrimSpace(*apiBaseValue),
		OAuthHost:           strings.TrimSpace(*oauthHostValue),
		AccountScope:        strings.TrimSpace(*accountScopeValue),
		OAuthOpenBrowser:    *oauthOpenBrowserValue,
		MakeDefault:         *makeDefaultValue,
		StateRoot:           strings.TrimSpace(*stateRootValue),
	}, nil
}

func runStdioSession(ctx context.Context, runtime engine.Engine, cfg config, stdin io.Reader, stdout io.Writer) error {
	handle, err := startStdioSession(ctx, runtime, cfg)
	if err != nil {
		return err
	}

	localTransport := transport.NewLocalTransport(runtime)
	if err := localTransport.Open(ctx, contracts.TransportTarget{
		Kind: "local",
		Addr: handle.SessionID,
	}); err != nil {
		return err
	}
	defer localTransport.Close(context.Background())

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := localTransport.Events(streamCtx)
	if err != nil {
		return err
	}

	encoded := make(chan error, 1)
	go func() {
		encoder := json.NewEncoder(stdout)
		for event := range stream {
			if err := encoder.Encode(event); err != nil {
				encoded <- err
				return
			}
		}
		encoded <- nil
	}()

	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var cmd contracts.SessionCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			cancel()
			<-encoded
			return fmt.Errorf("decode session command: %w", err)
		}
		if err := localTransport.Send(ctx, cmd); err != nil {
			cancel()
			<-encoded
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		cancel()
		<-encoded
		return err
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err == nil && summary.Status != contracts.SessionStatusClosed {
		if err := localTransport.Send(ctx, contracts.SessionCommand{
			Kind: contracts.CommandKindCloseSession,
			Payload: contracts.SessionCommandPayload{
				Reason: "stdio_eof",
			},
		}); err != nil {
			cancel()
			<-encoded
			return err
		}
	}

	if err := <-encoded; err != nil {
		return err
	}
	return nil
}

func executeHeadlessSession(ctx context.Context, runtime engine.Engine, cfg config) (result, error) {
	handle, err := startHeadlessSession(ctx, runtime, cfg)
	if err != nil {
		return result{}, err
	}

	if err := sendPrompt(ctx, runtime, handle.SessionID, cfg.Prompt); err != nil {
		return result{}, err
	}
	if err := runtime.CloseSession(ctx, handle.SessionID, "headless_complete"); err != nil {
		return result{}, err
	}

	return loadSessionResult(ctx, runtime, handle.SessionID)
}

func renderPersistedSession(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	if cfg.Format == outputFormatEvents {
		stream, err := runtime.StreamEvents(ctx, cfg.ResumeSessionID)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		for event := range stream {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		return nil
	}

	sessionResult, err := loadSessionResult(ctx, runtime, cfg.ResumeSessionID)
	if err != nil {
		return err
	}
	return renderResult(stdout, cfg.Format, sessionResult)
}

func streamEvents(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	handle, err := startHeadlessSession(ctx, runtime, cfg)
	if err != nil {
		return err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := runtime.StreamEvents(streamCtx, handle.SessionID)
	if err != nil {
		return err
	}

	if err := sendPrompt(ctx, runtime, handle.SessionID, cfg.Prompt); err != nil {
		return err
	}
	if err := runtime.CloseSession(ctx, handle.SessionID, "headless_complete"); err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	for event := range stream {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func startStdioSession(ctx context.Context, runtime engine.Engine, cfg config) (contracts.SessionHandle, error) {
	if cfg.ResumeSessionID != "" {
		return runtime.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: cfg.ResumeSessionID})
	}

	return runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: cfg.SessionID,
		CWD:       cfg.CWD,
		Mode:      contracts.SessionModeInteractive,
		ProfileID: cfg.ProfileID,
		Model:     cfg.Model,
	})
}

func startHeadlessSession(ctx context.Context, runtime engine.Engine, cfg config) (contracts.SessionHandle, error) {
	return runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: cfg.SessionID,
		CWD:       cfg.CWD,
		Mode:      contracts.SessionModeHeadless,
		ProfileID: cfg.ProfileID,
		Model:     cfg.Model,
	})
}

func sendPrompt(ctx context.Context, runtime engine.Engine, sessionID string, prompt string) error {
	return runtime.SendCommand(ctx, sessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   prompt,
			Source: contracts.MessageSourcePrint,
		},
	})
}

func loadSessionResult(ctx context.Context, runtime engine.Engine, sessionID string) (result, error) {
	handle, err := runtime.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: sessionID})
	if err != nil {
		return result{}, err
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		return result{}, err
	}
	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		return result{}, err
	}

	return result{
		Engine:  "cc-engine",
		Session: handle,
		Summary: summary,
		Events:  events,
	}, nil
}

func renderProfileCatalog(ctx context.Context, runtime engine.Engine, format outputFormat, stdout io.Writer) error {
	profiles, err := runtime.ListProfiles(ctx)
	if err != nil {
		return err
	}

	catalog := profileCatalogResult{
		Engine:   "cc-engine",
		Profiles: profiles,
	}

	switch format {
	case outputFormatText:
		return renderProfileCatalogText(stdout, catalog)
	case outputFormatJSON:
		fallthrough
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	}
}

func renderModelCatalog(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	profiles, err := runtime.ListProfiles(ctx)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return fmt.Errorf("no configured profiles")
	}

	selected, err := selectProfileStatus(profiles, cfg.ProfileID)
	if err != nil {
		return err
	}

	catalog := modelCatalogResult{
		Engine:       "cc-engine",
		ProfileID:    selected.Profile.ID,
		DefaultModel: selected.Profile.DefaultModel,
		Models:       selected.Models,
		Capabilities: selected.Capabilities,
	}

	switch cfg.Format {
	case outputFormatText:
		return renderModelCatalogText(stdout, catalog)
	case outputFormatJSON:
		fallthrough
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	}
}

func renderSessionStatus(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	sessionID := strings.TrimSpace(cfg.ResumeSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(cfg.SessionID)
	}
	if sessionID == "" {
		return fmt.Errorf("a session id is required for -status")
	}

	summary, err := runtime.GetSessionSummary(ctx, sessionID)
	if err != nil {
		return err
	}

	result := sessionStatusResult{
		Engine:  "cc-engine",
		Session: sessionID,
		Summary: summary,
	}

	switch cfg.Format {
	case outputFormatText:
		return renderSessionStatusText(stdout, result)
	case outputFormatJSON:
		fallthrough
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
}

func renderReplayPack(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	sessionID := strings.TrimSpace(cfg.ResumeSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(cfg.SessionID)
	}
	if sessionID == "" {
		return fmt.Errorf("a session id is required for -export-replay-pack")
	}

	pack, err := engine.ExportReplayPack(ctx, runtime, sessionID)
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatText:
		return renderReplayPackText(stdout, pack)
	case outputFormatJSON:
		fallthrough
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(pack)
	}
}

func upsertProfileAndRenderCatalog(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	profile, err := buildProfileFromConfig(cfg)
	if err != nil {
		return err
	}
	if _, err := runtime.SaveProfile(ctx, profile, cfg.MakeDefault); err != nil {
		return err
	}
	return renderProfileCatalog(ctx, runtime, cfg.Format, stdout)
}

func loginAnthropicOAuthAndRenderCatalog(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer, stderr io.Writer) error {
	profileID := strings.TrimSpace(cfg.ProfileID)
	if profileID == "" {
		profileID = "anthropic-main"
	}
	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		displayName = "Anthropic Main"
	}
	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	if defaultModel == "" {
		defaultModel = defaultModelForProvider(contracts.ProviderAnthropic)
	}
	accountScope := strings.TrimSpace(cfg.AccountScope)
	if accountScope == "" {
		accountScope = anthropicoauth.DefaultAccountScope
	}

	result, err := performAnthropicOAuthLogin(ctx, anthropicoauth.LoginOptions{
		ProfileID:    profileID,
		DisplayName:  displayName,
		DefaultModel: defaultModel,
		AccountScope: accountScope,
		OAuthHost:    cfg.OAuthHost,
		APIBase:      cfg.APIBase,
		OpenBrowser:  cfg.OAuthOpenBrowser,
		Output:       stderr,
	})
	if err != nil {
		return err
	}
	if _, err := runtime.SaveProfile(ctx, result.Profile, cfg.MakeDefault); err != nil {
		return err
	}
	return renderProfileCatalog(ctx, runtime, cfg.Format, stdout)
}

func logoutProfileAndRenderCatalog(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	if _, err := runtime.LogoutProfile(ctx, cfg.LogoutProfileID); err != nil {
		return err
	}
	return renderProfileCatalog(ctx, runtime, cfg.Format, stdout)
}

func renderResult(stdout io.Writer, format outputFormat, sessionResult result) error {
	switch format {
	case outputFormatText:
		return renderText(stdout, sessionResult)
	case outputFormatJSON:
		fallthrough
	default:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(sessionResult)
	}
}

func renderProfileCatalogText(stdout io.Writer, catalog profileCatalogResult) error {
	lines := []string{
		"cc-engine configured profiles",
	}
	for _, profile := range catalog.Profiles {
		line := fmt.Sprintf(
			"- %s (%s/%s) default_model=%s valid=%t auth=%s",
			profile.Profile.ID,
			profile.Profile.Provider,
			profile.Profile.Kind,
			profile.Profile.DefaultModel,
			profile.Validation.Valid,
			profile.Auth.State,
		)
		lines = append(lines, line)
		if profile.Auth.ExpiresAt != "" {
			lines = append(lines, fmt.Sprintf("  expires_at: %s", profile.Auth.ExpiresAt))
		}
		if profile.Auth.CanRefresh {
			lines = append(lines, "  can_refresh: true")
		}
		if profile.Validation.Message != "" {
			lines = append(lines, fmt.Sprintf("  validation: %s", profile.Validation.Message))
		}
		if len(profile.Models) > 0 {
			lines = append(lines, fmt.Sprintf("  models: %s", strings.Join(profile.Models, ", ")))
		}
		if caps := formatCapabilities(profile.Capabilities); caps != "" {
			lines = append(lines, fmt.Sprintf("  capabilities: %s", caps))
		}
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderModelCatalogText(stdout io.Writer, catalog modelCatalogResult) error {
	lines := []string{
		"cc-engine model catalog",
		fmt.Sprintf("profile: %s", catalog.ProfileID),
		fmt.Sprintf("default_model: %s", catalog.DefaultModel),
	}
	if len(catalog.Models) > 0 {
		lines = append(lines, fmt.Sprintf("models: %s", strings.Join(catalog.Models, ", ")))
	}
	if caps := formatCapabilities(catalog.Capabilities); caps != "" {
		lines = append(lines, fmt.Sprintf("capabilities: %s", caps))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderSessionStatusText(stdout io.Writer, result sessionStatusResult) error {
	lines := []string{
		"cc-engine session status",
		fmt.Sprintf("session: %s", result.Session),
		fmt.Sprintf("mode: %s", result.Summary.Mode),
		fmt.Sprintf("status: %s", result.Summary.Status),
		fmt.Sprintf("profile: %s", result.Summary.ProfileID),
		fmt.Sprintf("model: %s", result.Summary.Model),
		fmt.Sprintf("turns: %d", result.Summary.TurnCount),
		fmt.Sprintf("events: %d", result.Summary.EventCount),
		fmt.Sprintf("last_event: %s", result.Summary.LastEventKind),
		fmt.Sprintf("terminal_outcome: %s", result.Summary.TerminalOutcome),
	}
	if result.Summary.ClosedReason != "" {
		lines = append(lines, fmt.Sprintf("closed_reason: %s", result.Summary.ClosedReason))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderReplayPackText(stdout io.Writer, pack contracts.ReplayPack) error {
	lines := []string{
		"cc-engine replay pack",
		fmt.Sprintf("session: %s", pack.Session.SessionID),
		fmt.Sprintf("mode: %s", pack.Summary.Mode),
		fmt.Sprintf("status: %s", pack.Summary.Status),
		fmt.Sprintf("profile: %s", pack.Summary.ProfileID),
		fmt.Sprintf("model: %s", pack.Summary.Model),
		fmt.Sprintf("events: %d", len(pack.Events)),
		fmt.Sprintf("terminal_outcome: %s", pack.Summary.TerminalOutcome),
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func formatCapabilities(caps contracts.CapabilitySet) string {
	enabled := make([]string, 0, 9)
	if caps.Streaming {
		enabled = append(enabled, "streaming")
	}
	if caps.ToolCalling {
		enabled = append(enabled, "tool_calling")
	}
	if caps.StructuredOutputs {
		enabled = append(enabled, "structured_outputs")
	}
	if caps.TokenCounting {
		enabled = append(enabled, "token_counting")
	}
	if caps.PromptCaching {
		enabled = append(enabled, "prompt_caching")
	}
	if caps.ReasoningControls {
		enabled = append(enabled, "reasoning_controls")
	}
	if caps.DeferredToolSearch {
		enabled = append(enabled, "deferred_tool_search")
	}
	if caps.ImageInput {
		enabled = append(enabled, "image_input")
	}
	if caps.DocumentInput {
		enabled = append(enabled, "document_input")
	}
	return strings.Join(enabled, ", ")
}

func selectProfileStatus(profiles []contracts.ProfileStatus, profileID string) (contracts.ProfileStatus, error) {
	trimmed := strings.TrimSpace(profileID)
	if trimmed == "" {
		return profiles[0], nil
	}
	for _, profile := range profiles {
		if profile.Profile.ID == trimmed {
			return profile, nil
		}
	}
	return contracts.ProfileStatus{}, fmt.Errorf("unknown profile: %s", trimmed)
}

func buildProfileFromConfig(cfg config) (contracts.AuthProfile, error) {
	profileID := strings.TrimSpace(cfg.ProfileID)
	if profileID == "" {
		return contracts.AuthProfile{}, fmt.Errorf("-profile-id is required for -upsert-profile")
	}

	profileKind, providerKind, err := resolveProfileKinds(cfg)
	if err != nil {
		return contracts.AuthProfile{}, err
	}

	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	if defaultModel == "" {
		defaultModel = defaultModelForProvider(providerKind)
	}

	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		displayName = profileID
	}

	settings := map[string]string{}
	if credentialRef := strings.TrimSpace(cfg.CredentialRef); credentialRef != "" {
		settings["credential_ref"] = credentialRef
	}

	switch profileKind {
	case contracts.AuthProfileAnthropicOAuth:
		oauthHost := strings.TrimSpace(cfg.OAuthHost)
		if oauthHost == "" {
			oauthHost = "https://claude.ai"
		}
		accountScope := strings.TrimSpace(cfg.AccountScope)
		if accountScope == "" {
			accountScope = "claude"
		}
		settings["oauth_host"] = oauthHost
		settings["account_scope"] = accountScope
	case contracts.AuthProfileAnthropicAPIKey:
		apiBase := strings.TrimSpace(cfg.APIBase)
		if apiBase == "" {
			apiBase = "https://api.anthropic.com"
		}
		settings["api_base"] = apiBase
	case contracts.AuthProfileOpenRouterAPIKey:
		apiBase := strings.TrimSpace(cfg.APIBase)
		if apiBase == "" {
			apiBase = "https://openrouter.ai/api/v1"
		}
		settings["api_base"] = apiBase
		settings["app_name"] = "Klaude Kode"
		settings["http_referer"] = "https://local.cli"
	}

	return contracts.AuthProfile{
		ID:           profileID,
		Kind:         profileKind,
		Provider:     providerKind,
		DisplayName:  displayName,
		DefaultModel: defaultModel,
		Settings:     settings,
	}, nil
}

func resolveProfileKinds(cfg config) (contracts.AuthProfileKind, contracts.ProviderKind, error) {
	profileKind := contracts.AuthProfileKind(strings.TrimSpace(cfg.ProfileKind))
	providerKind := contracts.ProviderKind(strings.TrimSpace(cfg.ProfileProvider))

	if providerKind == "" {
		switch profileKind {
		case contracts.AuthProfileAnthropicOAuth, contracts.AuthProfileAnthropicAPIKey:
			providerKind = contracts.ProviderAnthropic
		case contracts.AuthProfileOpenRouterAPIKey:
			providerKind = contracts.ProviderOpenRouter
		default:
			providerKind = provider.ResolveSessionProfile(cfg.ProfileID, cfg.DefaultModel).Provider
		}
	}

	if profileKind == "" {
		switch providerKind {
		case contracts.ProviderOpenRouter:
			profileKind = contracts.AuthProfileOpenRouterAPIKey
		case contracts.ProviderAnthropic:
			profileKind = contracts.AuthProfileAnthropicAPIKey
		default:
			return "", "", fmt.Errorf("unsupported provider %q", providerKind)
		}
	}

	switch providerKind {
	case contracts.ProviderAnthropic, contracts.ProviderOpenRouter:
	default:
		return "", "", fmt.Errorf("unsupported provider %q", providerKind)
	}

	switch profileKind {
	case contracts.AuthProfileAnthropicOAuth, contracts.AuthProfileAnthropicAPIKey, contracts.AuthProfileOpenRouterAPIKey:
	default:
		return "", "", fmt.Errorf("unsupported profile kind %q", profileKind)
	}

	if providerKind == contracts.ProviderOpenRouter && profileKind != contracts.AuthProfileOpenRouterAPIKey {
		return "", "", fmt.Errorf("openrouter profiles must use kind %q", contracts.AuthProfileOpenRouterAPIKey)
	}
	if providerKind == contracts.ProviderAnthropic && profileKind == contracts.AuthProfileOpenRouterAPIKey {
		return "", "", fmt.Errorf("anthropic profiles cannot use kind %q", contracts.AuthProfileOpenRouterAPIKey)
	}

	return profileKind, providerKind, nil
}

func defaultModelForProvider(kind contracts.ProviderKind) string {
	switch kind {
	case contracts.ProviderOpenRouter:
		return "openrouter/auto"
	case contracts.ProviderAnthropic:
		fallthrough
	default:
		return "claude-sonnet-4-6"
	}
}

func renderText(stdout io.Writer, sessionResult result) error {
	_, err := fmt.Fprintf(
		stdout,
		"cc-engine headless session\nsession: %s\nmode: %s\nstatus: %s\nevents: %d\nlast_event: %s\nclosed_reason: %s\n",
		sessionResult.Session.SessionID,
		sessionResult.Session.Mode,
		sessionResult.Summary.Status,
		sessionResult.Summary.EventCount,
		sessionResult.Summary.LastEventKind,
		sessionResult.Summary.ClosedReason,
	)
	return err
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
