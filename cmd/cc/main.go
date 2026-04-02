package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
	"github.com/cdossman/klaude-kode/internal/harness"
	"github.com/cdossman/klaude-kode/internal/transport"
)

type outputFormat string

const (
	outputFormatJSON   outputFormat = "json"
	outputFormatText   outputFormat = "text"
	outputFormatEvents outputFormat = "events"
)

type config struct {
	Format            outputFormat
	ListProfiles      bool
	ListModels        bool
	ShowStatus        bool
	ExportReplayPack  bool
	ValidateCandidate bool
	RunReplayEval     bool
	RunBenchmarkEval  bool
	SummarizeRuns     bool
	ShowRun           bool
	DiffRuns          bool
	ListFrontier      bool
	Prompt            string
	SessionID         string
	ResumeSessionID   string
	CWD               string
	ProfileID         string
	Model             string
	StateRoot         string
	ReplayPath        string
	BenchmarkPath     string
	RunID             string
	LeftRunID         string
	RightRunID        string
	FrontierLimit     int
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
	if cfg.ExportReplayPack {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-export-replay-pack does not support -format=events")
		}
		return renderReplayPack(ctx, runtime, cfg, stdout)
	}
	if cfg.ValidateCandidate {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-validate-candidate does not support -format=events")
		}
		return renderCandidateValidation(cfg, stdout)
	}
	if cfg.SummarizeRuns {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-summarize-runs does not support -format=events")
		}
		return renderRunSummary(cfg, stdout)
	}
	if cfg.ShowRun {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-show-run does not support -format=events")
		}
		return renderShowRun(cfg, stdout)
	}
	if cfg.DiffRuns {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-diff-runs does not support -format=events")
		}
		return renderDiffRuns(cfg, stdout)
	}
	if cfg.ListFrontier {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-list-frontier does not support -format=events")
		}
		return renderListFrontier(cfg, stdout)
	}
	if cfg.RunReplayEval {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-run-replay-eval does not support -format=events")
		}
		return renderReplayEval(cfg, stdout)
	}
	if cfg.RunBenchmarkEval {
		if cfg.Format == outputFormatEvents {
			return fmt.Errorf("-run-benchmark-eval does not support -format=events")
		}
		return renderBenchmarkEval(cfg, stdout)
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
	exportReplayPackValue := fs.Bool("export-replay-pack", false, "export a replay pack for an existing session and exit")
	validateCandidateValue := fs.Bool("validate-candidate", false, "validate a candidate root and exit")
	summarizeRunsValue := fs.Bool("summarize-runs", false, "summarize persisted replay evaluation runs and exit")
	showRunValue := fs.Bool("show-run", false, "show a persisted replay evaluation run and exit")
	diffRunsValue := fs.Bool("diff-runs", false, "diff two persisted replay evaluation runs and exit")
	listFrontierValue := fs.Bool("list-frontier", false, "list the best persisted eval runs and exit")
	runReplayEvalValue := fs.Bool("run-replay-eval", false, "run a replay evaluation for the current candidate root and exit")
	runBenchmarkEvalValue := fs.Bool("run-benchmark-eval", false, "run a benchmark evaluation for the current candidate root and exit")
	promptValue := fs.String("prompt", "bootstrap hello from cc", "prompt to submit to the session")
	sessionIDValue := fs.String("session-id", "cc-bootstrap", "session identifier")
	resumeSessionValue := fs.String("resume-session", "", "load and render a persisted session")
	cwdValue := fs.String("cwd", mustGetwd(), "session working directory")
	profileIDValue := fs.String("profile-id", "", "active auth profile id")
	modelValue := fs.String("model", "", "active model id")
	stateRootValue := fs.String("state-root", engine.DefaultStateRoot(), "engine state root")
	replayPathValue := fs.String("replay-path", "", "path to a replay pack file for harness evaluation")
	benchmarkPathValue := fs.String("benchmark-path", "", "path to a benchmark pack file for harness evaluation")
	runIDValue := fs.String("run-id", "", "run identifier for -show-run")
	leftRunIDValue := fs.String("left-run-id", "", "left run identifier for -diff-runs")
	rightRunIDValue := fs.String("right-run-id", "", "right run identifier for -diff-runs")
	frontierLimitValue := fs.Int("frontier-limit", 10, "maximum number of frontier entries to return")

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
		Format:            format,
		ListProfiles:      *listProfilesValue,
		ListModels:        *listModelsValue,
		ShowStatus:        *showStatusValue,
		ExportReplayPack:  *exportReplayPackValue,
		ValidateCandidate: *validateCandidateValue,
		SummarizeRuns:     *summarizeRunsValue,
		ShowRun:           *showRunValue,
		DiffRuns:          *diffRunsValue,
		ListFrontier:      *listFrontierValue,
		RunReplayEval:     *runReplayEvalValue,
		RunBenchmarkEval:  *runBenchmarkEvalValue,
		Prompt:            strings.TrimSpace(*promptValue),
		SessionID:         strings.TrimSpace(*sessionIDValue),
		ResumeSessionID:   strings.TrimSpace(*resumeSessionValue),
		CWD:               strings.TrimSpace(*cwdValue),
		ProfileID:         strings.TrimSpace(*profileIDValue),
		Model:             strings.TrimSpace(*modelValue),
		StateRoot:         strings.TrimSpace(*stateRootValue),
		ReplayPath:        strings.TrimSpace(*replayPathValue),
		BenchmarkPath:     strings.TrimSpace(*benchmarkPathValue),
		RunID:             strings.TrimSpace(*runIDValue),
		LeftRunID:         strings.TrimSpace(*leftRunIDValue),
		RightRunID:        strings.TrimSpace(*rightRunIDValue),
		FrontierLimit:     *frontierLimitValue,
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
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(pack)
	case outputFormatText:
		fallthrough
	default:
		return renderReplayPackText(stdout, pack)
	}
}

