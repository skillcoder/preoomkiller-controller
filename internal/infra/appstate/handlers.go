package appstate

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type statusResponse struct {
	State     string    `json:"state"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"startTime"`
	UptimeSec float64   `json:"uptimeSeconds"`
}

// HandleHealthz returns an http.HandlerFunc for the /-/healthz endpoint
func HandleHealthz(
	logger *slog.Logger,
	appState healthChecker,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := middleware.GetReqID(ctx)
		logger = logger.With("traceID", requestID)

		if !appState.IsHealthy() {
			w.WriteHeader(http.StatusServiceUnavailable)
			logger.DebugContext(ctx, "health check failed")

			return
		}

		w.WriteHeader(http.StatusOK)
		logger.DebugContext(ctx, "health check passed")
	}
}

// HandleReadyz returns an http.HandlerFunc for the /-/readyz endpoint
func HandleReadyz(
	logger *slog.Logger,
	appState readyChecker,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := middleware.GetReqID(ctx)
		logger = logger.With("traceID", requestID)

		if !appState.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			logger.DebugContext(ctx, "readiness check failed")

			return
		}

		w.WriteHeader(http.StatusOK)
		logger.DebugContext(ctx, "readiness check passed")
	}
}

// HandleStatus returns an http.HandlerFunc for the /-/status endpoint
func HandleStatus(
	logger *slog.Logger,
	appState statusGetter,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		requestID := middleware.GetReqID(ctx)
		logger = logger.With("traceID", requestID)

		state := appState.GetState()
		uptime := appState.GetUptime()
		startTime := appState.GetStartTime()

		response := statusResponse{
			State:     string(state),
			Uptime:    uptime.String(),
			StartTime: startTime,
			UptimeSec: uptime.Seconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			logger.ErrorContext(ctx, "failed to encode status response",
				"error", err,
			)

			return
		}

		logger.DebugContext(ctx, "status response sent",
			"state", string(state),
			"uptime", uptime.String(),
		)
	}
}
