package engine

import (
	"context"
	"time"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

func ExportReplayPack(ctx context.Context, runtime Engine, sessionID string) (contracts.ReplayPack, error) {
	handle, err := runtime.ResumeSession(ctx, contracts.ResumeSessionRequest{SessionID: sessionID})
	if err != nil {
		return contracts.ReplayPack{}, err
	}

	summary, err := runtime.GetSessionSummary(ctx, handle.SessionID)
	if err != nil {
		return contracts.ReplayPack{}, err
	}

	events, err := runtime.ListEvents(ctx, handle.SessionID)
	if err != nil {
		return contracts.ReplayPack{}, err
	}

	return contracts.ReplayPack{
		SchemaVersion: contracts.SchemaVersionV1,
		ExportedAt:    time.Now().UTC(),
		Session:       handle,
		Summary:       summary,
		Events:        events,
	}, nil
}
