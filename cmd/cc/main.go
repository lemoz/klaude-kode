package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cdossman/klaude-kode/internal/contracts"
	"github.com/cdossman/klaude-kode/internal/engine"
)

func main() {
	ctx := context.Background()
	e := engine.NewInMemoryEngine()

	handle, err := e.StartSession(ctx, contracts.StartSessionRequest{
		SessionID: "local-bootstrap",
		CWD:       mustGetwd(),
		Mode:      contracts.SessionModeInteractive,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start session: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Klaude Kode launcher bootstrap\n")
	fmt.Printf("session: %s\n", handle.SessionID)
	fmt.Printf("mode: %s\n", handle.Mode)
	fmt.Printf("cwd: %s\n", handle.CWD)
	fmt.Printf("status: repo scaffold ready, engine not implemented yet\n")
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

