package toolruntime

import (
	"context"

	"github.com/cdossman/klaude-kode/internal/contracts"
)

type Runtime interface {
	ListTools(ctx context.Context, s contracts.SessionContext) ([]contracts.ToolDescriptor, error)
	ExecuteTool(ctx context.Context, s contracts.SessionContext, call contracts.ToolCall) (<-chan contracts.ToolEvent, error)
	ResolveResources(ctx context.Context, s contracts.SessionContext) (contracts.ResourceSnapshot, error)
}

