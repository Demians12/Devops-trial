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
		bucketCounts: make([]float64, len(buckets)+1), // +Inf bucket
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
		fmt.Fprintf(
			w,
			`http_request_duration_seconds_bucket{le="%g"} %.0f`+"\n",
			boundary,
			cumulative,
		)
	}
	// +Inf bucket
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
	if err != nil {
		return fallback
	}
	if parsed < 0 {
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
	if err != nil {
		return fallback
	}
	if parsed < 0 {
		return fallback
	}
	return parsed
}

type scheduleSlot struct {
	Start     string `json:"start"`
	End       string `json:"end"`
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

func buildSchedule(professionalID, unitID int, days int) []schedulePayload {
	base := time.Now().UTC()
	response := make([]schedulePayload, 0, days)
	for i := 0; i < days; i++ {
		current := base.Add(time.Duration(i) * 24 * time.Hour)
		slots := make([]scheduleSlot, 0, 6)
		for step := 0; step < 6; step++ {
			start := current.Add(time.Hour*9 + time.Minute*time.Duration(30*step))
			end := start.Add(30 * time.Minute)
			slots = append(slots, scheduleSlot{
				Start:     start.Format("15:04"),
				End:       end.Format("15:04"),
				Available: rand.Intn(10) > 1,
			})
		}
		payload := schedulePayload{
			Professional: map[string]interface{}{
				"id":   professionalID,
				"name": "Dr(a). Júlia Fontes",
			},
			Unit: map[string]interface{}{
				"id":   unitID,
				"name": "Unidade Bela Vista",
			},
			Room: map[string]interface{}{
				"id":   203,
				"name": "Consultório 3",
			},
			Specialty: map[string]interface{}{
				"id":   77,
				"name": "Dermatologia",
			},
			Date:  current.Format("2006-01-02"),
			Slots: slots,
		}
		response = append(response, payload)
	}
	return response
}

type server struct {
	serviceName string
	errorRate   float64
	extraDelay  time.Duration
	metrics     *metricsStore
}

func (s *server) instrument(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		if s.extraDelay > 0 {
			time.Sleep(s.extraDelay)
		}

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), "route", route)

		if rand.Float64() < s.errorRate {
			sw.WriteHeader(http.StatusInternalServerError)
			sw.Header().Set("Content-Type", "application/json")
			_, _ = sw.Write([]byte(`{"error":"transient error retrieving schedule"}`))
			log.Printf(`{"service":"%s","route":"%s","status":%d,"latency_ms":%.2f,"note":"simulated failure"}`,
				s.serviceName, route, sw.status, float64(time.Since(start))/float64(time.Millisecond))
		} else {
			next(sw, r.WithContext(ctx))
			if sw.status == 0 {
				sw.status = http.StatusOK
			}
			log.Printf(`{"service":"%s","route":"%s","status":%d,"latency_ms":%.2f}`,
				s.serviceName, route, sw.status, float64(time.Since(start))/float64(time.Millisecond))
		}

		duration := time.Since(start).Seconds()
		statusCode := sw.status
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		s.metrics.observe(route, statusCode, duration)
	}
}

func (s *server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	payload := map[string]interface{}{
		"service": s.serviceName,
		"status":  "ok",
		"endpoints": []string{
			"/go/appoints/available-schedule",
			"/go/healthz",
			"/go/metrics",
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
	days, _ := strconv.Atoi(query.Get("days"))
	if days <= 0 || days > 7 {
		days = 3
	}

	response := availableScheduleResponse{
		Success: true,
		Filters: map[string]interface{}{
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
			"professional_id": professionalID,
			"unit_id":         unitID,
			"days":            days,
		},
		Data: buildSchedule(professionalID, unitID, days),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "available-schedules"
	}

	errorRate := parseFloatEnv("ERROR_RATE", 0.01)
	if errorRate > 1 {
		errorRate = 1
	}

	extraLatency := time.Duration(parseIntEnv("EXTRA_LATENCY_MS", 0)) * time.Millisecond

	metrics := newMetricsStore([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 5})
	app := &server{
		serviceName: serviceName,
		errorRate:   errorRate,
		extraDelay:  extraLatency,
		metrics:     metrics,
	}

	// Root handlers
	http.HandleFunc("/", app.handleRoot)
	http.HandleFunc("/go", app.handleRoot)
	http.HandleFunc("/go/healthz", app.handleHealth)
	http.HandleFunc("/healthz", app.handleHealth)

	// Metrics endpoint shared between prefixes
	http.HandleFunc("/go/metrics", func(w http.ResponseWriter, r *http.Request) {
		app.metrics.writePrometheus(w)
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		app.metrics.writePrometheus(w)
	})

	// Schedule handlers
	http.HandleFunc("/go/appoints/available-schedule", app.instrument("/go/appoints/available-schedule", app.handleAvailableSchedule))

	// Friendly alias if the service is accessed directly (without ingress prefix)
	http.HandleFunc("/appoints/available-schedule", app.instrument("/appoints/available-schedule", app.handleAvailableSchedule))

	addr := ":8080"
	log.Printf("starting %s listening on %s", serviceName, addr)
	if err := http.ListenAndServe(addr, nil); err != nil && !strings.Contains(err.Error(), "Server closed") {
		log.Fatalf("server error: %v", err)
	}
}
