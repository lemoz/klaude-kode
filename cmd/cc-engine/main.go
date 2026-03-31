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
	Transport       string
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
	Engine  string                   `json:"engine"`
	Session contracts.SessionHandle  `json:"session"`
	Summary contracts.SessionSummary `json:"summary"`
	Events  []contracts.SessionEvent `json:"events"`
}

type profileCatalogResult struct {
	Engine   string                    `json:"engine"`
	Profiles []contracts.ProfileStatus `json:"profiles"`
}

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
	promptValue := fs.String("prompt", "bootstrap hello from cc-engine", "prompt to submit to the session")
	sessionIDValue := fs.String("session-id", "engine-bootstrap", "session identifier")
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

	transportMode := strings.ToLower(strings.TrimSpace(*transportValue))
	switch transportMode {
	case "headless", "stdio":
	default:
		return config{}, fmt.Errorf("unsupported transport %q", *transportValue)
	}

	return config{
		Transport:       transportMode,
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
