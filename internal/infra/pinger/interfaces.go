package pinger

import "context"

// Pinger defines the interface for health check pingers
type Pinger interface {
	Name() string
	Ping(ctx context.Context) error
}
