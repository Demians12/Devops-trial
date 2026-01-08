package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/stdr"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type metricsStore struct {
	mu           sync.Mutex
	counts       map[string]map[int]float64
	buckets      []float64
	bucketCounts []float64
	sum          float64
	count        float64
}

func newMetricsStore(buckets []float64) *metricsStore {
	return &metricsStore{
		counts:       make(map[string]map[int]float64),
		buckets:      buckets,
		bucketCounts: make([]float64, len(buckets)+1),
	}
}

func (m *metricsStore) observe(route string, status int, durationSeconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.counts[route]; !ok {
		m.counts[route] = make(map[int]float64)
	}
	m.counts[route][status]++

	m.sum += durationSeconds
	m.count++

	for i, boundary := range m.buckets {
		if durationSeconds <= boundary {
			m.bucketCounts[i]++
			return
		}
	}
	m.bucketCounts[len(m.bucketCounts)-1]++
}

func (m *metricsStore) writePrometheus(w http.ResponseWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# HELP http_requests_total Total HTTP requests")
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	for route, statuses := range m.counts {
		for status, value := range statuses {
			fmt.Fprintf(w, `http_requests_total{route=%q,status=%q} %.0f`+"\n", route, strconv.Itoa(status), value)
		}
	}

	fmt.Fprintln(w, "# HELP http_request_duration_seconds Request latency in seconds")
	fmt.Fprintln(w, "# TYPE http_request_duration_seconds histogram")
	cumulative := 0.0
	for i, boundary := range m.buckets {
		cumulative += m.bucketCounts[i]
		fmt.Fprintf(w, `http_request_duration_seconds_bucket{le="%g"} %.0f`+"\n", boundary, cumulative)
	}
	cumulative += m.bucketCounts[len(m.bucketCounts)-1]
	fmt.Fprintf(w, `http_request_duration_seconds_bucket{le="+Inf"} %.0f`+"\n", cumulative)
	fmt.Fprintf(w, "http_request_duration_seconds_sum %.6f\n", m.sum)
	fmt.Fprintf(w, "http_request_duration_seconds_count %.0f\n", m.count)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

type scheduleSlot struct {
	Start     string `json:"start"`
	Available bool   `json:"available"`
}

type schedulePayload struct {
	Professional map[string]interface{} `json:"professional"`
	Unit         map[string]interface{} `json:"unit"`
	Room         map[string]interface{} `json:"room"`
	Specialty    map[string]interface{} `json:"specialty"`
	Date         string                 `json:"date"`
	Slots        []scheduleSlot         `json:"slots"`
}

type availableScheduleResponse struct {
	Success bool                   `json:"success"`
	Filters map[string]interface{} `json:"filters"`
	Data    []schedulePayload      `json:"response"`
}

var professionals = []struct {
	ID        int
	Name      string
	Specialty string
}{
	{2684, "Dr(a). Pat Duarte", "Cardiologia"},
	{512, "Dr. Ícaro Menezes", "Dermatologia"},
}

func buildSchedule(profID, unitID, days int, start time.Time) []schedulePayload {
	return []schedulePayload{
		{
			Date:  start.Format("2006-01-02"),
			Slots: []scheduleSlot{{Start: "09:00", Available: true}},
		},
	}
}

type server struct {
	serviceName string
	env         string
	version     string
	errorRate   float64
	extraDelay  time.Duration
	metrics     *metricsStore
}

func (s *server) instrument(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		if rand.Float64() < s.errorRate {
			sw.WriteHeader(http.StatusInternalServerError)
			sw.Header().Set("Content-Type", "application/json")
			_, _ = sw.Write([]byte(`{"error":"transient error retrieving schedule"}`))
		}

		ctx := r.Context()
		span := trace.SpanFromContext(ctx)
		sc := span.SpanContext()
		log.Printf("IsValid: %v, TraceID: %s", sc.IsValid(), sc.TraceID().String())
		traceID := ""
		spanID := ""
		if sc.IsValid() {
			traceID = sc.TraceID().String()
			spanID = sc.SpanID().String()
		}

		latencyMs := float64(time.Since(start)) / float64(time.Millisecond)

		log.Printf(`{"service":"%s","env":"%s","version":"%s","route":"%s","method":"%s","status":%d,"latency_ms":%.2f,"trace_id":"%s","span_id":"%s"}`,
			s.serviceName, s.env, s.version, route, r.Method, sw.status, latencyMs, traceID, spanID)

		s.metrics.observe(route, sw.status, time.Since(start).Seconds())
	})
}

func (s *server) handleAvailableSchedule(w http.ResponseWriter, r *http.Request) {
	resp := availableScheduleResponse{Success: true, Data: buildSchedule(1, 1, 15, time.Now())}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func initTracer(ctx context.Context, serviceName string) (*sdktrace.TracerProvider, error) {

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "tempo.observability:4318"
	}

	cleanEndpoint := strings.TrimPrefix(endpoint, "http://")
	cleanEndpoint = strings.TrimPrefix(cleanEndpoint, "https://")
	cleanEndpoint = strings.Split(cleanEndpoint, "/")[0]

	log.Printf("Inicializando Tracer. Serviço: %s | Endpoint: %s", serviceName, cleanEndpoint)

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cleanEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar exportador: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(os.Getenv("VERSION")),
			semconv.DeploymentEnvironmentKey.String(os.Getenv("ENV")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar recurso: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Println("DEBUG: Tracer configurado com sucesso e registrado globalmente")
	return tp, nil
}
func main() {
	log.SetFlags(0)
	otel.SetLogger(stdr.New(log.New(os.Stderr, "OTEL_DIAGNOSTIC: ", log.LstdFlags)))

	ctx := context.Background()
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "available-schedules-go"
	}

	tp, err := initTracer(ctx, serviceName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	metrics := newMetricsStore([]float64{0.05, 0.1, 0.2, 0.5, 1.0})
	app := &server{
		serviceName: serviceName,
		env:         os.Getenv("ENV"),
		version:     os.Getenv("VERSION"),
		errorRate:   0.02,
		metrics:     metrics,
	}

	http.HandleFunc("/healthz", app.handleHealth)
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) { app.metrics.writePrometheus(w) })

	// Correlation
	baseHandler := http.HandlerFunc(app.handleAvailableSchedule)
	instrumentedHandler := app.instrument("/v2/appoints/available-schedule", baseHandler)

	finalHandler := otelhttp.NewHandler(instrumentedHandler, "AvailableSchedule")
	http.Handle("/v2/appoints/available-schedule", finalHandler)

	log.Printf("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
