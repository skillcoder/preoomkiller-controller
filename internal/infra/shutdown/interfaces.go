package shutdown

import "context"

// Shutdowner handles graceful shutdown operations.
type Shutdowner interface {
	HandleSignals(ctx context.Context, cancel func())
	CheckTermination(ctx context.Context) error
}
