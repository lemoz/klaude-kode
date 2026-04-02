package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

type Result struct {
	State    string        `json:"state"`
	ExitCode int           `json:"exit_code"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
}

type LocalRunner struct{}

func NewLocalRunner() LocalRunner {
	return LocalRunner{}
}

func (r LocalRunner) Run(ctx context.Context, definition Definition, payload any) (Result, error) {
	shell := definition.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	runCtx := ctx
	cancel := func() {}
	if definition.TimeoutSeconds > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(definition.TimeoutSeconds)*time.Second)
	}
	defer cancel()

	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return Result{}, fmt.Errorf("marshal hook payload: %w", err)
	}

	startedAt := time.Now()
	cmd := exec.CommandContext(runCtx, shell, "-lc", definition.Command)
	cmd.Stdin = bytes.NewReader(encodedPayload)

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	runErr := cmd.Run()
	duration := time.Since(startedAt)

	result := Result{
		State:    "completed",
		ExitCode: 0,
		Output:   combined.String(),
		Duration: duration,
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.State = "timed_out"
		result.ExitCode = -1
		return result, nil
	}

	if runErr == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.State = "failed"
		result.ExitCode = exitCode(exitErr)
		return result, nil
	}

	return Result{}, fmt.Errorf("run hook: %w", runErr)
}

func exitCode(err *exec.ExitError) int {
	if err == nil {
		return 0
	}
	status, ok := err.Sys().(syscall.WaitStatus)
	if ok {
		return status.ExitStatus()
	}
	return -1
}