func renderCandidateValidation(cfg config, stdout io.Writer) error {
	result, err := harness.ValidateCandidateRoot(cfg.CWD)
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case outputFormatText:
		fallthrough
	default:
		return renderCandidateValidationText(stdout, result)
	}
}

func renderReplayEval(cfg config, stdout io.Writer) error {
	if cfg.ReplayPath == "" {
		return fmt.Errorf("a replay path is required for -run-replay-eval")
	}

	run, err := harness.RunReplayEval(cfg.CWD, cfg.ReplayPath)
	if err != nil {
		return err
	}
	if _, err := harness.PersistEvalRun(harness.DefaultArtifactRoot(run.Candidate.Root), run); err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(run)
	case outputFormatText:
		fallthrough
	default:
		return renderReplayEvalText(stdout, run)
	}
}

func renderBenchmarkEval(cfg config, stdout io.Writer) error {
	if cfg.BenchmarkPath == "" {
		return fmt.Errorf("a benchmark path is required for -run-benchmark-eval")
	}

	run, err := harness.RunBenchmarkEval(cfg.CWD, cfg.BenchmarkPath)
	if err != nil {
		return err
	}
	if _, err := harness.PersistEvalRun(harness.DefaultArtifactRoot(run.Candidate.Root), run); err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(run)
	case outputFormatText:
		fallthrough
	default:
		return renderBenchmarkEvalText(stdout, run)
	}
}

func renderRunSummary(cfg config, stdout io.Writer) error {
	summary, err := harness.SummarizeIndexedEvalRuns(harness.DefaultArtifactRoot(cfg.CWD))
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	case outputFormatText:
		fallthrough
	default:
		return renderRunSummaryText(stdout, summary)
	}
}

func renderShowRun(cfg config, stdout io.Writer) error {
	if cfg.RunID == "" {
		return fmt.Errorf("a run id is required for -show-run")
	}

	run, err := harness.LoadEvalRun(harness.DefaultArtifactRoot(cfg.CWD), cfg.RunID)
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(run)
	case outputFormatText:
		fallthrough
	default:
		return renderShowRunText(stdout, run)
	}
}

