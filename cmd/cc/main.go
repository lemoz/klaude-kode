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
			"- %s (%s/%s) default_model=%s valid=%t",
			profile.Profile.ID,
			profile.Profile.Provider,
			profile.Profile.Kind,
			profile.Profile.DefaultModel,
			profile.Validation.Valid,
		)
		lines = append(lines, line)
		if profile.Validation.Message != "" {
			lines = append(lines, fmt.Sprintf("  validation: %s", profile.Validation.Message))
		}
		if len(profile.Models) > 0 {
			lines = append(lines, fmt.Sprintf("  models: %s", strings.Join(profile.Models, ", ")))
		}
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
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
