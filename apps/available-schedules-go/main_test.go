package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func setupTestHandler(app *server, route string) http.Handler {
	baseHandler := http.HandlerFunc(app.handleAvailableSchedule)
	instrumented := app.instrument(route, baseHandler)
	return otelhttp.NewHandler(instrumented, "TestOperation")
}

func TestAvailableScheduleHandler_Success200_AndMetricsRecorded(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	metrics := newMetricsStore([]float64{0.05, 0.1, 0.5})
	app := &server{
		serviceName: "available-schedules",
		env:         "test",
		version:     "1.0.0-test",
		errorRate:   0,
		metrics:     metrics,
	}

	route := "/v2/appoints/available-schedule"
	handler := setupTestHandler(app, route)

	req := httptest.NewRequest("GET", "http://example.com"+route, nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp availableScheduleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true, got false")
	}

	app.metrics.mu.Lock()
	defer app.metrics.mu.Unlock()
	if got := app.metrics.counts[route][http.StatusOK]; got != 1 {
		t.Errorf("expected metrics count 1, got %.0f", got)
	}
}

func TestAvailableScheduleHandler_Error500_AndMetricsRecorded(t *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	metrics := newMetricsStore([]float64{0.05, 0.1})
	app := &server{
		serviceName: "available-schedules",
		errorRate:   1.0,
		metrics:     metrics,
	}

	route := "/v2/appoints/available-schedule"

	emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	instrumented := app.instrument(route, emptyHandler)
	handler := otelhttp.NewHandler(instrumented, "TestErrorOperation")

	req := httptest.NewRequest("GET", "http://example.com"+route, nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	app.metrics.mu.Lock()
	defer app.metrics.mu.Unlock()
	if got := app.metrics.counts[route][http.StatusInternalServerError]; got != 1 {
		t.Errorf("expected error metrics count 1, got %.0f", got)
	}
}
