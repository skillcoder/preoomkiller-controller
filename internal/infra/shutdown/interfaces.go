package shutdown

import (
	"context"
	"os"
)

// Shutdowner is the interface that components must implement for graceful shutdown
type Shutdowner interface {
	Name() string
	Shutdown(ctx context.Context) error
}

type quiter interface {
	Quit() <-chan os.Signal
}
