package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
)

func main() {
	ctx := context.Background()
	runtime := engine.NewInMemoryEngine()

	handle, err := runtime.StartSession(ctx, contracts.StartSessionRequest{
		CWD:       mustGetwd(),
		Mode:      contracts.SessionModeInteractive,
		ProfileID: "local-default",
		Model:     "bootstrap-model",
	})
	if err != nil {
		fatalf("failed to start session: %v", err)
	}

	prompt := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if prompt == "" {
		prompt = "bootstrap hello from cc"
	}

	err = runtime.SendCommand(ctx, handle.SessionID, contracts.SessionCommand{
		Kind: contracts.CommandKindUserInput,
		Payload: contracts.SessionCommandPayload{
			Text:   prompt,
			Source: contracts.MessageSourceInteractive,
		},
	})
	if err != nil {
		fatalf("failed to send command: %v", err)
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		fatalf("failed to read session summary: %v", err)
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		fatalf("failed to read session events: %v", err)
	}

	printJSON(map[string]any{
		"launcher": "cc",
		"session":  handle,
		"summary":  summary,
		"events":   events,
	})
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func printJSON(v any) {
	encoded, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatalf("failed to marshal json: %v", err)
	}
	fmt.Println(string(encoded))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
