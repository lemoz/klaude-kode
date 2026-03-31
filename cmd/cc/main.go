package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
	"github.com/cdossman/klaude-kode/internal/transport"
)

type outputFormat string

const (
	outputFormatJSON   outputFormat = "json"
	outputFormatText   outputFormat = "text"
	outputFormatEvents outputFormat = "events"
)

type config struct {
	Format          outputFormat
	ListProfiles    bool
	ListModels      bool
	ShowStatus      bool
	Prompt          string
	SessionID       string
	ResumeSessionID string
	CWD             string
	ProfileID       string
	Model           string
	StateRoot       string
}

type result struct {
	Launcher  string                   `json:"launcher"`
	Transport string                   `json:"transport"`
	Session   contracts.SessionHandle  `json:"session"`
	Summary   contracts.SessionSummary `json:"summary"`
	Events    []contracts.SessionEvent `json:"events"`
}

type profileCatalogResult struct {
	Launcher string                    `json:"launcher"`
	Profiles []contracts.ProfileStatus `json:"profiles"`
}

type modelCatalogResult struct {
	Launcher     string                  `json:"launcher"`
	ProfileID    string                  `json:"profile_id"`
	DefaultModel string                  `json:"default_model"`
	Models       []string                `json:"models"`
	Capabilities contracts.CapabilitySet `json:"capabilities"`
}

type sessionStatusResult struct {
	Launcher string                   `json:"launcher"`
	Session  string                   `json:"session"`
	Summary  contracts.SessionSummary `json:"summary"`
}

type collectedEvents struct {
	Events []contracts.SessionEvent
	Err    error
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
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
	if cfg.ResumeSessionID != "" {
		return renderPersistedSession(ctx, runtime, cfg, stdout)
	}

	sessionResult, err := executeLocalSession(ctx, runtime, cfg)
	if err != nil {
		return err
	}

	return renderResult(stdout, cfg.Format, sessionResult)
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("cc", flag.ContinueOnError)
	fs.SetOutput(stderr)

	formatValue := fs.String("format", string(outputFormatText), "output format: json, text, events")
	listProfilesValue := fs.Bool("profiles", false, "list configured auth profiles and exit")
	listModelsValue := fs.Bool("models", false, "list available models for the selected or default profile and exit")
	showStatusValue := fs.Bool("status", false, "show summary for an existing session and exit")
	promptValue := fs.String("prompt", "bootstrap hello from cc", "prompt to submit to the session")
	sessionIDValue := fs.String("session-id", "cc-bootstrap", "session identifier")
	resumeSessionValue := fs.String("resume-session", "", "load and render a persisted session")
	cwdValue := fs.String("cwd", mustGetwd(), "session working directory")
	profileIDValue := fs.String("profile-id", "", "active auth profile id")
	modelValue := fs.String("model", "", "active model id")
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

	return config{
		Format:          format,
		ListProfiles:    *listProfilesValue,
		ListModels:      *listModelsValue,
		ShowStatus:      *showStatusValue,
		Prompt:          strings.TrimSpace(*promptValue),
		SessionID:       strings.TrimSpace(*sessionIDValue),
		ResumeSessionID: strings.TrimSpace(*resumeSessionValue),
		CWD:             strings.TrimSpace(*cwdValue),
		ProfileID:       strings.TrimSpace(*profileIDValue),
		Model:           strings.TrimSpace(*modelValue),
		StateRoot:       strings.TrimSpace(*stateRootValue),
	}, nil
}

