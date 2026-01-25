package httpserver

import (
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
)

// appstater is an internal interface for application state management
type appstater interface {
	GetState() appstate.State
	IsHealthy() bool
	IsReady() bool
	GetUptime() time.Duration
	GetStartTime() time.Time
}
