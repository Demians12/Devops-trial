package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNormalizeStartDate_PastBecomesTodayUTC(t *testing.T) {
	now := time.Date(2025, 12, 23, 10, 0, 0, 0, time.UTC)
	requested := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	got := normalizeStartDate(requested, now)
	want := time.Date(2025, 12, 23, 0, 0, 0, 0, time.UTC)

	if !got.Equal(want) {
		t.Fatalf("normalizeStartDate: got=%s want=%s", got, want)
	}
}

func TestAvailableScheduleHandler_Success200_AndMetricsRecorded(t *testing.T) {
	metrics := newMetricsStore([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 5})
	app := &server{
		serviceName: "available-schedules",
		errorRate:   0, // deterministic success
		extraDelay:  0,
		metrics:     metrics,
	}

	route := "/v2/appoints/available-schedule"
	handler := app.instrument(route, app.handleAvailableSchedule)

	req := httptest.NewRequest("GET", "http://example.com"+route+"?days=15", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp availableScheduleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v body=%s", err, rr.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got false")
	}
	if len(resp.Data) < 15 || len(resp.Data) > 30 {
		t.Fatalf("expected data length between 15 and 30, got %d", len(resp.Data))
	}

	// Validate metrics were recorded for route + status
	app.metrics.mu.Lock()
	defer app.metrics.mu.Unlock()

	if _, ok := app.metrics.counts[route]; !ok {
		t.Fatalf("metrics missing route=%s", route)
	}
	if got := app.metrics.counts[route][http.StatusOK]; got != 1 {
		t.Fatalf("expected metrics count for 200 to be 1, got %.0f", got)
	}
}

func TestAvailableScheduleHandler_Error500_AndMetricsRecorded(t *testing.T) {
	metrics := newMetricsStore([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 5})
	app := &server{
		serviceName: "available-schedules",
		errorRate:   1, // deterministic failure
		extraDelay:  0,
		metrics:     metrics,
	}

	route := "/v2/appoints/available-schedule"
	handler := app.instrument(route, app.handleAvailableSchedule)

	req := httptest.NewRequest("GET", "http://example.com"+route+"?days=15", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rr.Code, rr.Body.String())
	}

	// Validate metrics were recorded for route + status 500
	app.metrics.mu.Lock()
	defer app.metrics.mu.Unlock()

	if _, ok := app.metrics.counts[route]; !ok {
		t.Fatalf("metrics missing route=%s", route)
	}
	if got := app.metrics.counts[route][http.StatusInternalServerError]; got != 1 {
		t.Fatalf("expected metrics count for 500 to be 1, got %.0f", got)
	}
}
