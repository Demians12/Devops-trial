package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
			fmt.Fprintf(
				w,
				`http_requests_total{route=%q,status=%q} %.0f`+"\n",
				route,
				strconv.Itoa(status),
				value,
			)
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

func parseFloatEnv(key string, fallback float64) float64 {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func parseIntEnv(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
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

func normalizeStartDate(requested time.Time, now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if requested.Before(today) {
		return today
	}
	return time.Date(requested.Year(), requested.Month(), requested.Day(), 0, 0, 0, 0, time.UTC)
}

type specialty struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type professional struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Specialty specialty `json:"specialty"`
}

type unitInfo struct {
	ID   int                    `json:"id"`
	Name string                 `json:"name"`
	Room map[string]interface{} `json:"room"`
}

var professionals = []professional{
	{ID: 2684, Name: "Dr(a). Pat Duarte", Specialty: specialty{ID: 55, Name: "Cardiologia"}},
	{ID: 512, Name: "Dr. Ícaro Menezes", Specialty: specialty{ID: 77, Name: "Dermatologia"}},
	{ID: 782, Name: "Dr(a). Helena Faria", Specialty: specialty{ID: 33, Name: "Pediatria"}},
	{ID: 903, Name: "Dr. André Ribeiro", Specialty: specialty{ID: 18, Name: "Ortopedia"}},
}

var units = []unitInfo{
	{ID: 901, Name: "Clínica Central", Room: map[string]interface{}{"id": 12, "name": "Sala Azul"}},
	{ID: 905, Name: "Unidade Bela Vista", Room: map[string]interface{}{"id": 203, "name": "Consultório 3"}},
	{ID: 910, Name: "Centro Norte", Room: map[string]interface{}{"id": 21, "name": "Sala Verde"}},
	{ID: 915, Name: "Hub Telemedicina", Room: map[string]interface{}{"id": 7, "name": "Estúdio 1"}},
}

func alignToHalfHour(t time.Time) time.Time {
	minute := t.Minute()
	remainder := minute % 30
	if remainder != 0 {
		t = t.Add(time.Duration(30-remainder) * time.Minute)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

func resolveProfessional(id int) professional {
	for _, p := range professionals {
		if p.ID == id {
			return p
		}
	}
	return professionals[0]
}

func resolveUnit(id int) unitInfo {
	for _, u := range units {
		if u.ID == id {
			return u
		}
	}
	return units[0]
}

func buildSchedule(professionalID, unitID int, days int, startDate time.Time) []schedulePayload {
	now := time.Now().UTC()
	if days < 15 {
		days = 15
	}
	if days > 30 {
		days = 30
	}

	prof := resolveProfessional(professionalID)
	unit := resolveUnit(unitID)

	baseDate := normalizeStartDate(startDate, now)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	response := make([]schedulePayload, 0, days)
	added := 0
	offset := 0

	for added < days {
		currentDate := baseDate.AddDate(0, 0, offset)
		offset++

		var start time.Time
		if currentDate.Equal(today) {
			start = alignToHalfHour(now.Add(15 * time.Minute))
		} else {
			start = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 8, 0, 0, 0, time.UTC)
		}

		limit := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 18, 0, 0, 0, time.UTC)
		if start.After(limit) {
			continue
		}

		slots := make([]scheduleSlot, 0, 8)
		for len(slots) < 8 && !start.After(limit) {
			slots = append(slots, scheduleSlot{
				Start:     start.Format("15:04"),
				Available: rand.Intn(10) > 1,
			})
			start = start.Add(30 * time.Minute)
		}

		response = append(response, schedulePayload{
			Professional: map[string]interface{}{"id": prof.ID, "name": prof.Name},
			Unit:         map[string]interface{}{"id": unit.ID, "name": unit.Name},
			Room:         unit.Room,
			Specialty:    map[string]interface{}{"id": prof.Specialty.ID, "name": prof.Specialty.Name},
			Date:         currentDate.Format("2006-01-02"),
			Slots:        slots,
		})
		added++
	}

	return response
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

		if s.extraDelay > 0 {
			time.Sleep(s.extraDelay)
		}

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		if rand.Float64() < s.errorRate {
			sw.WriteHeader(http.StatusInternalServerError)
			sw.Header().Set("Content-Type", "application/json")
			_, _ = sw.Write([]byte(`{"error":"transient error retrieving schedule"}`))
		} else {
			next.ServeHTTP(sw, r)
			if sw.status == 0 {
				sw.status = http.StatusOK
			}
		}

		span := trace.SpanFromContext(r.Context())
		sc := span.SpanContext()
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

func (s *server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	payload := map[string]interface{}{
		"service": s.serviceName,
		"status":  "ok",
		"endpoints": []string{
			"/v2/appoints/available-schedule",
			"/v2/healthz",
			"/v2/metrics",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *server) handleAvailableSchedule(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	professionalID, _ := strconv.Atoi(query.Get("professional_id"))
	if professionalID == 0 {
		professionalID = 4102
	}
	unitID, _ := strconv.Atoi(query.Get("unit_id"))
	if unitID == 0 {
		unitID = 108
	}

	daysRequested, err := strconv.Atoi(query.Get("days"))
	if err != nil || daysRequested <= 0 {
		daysRequested = 15
	}
	days := daysRequested
	if days < 15 {
		days = 15
	}
	if days > 30 {
		days = 30
	}

	startDateRequested := query.Get("start_date")
	now := time.Now().UTC()
	startDate := now
	if startDateRequested != "" {
		if parsed, err := time.Parse("2006-01-02", startDateRequested); err == nil {
			startDate = normalizeStartDate(parsed, now)
		}
	}

	resp := availableScheduleResponse{
		Success: true,
		Filters: map[string]interface{}{
			"generated_at":         time.Now().UTC().Format(time.RFC3339),
			"professional_id":      professionalID,
			"unit_id":              unitID,
			"days_requested":       daysRequested,
			"days_returned":        days,
			"start_date_requested": startDateRequested,
			"start_date_applied":   startDate.Format("2006-01-02"),
		},
		Data: buildSchedule(professionalID, unitID, days, startDate),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func initTracer(ctx context.Context, serviceName string) (*sdktrace.TracerProvider, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = "http://otel-collector.observability.svc.cluster.local:4318/v1/traces"
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_ENDPOINT: %w", err)
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(u.Host),
		otlptracehttp.WithURLPath(u.EscapedPath()),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func main() {
	rand.NewSource(time.Now().UnixNano())
	ctx := context.Background()

	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "available-schedules"
	}
	env := strings.TrimSpace(os.Getenv("ENV"))
	if env == "" {
		env = "dev"
	}
	version := strings.TrimSpace(os.Getenv("VERSION"))
	if version == "" {
		version = "unknown"
	}

	tp, err := initTracer(ctx, serviceName)
	if err != nil {
		log.Fatalf("init tracer: %v", err)
	}
	defer func() { _ = tp.Shutdown(ctx) }()

	errorRate := parseFloatEnv("ERROR_RATE", 0.01)
	if errorRate > 1 {
		errorRate = 1
	}
	extraLatency := time.Duration(parseIntEnv("EXTRA_LATENCY_MS", 0)) * time.Millisecond

	metrics := newMetricsStore([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 5})
	app := &server{
		serviceName: serviceName,
		env:         env,
		version:     version,
		errorRate:   errorRate,
		extraDelay:  extraLatency,
		metrics:     metrics,
	}

	http.HandleFunc("/", app.handleRoot)
	http.HandleFunc("/v2", app.handleRoot)
	http.HandleFunc("/v2/healthz", app.handleHealth)
	http.HandleFunc("/healthz", app.handleHealth)
	http.HandleFunc("/v2/metrics", func(w http.ResponseWriter, r *http.Request) { app.metrics.writePrometheus(w) })
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) { app.metrics.writePrometheus(w) })

	base := http.HandlerFunc(app.handleAvailableSchedule)

	traced := otelhttp.NewHandler(base, "GET /v2/appoints/available-schedule")

	instrumented := app.instrument("/v2/appoints/available-schedule", traced)

	http.Handle("/v2/appoints/available-schedule", instrumented)

	http.Handle("/appoints/available-schedule", app.instrument("/appoints/available-schedule", traced))

	addr := ":8080"
	log.Printf("starting %s listening on %s", serviceName, addr)
	if err := http.ListenAndServe(addr, nil); err != nil && !strings.Contains(err.Error(), "Server closed") {
		log.Fatalf("server error: %v", err)
	}
}
