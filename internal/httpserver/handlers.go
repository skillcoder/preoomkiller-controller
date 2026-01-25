package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type statusResponse struct {
	State     string    `json:"state"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"startTime"`
	UptimeSec float64   `json:"uptimeSeconds"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if !s.appState.IsHealthy() {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.appState.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	state := s.appState.GetState()
	uptime := s.appState.GetUptime()
	startTime := s.appState.GetStartTime()

	response := statusResponse{
		State:     string(state),
		Uptime:    uptime.String(),
		StartTime: startTime,
		UptimeSec: uptime.Seconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.ErrorContext(ctx, "failed to encode status response",
			"error", err,
		)
	}
}
