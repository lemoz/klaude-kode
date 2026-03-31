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
)

type outputFormat string

const (
	outputFormatJSON   outputFormat = "json"
	outputFormatText   outputFormat = "text"
	outputFormatEvents outputFormat = "events"
)

type config struct {
	Format    outputFormat
	Prompt    string
	SessionID string
	CWD       string
	ProfileID string
	Model     string
}

type result struct {
	Engine  string                   `json:"engine"`
	Session contracts.SessionHandle  `json:"session"`
	Summary contracts.SessionSummary `json:"summary"`
	Events  []contracts.SessionEvent `json:"events"`
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

	runtime := engine.NewInMemoryEngine()
	ctx := context.Background()

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

	formatValue := fs.String("format", string(outputFormatJSON), "output format: json, text, events")
	promptValue := fs.String("prompt", "bootstrap hello from cc-engine", "prompt to submit to the session")
	sessionIDValue := fs.String("session-id", "engine-bootstrap", "session identifier")
	cwdValue := fs.String("cwd", mustGetwd(), "session working directory")
	profileIDValue := fs.String("profile-id", "headless-default", "active auth profile id")
	modelValue := fs.String("model", "bootstrap-model", "active model id")

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
		Format:    format,
		Prompt:    strings.TrimSpace(*promptValue),
		SessionID: strings.TrimSpace(*sessionIDValue),
		CWD:       strings.TrimSpace(*cwdValue),
		ProfileID: strings.TrimSpace(*profileIDValue),
		Model:     strings.TrimSpace(*modelValue),
	}, nil
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
