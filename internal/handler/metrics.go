package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type metricKey struct {
	method string
	path   string
	status int
}

type metricsCollector struct {
	mu          sync.Mutex
	total       int64
	statusCount map[int]int64
	pathCount   map[metricKey]int64
	pathLatency map[metricKey][]float64
}

var metrics = &metricsCollector{
	statusCount: make(map[int]int64),
	pathCount:   make(map[metricKey]int64),
	pathLatency: make(map[metricKey][]float64),
}

func recordMetric(method, path string, status int, dur time.Duration) {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.total++
	metrics.statusCount[status]++

	key := metricKey{method: method, path: normalizePath(path), status: status}
	metrics.pathCount[key]++
	metrics.pathLatency[key] = append(metrics.pathLatency[key], float64(dur.Milliseconds()))
}

func normalizePath(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	for i, p := range parts {
		// Replace UUID-like and job-id segments with a placeholder
		if len(p) == 36 && strings.Count(p, "-") == 4 {
			parts[i] = ":id"
		}
		if len(p) == 32 {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests\n")
	fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
	fmt.Fprintf(w, "http_requests_total %d\n", metrics.total)

	fmt.Fprintf(w, "\n# HELP http_requests_by_status HTTP requests by status code\n")
	fmt.Fprintf(w, "# TYPE http_requests_by_status counter\n")
	for _, st := range sortedStatusCodes(metrics.statusCount) {
		fmt.Fprintf(w, "http_requests_by_status{status=\"%d\"} %d\n", st, metrics.statusCount[st])
	}

	fmt.Fprintf(w, "\n# HELP http_requests_by_path HTTP requests by method, path, status\n")
	fmt.Fprintf(w, "# TYPE http_requests_by_path counter\n")
	keys := sortedMetricKeys(metrics.pathCount)
	for _, k := range keys {
		fmt.Fprintf(w, "http_requests_by_path{method=\"%s\",path=\"%s\",status=\"%d\"} %d\n",
			k.method, k.path, k.status, metrics.pathCount[k])
	}
}

func sortedStatusCodes(m map[int]int64) []int {
	var codes []int
	for c := range m {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	return codes
}

func sortedMetricKeys(m map[metricKey]int64) []metricKey {
	var keys []metricKey
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		if keys[i].path != keys[j].path {
			return keys[i].path < keys[j].path
		}
		return keys[i].status < keys[j].status
	})
	return keys
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &metricsResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		recordMetric(r.Method, r.URL.Path, rw.statusCode, time.Since(start))
	})
}
