package appstate

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serveAndAssertStatus(t *testing.T, handler http.HandlerFunc, path string, wantCode int) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)

	handler.ServeHTTP(rec, req)

	if rec.Code != wantCode {
		t.Errorf("want status %d, got %d", wantCode, rec.Code)
	}
}

//nolint:dupl // healthz and readyz tests follow same pattern for readability
func TestHandleHealthz(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("healthy returns 200", func(t *testing.T) {
		t.Parallel()

		m := newMockhealthChecker(t)
		m.EXPECT().IsHealthy().Return(true).Once()
		m.EXPECT().GetAllStats().Return(nil).Maybe()

		serveAndAssertStatus(t, HandleHealthz(logger, m), "/-/healthz", http.StatusOK)
	})

	t.Run("unhealthy returns 503", func(t *testing.T) {
		t.Parallel()

		m := newMockhealthChecker(t)
		m.EXPECT().IsHealthy().Return(false).Once()
		m.EXPECT().GetAllStats().Return(nil).Maybe()

		serveAndAssertStatus(t, HandleHealthz(logger, m), "/-/healthz", http.StatusServiceUnavailable)
	})
}

//nolint:dupl // healthz and readyz tests follow same pattern for readability
func TestHandleReadyz(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("ready returns 200", func(t *testing.T) {
		t.Parallel()

		m := newMockreadyChecker(t)
		m.EXPECT().IsReady().Return(true).Once()
		m.EXPECT().GetAllStats().Return(nil).Maybe()

		serveAndAssertStatus(t, HandleReadyz(logger, m), "/-/readyz", http.StatusOK)
	})

	t.Run("not ready returns 503", func(t *testing.T) {
		t.Parallel()

		m := newMockreadyChecker(t)
		m.EXPECT().IsReady().Return(false).Once()
		m.EXPECT().GetAllStats().Return(nil).Maybe()

		serveAndAssertStatus(t, HandleReadyz(logger, m), "/-/readyz", http.StatusServiceUnavailable)
	})
}

func TestHandleStatus(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	giveState := StateRunning
	giveStartTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	giveUptime := 5 * time.Second

	m := newMockstatusGetter(t)
	m.EXPECT().GetState().Return(giveState).Once()
	m.EXPECT().GetUptime().Return(giveUptime).Once()
	m.EXPECT().GetStartTime().Return(giveStartTime).Once()
	m.EXPECT().GetAllStats().Return(nil).Maybe()

	handler := HandleStatus(logger, m)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/-/status", http.NoBody)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("want Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	var body struct {
		State     string  `json:"state"`
		Uptime    string  `json:"uptime"`
		StartTime string  `json:"startTime"`
		UptimeSec float64 `json:"uptimeSeconds"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.State != string(giveState) {
		t.Errorf("want state %q, got %q", giveState, body.State)
	}

	if body.Uptime != giveUptime.String() {
		t.Errorf("want uptime %q, got %q", giveUptime, body.Uptime)
	}

	if body.UptimeSec != giveUptime.Seconds() {
		t.Errorf("want uptimeSeconds %f, got %f", giveUptime.Seconds(), body.UptimeSec)
	}
}