func executeLocalSession(ctx context.Context, runtime engine.Engine, cfg config) (result, error) {
	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: cfg.SessionID,
		CWD:       cfg.CWD,
		Mode:      contracts.SessionModeInteractive,
		ProfileID: cfg.ProfileID,
		Model:     cfg.Model,
	})
	if err != nil {
		return result{}, err
	}

	localTransport := transport.NewLocalTransport(runtime)
	if err := localTransport.Open(ctx, contracts.TransportTarget{
		Kind: "local",
		Addr: handle.SessionID,
	}); err != nil {
		return result{}, err
	}
	defer localTransport.Close(context.Background())

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := localTransport.Events(streamCtx)
	if err != nil {
		return result{}, err
	}

	collected := make(chan collectedEvents, 1)
	go func() {
		events := make([]contracts.SessionEvent, 0, 16)
		for event := range stream {
			events = append(events, event)
		}
		collected <- collectedEvents{Events: events}
	}()

	if err := localTransport.Send(ctx, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   cfg.Prompt,
			Source: contracts.MessageSourceInteractive,
		},
	}); err != nil {
		cancel()
		drainCollected(collected)
		return result{}, err
	}
	if err := localTransport.Send(ctx, contracts.SessionCommand{
		Kind: contracts.CommandKindCloseSession,
		Payload: contracts.SessionCommandPayload{
			Reason: "cc_complete",
		},
	}); err != nil {
		cancel()
		drainCollected(collected)
		return result{}, err
	}

	streamResult := <-collected
	if streamResult.Err != nil {
		return result{}, streamResult.Err
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		return result{}, err
	}

	return result{
		Launcher:  "cc",
		Transport: "local",
		Session:   handle,
		Summary:   summary,
		Events:    streamResult.Events,
	}, nil
}

func renderPersistedSession(ctx context.Context, runtime engine.Engine, cfg config, stdout io.Writer) error {
	sessionResult, err := loadSessionResult(ctx, runtime, cfg.ResumeSessionID)
	if err != nil {
		return err
	}

	if cfg.Format == outputFormatEvents {
		encoder := json.NewEncoder(stdout)
		for _, event := range sessionResult.Events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		return nil
	}

	return renderResult(stdout, cfg.Format, sessionResult)
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
		Launcher:  "cc",
		Transport: "local",
		Session:   handle,
		Summary:   summary,
		Events:    events,
	}, nil
}

func renderProfileCatalog(ctx context.Context, runtime engine.Engine, format outputFormat, stdout io.Writer) error {
	profiles, err := runtime.ListProfiles(ctx)
	if err != nil {
		return err
	}

	catalog := profileCatalogResult{
		Launcher: "cc",
		Profiles: profiles,
	}

	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	case outputFormatText:
		fallthrough
	default:
		return renderProfileCatalogText(stdout, catalog)
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
		Launcher:     "cc",
		ProfileID:    selected.Profile.ID,
		DefaultModel: selected.Profile.DefaultModel,
		Models:       selected.Models,
		Capabilities: selected.Capabilities,
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalog)
	case outputFormatText:
		fallthrough
	default:
		return renderModelCatalogText(stdout, catalog)
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
		Launcher: "cc",
		Session:  sessionID,
		Summary:  summary,
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case outputFormatText:
		fallthrough
	default:
		return renderSessionStatusText(stdout, result)
	}
}

func renderResult(stdout io.Writer, format outputFormat, sessionResult result) error {
	switch format {
	case outputFormatEvents:
		encoder := json.NewEncoder(stdout)
		for _, event := range sessionResult.Events {
			if err := encoder.Encode(event); err != nil {
				return err
			}
		}
		return nil
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(sessionResult)
	case outputFormatText:
		fallthrough
	default:
		return renderText(stdout, sessionResult)
	}
}

func renderProfileCatalogText(stdout io.Writer, catalog profileCatalogResult) error {
	lines := []string{
		"cc configured profiles",
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
		"cc model catalog",
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
		"cc session status",
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

func renderText(stdout io.Writer, sessionResult result) error {
	_, err := fmt.Fprintf(
		stdout,
		"cc local session\nsession: %s\nmode: %s\ntransport: %s\nstatus: %s\nevents: %d\nlast_event: %s\nclosed_reason: %s\n",
		sessionResult.Session.SessionID,
		sessionResult.Session.Mode,
		sessionResult.Transport,
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

func drainCollected(collected <-chan collectedEvents) {
	select {
	case <-collected:
	default:
	}
}
