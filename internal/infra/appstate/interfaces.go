package appstate

import (
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/pinger"
)

type pingerStatsGetter interface {
	GetAllStats() map[string]*pinger.Statistics
}

// pingerServer is an internal interface for pinger management
type pingerServer interface {
	Register(pinger pinger.Pinger) error
	pingerStatsGetter
}

// healthChecker is an internal interface for health checking
type healthChecker interface {
	pingerStatsGetter
	IsHealthy() bool
}

// readyChecker is an internal interface for readiness checking
type readyChecker interface {
	pingerStatsGetter
	IsReady() bool
}

// statusGetter is an internal interface for getting the application status
type statusGetter interface {
	pingerStatsGetter
	GetState() State
	GetUptime() time.Duration
	GetStartTime() time.Time
}
