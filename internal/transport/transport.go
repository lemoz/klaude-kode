package transport

import (
	"context"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type SessionTransport interface {
	Open(ctx context.Context, target contracts.TransportTarget) error
	Send(ctx context.Context, cmd contracts.SessionCommand) error
	Events(ctx context.Context) (<-chan contracts.SessionEvent, error)
	Close(ctx context.Context) error
}