func renderDiffRuns(cfg config, stdout io.Writer) error {
	if cfg.LeftRunID == "" || cfg.RightRunID == "" {
		return fmt.Errorf("left and right run ids are required for -diff-runs")
	}

	diff, err := harness.DiffPersistedEvalRuns(harness.DefaultArtifactRoot(cfg.CWD), cfg.LeftRunID, cfg.RightRunID)
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(diff)
	case outputFormatText:
		fallthrough
	default:
		return renderDiffRunsText(stdout, diff)
	}
}

func renderListFrontier(cfg config, stdout io.Writer) error {
	entries, err := harness.ListFrontier(harness.DefaultArtifactRoot(cfg.CWD), cfg.FrontierLimit)
	if err != nil {
		return err
	}

	switch cfg.Format {
	case outputFormatJSON:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	case outputFormatText:
		fallthrough
	default:
		return renderListFrontierText(stdout, entries)
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
			"- %s (%s/%s) default_model=%s valid=%t auth=%s auth_method=%s",
			profile.Profile.ID,
			profile.Profile.Provider,
			profile.Profile.Kind,
			profile.Profile.DefaultModel,
			profile.Validation.Valid,
			profile.Auth.State,
			profile.Auth.Method,
		)
		lines = append(lines, line)
		if profile.Auth.ExpiresAt != "" {
			lines = append(lines, fmt.Sprintf("  expires_at: %s", profile.Auth.ExpiresAt))
		} else {
			lines = append(lines, "  expires_at: n/a")
		}
		lines = append(lines, fmt.Sprintf("  auth_method: %s", profile.Profile.Kind))
		lines = append(lines, fmt.Sprintf("  default_model: %s", profile.Profile.DefaultModel))
		lines = append(lines, fmt.Sprintf("  can_refresh: %t", profile.Auth.CanRefresh))
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

func renderReplayPackText(stdout io.Writer, pack contracts.ReplayPack) error {
	lines := []string{
		"cc replay pack",
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

func renderCandidateValidationText(stdout io.Writer, result harness.CandidateValidationResult) error {
	lines := []string{
		"cc candidate validation",
		fmt.Sprintf("valid: %t", result.Valid),
		fmt.Sprintf("root: %s", result.Candidate.Root),
		fmt.Sprintf("default_profile: %s", result.Candidate.DefaultProfileID),
		fmt.Sprintf("default_model: %s", result.Candidate.DefaultModel),
	}
	if len(result.Issues) > 0 {
		lines = append(lines, "issues:")
		for _, issue := range result.Issues {
			lines = append(lines, "  - "+issue)
		}
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderReplayEvalText(stdout io.Writer, run harness.EvalRun) error {
	lines := []string{
		"cc replay eval",
		fmt.Sprintf("run: %s", run.ID),
		fmt.Sprintf("candidate_root: %s", run.Candidate.Root),
		fmt.Sprintf("replay_path: %s", run.ReplayPath),
		fmt.Sprintf("status: %s", run.Status),
		fmt.Sprintf("score: %.2f", run.Score),
	}
	if run.Failure != nil {
		lines = append(lines, fmt.Sprintf("failure_code: %s", run.Failure.Code))
		lines = append(lines, fmt.Sprintf("failure_message: %s", run.Failure.Message))
		lines = append(lines, fmt.Sprintf("retryable: %t", run.Failure.Retryable))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderRunSummaryText(stdout io.Writer, summary harness.EvalRunSummary) error {
	lines := []string{
		"cc run summary",
		fmt.Sprintf("artifact_root: %s", summary.ArtifactRoot),
		fmt.Sprintf("total_runs: %d", summary.TotalRuns),
		fmt.Sprintf("completed: %d", summary.Completed),
		fmt.Sprintf("failed: %d", summary.Failed),
		fmt.Sprintf("average_score: %.2f", summary.AverageScore),
		fmt.Sprintf("latest_run: %s", summary.LatestRunID),
		fmt.Sprintf("latest_status: %s", summary.LatestStatus),
	}
	if len(summary.FailureCodes) > 0 {
		keys := make([]string, 0, len(summary.FailureCodes))
		for code := range summary.FailureCodes {
			keys = append(keys, code)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, code := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", code, summary.FailureCodes[code]))
		}
		lines = append(lines, fmt.Sprintf("failure_codes: %s", strings.Join(parts, ", ")))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderShowRunText(stdout io.Writer, run harness.EvalRun) error {
	lines := []string{
		"cc replay run",
		fmt.Sprintf("run: %s", run.ID),
		fmt.Sprintf("candidate_root: %s", run.Candidate.Root),
		fmt.Sprintf("replay_path: %s", run.ReplayPath),
		fmt.Sprintf("status: %s", run.Status),
		fmt.Sprintf("score: %.2f", run.Score),
	}
	if run.Failure != nil {
		lines = append(lines, fmt.Sprintf("failure_code: %s", run.Failure.Code))
		lines = append(lines, fmt.Sprintf("failure_message: %s", run.Failure.Message))
		lines = append(lines, fmt.Sprintf("retryable: %t", run.Failure.Retryable))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderBenchmarkEvalText(stdout io.Writer, run harness.EvalRun) error {
	lines := []string{
		"cc benchmark eval",
		fmt.Sprintf("run: %s", run.ID),
		fmt.Sprintf("candidate_root: %s", run.Candidate.Root),
		fmt.Sprintf("status: %s", run.Status),
		fmt.Sprintf("score: %.2f", run.Score),
	}
	if run.Benchmark != nil {
		lines = append(lines, fmt.Sprintf("benchmark: %s", run.Benchmark.Name))
		lines = append(lines, fmt.Sprintf("benchmark_path: %s", run.Benchmark.Path))
		lines = append(lines, fmt.Sprintf("cases: %d", run.Benchmark.CaseCount))
	}
	if run.Failure != nil {
		lines = append(lines, fmt.Sprintf("failure_code: %s", run.Failure.Code))
		lines = append(lines, fmt.Sprintf("failure_message: %s", run.Failure.Message))
		lines = append(lines, fmt.Sprintf("retryable: %t", run.Failure.Retryable))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderDiffRunsText(stdout io.Writer, diff harness.EvalRunDiff) error {
	lines := []string{
		"cc run diff",
		fmt.Sprintf("left_run: %s", diff.LeftRunID),
		fmt.Sprintf("right_run: %s", diff.RightRunID),
		fmt.Sprintf("left_status: %s", diff.LeftStatus),
		fmt.Sprintf("right_status: %s", diff.RightStatus),
		fmt.Sprintf("left_score: %.2f", diff.LeftScore),
		fmt.Sprintf("right_score: %.2f", diff.RightScore),
		fmt.Sprintf("score_delta: %.2f", diff.ScoreDelta),
	}
	if diff.LeftFailureCode != "" || diff.RightFailureCode != "" {
		lines = append(lines, fmt.Sprintf("left_failure_code: %s", diff.LeftFailureCode))
		lines = append(lines, fmt.Sprintf("right_failure_code: %s", diff.RightFailureCode))
	}
	if len(diff.CaseDiffs) > 0 {
		lines = append(lines, fmt.Sprintf("case_diffs: %d", len(diff.CaseDiffs)))
	}
	_, err := fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return err
}

func renderListFrontierText(stdout io.Writer, entries []harness.FrontierEntry) error {
	lines := []string{
		"cc frontier",
		fmt.Sprintf("entries: %d", len(entries)),
	}
	for _, entry := range entries {
		line := fmt.Sprintf("- %s kind=%s status=%s score=%.2f", entry.RunID, entry.Kind, entry.Status, entry.Score)
		lines = append(lines, line)
		if entry.Benchmark != "" {
			lines = append(lines, fmt.Sprintf("  benchmark: %s", entry.Benchmark))
		}
		if entry.FailureCode != "" {
			lines = append(lines, fmt.Sprintf("  failure_code: %s", entry.FailureCode))
		}
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
