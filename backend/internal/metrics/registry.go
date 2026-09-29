package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type Registry struct {
	mu sync.RWMutex

	httpRequests  map[httpRequestKey]uint64
	httpDurations map[httpDurationKey]*histogram
	llmOperations map[labelPair]uint64
	moves         map[moveKey]uint64
	scoreChanges  map[labelPair]uint64
	sessionsStart map[string]uint64
	sessionsEnd   map[labelPair]uint64
}

type httpRequestKey struct {
	Method string
	Route  string
	Status int
}

type httpDurationKey struct {
	Method string
	Route  string
}

type labelPair struct {
	First  string
	Second string
}

type moveKey struct {
	Intent      string
	Technique   string
	ReplySource string
}

type histogram struct {
	Buckets []uint64
	Count   uint64
	Sum     float64
}

func NewRegistry() *Registry {
	return &Registry{
		httpRequests:  make(map[httpRequestKey]uint64),
		httpDurations: make(map[httpDurationKey]*histogram),
		llmOperations: make(map[labelPair]uint64),
		moves:         make(map[moveKey]uint64),
		scoreChanges:  make(map[labelPair]uint64),
		sessionsStart: make(map[string]uint64),
		sessionsEnd:   make(map[labelPair]uint64),
	}
}

func (registry *Registry) ObserveHTTP(method, route string, status int, duration time.Duration) {
	if registry == nil {
		return
	}
	method, route = stableLabel(method, "UNKNOWN"), stableLabel(route, "unmatched")
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.httpRequests[httpRequestKey{Method: method, Route: route, Status: status}]++
	key := httpDurationKey{Method: method, Route: route}
	item := registry.httpDurations[key]
	if item == nil {
		item = &histogram{Buckets: make([]uint64, len(durationBuckets))}
		registry.httpDurations[key] = item
	}
	seconds := duration.Seconds()
	item.Count++
	item.Sum += seconds
	for index, upperBound := range durationBuckets {
		if seconds <= upperBound {
			item.Buckets[index]++
		}
	}
}

func (registry *Registry) ObserveLLM(operation, result string) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	registry.llmOperations[labelPair{stableLabel(operation, "unknown"), stableLabel(result, "unknown")}]++
	registry.mu.Unlock()
}

func (registry *Registry) ObserveMove(intent, technique, replySource string, trustDelta, argumentDelta, pressureDelta int) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.moves[moveKey{
		Intent: stableLabel(intent, "unknown"), Technique: stableLabel(technique, "unknown"),
		ReplySource: stableLabel(replySource, "unknown"),
	}]++
	registry.addScoreChange("trust", trustDelta)
	registry.addScoreChange("argument", argumentDelta)
	registry.addScoreChange("pressure", pressureDelta)
}

func (registry *Registry) addScoreChange(attribute string, delta int) {
	if delta == 0 {
		return
	}
	direction := "increase"
	if delta < 0 {
		direction = "decrease"
		delta = -delta
	}
	registry.scoreChanges[labelPair{attribute, direction}] += uint64(delta)
}

func (registry *Registry) ObserveSessionStarted(scenarioID string) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	registry.sessionsStart[stableLabel(scenarioID, "unknown")]++
	registry.mu.Unlock()
}

func (registry *Registry) ObserveSessionCompleted(status, outcome string) {
	if registry == nil {
		return
	}
	registry.mu.Lock()
	registry.sessionsEnd[labelPair{stableLabel(status, "unknown"), stableLabel(outcome, "unknown")}]++
	registry.mu.Unlock()
}

func (registry *Registry) ServeHTTP(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	writeHTTPMetrics(writer, registry.httpRequests, registry.httpDurations)
	writePairCounter(writer, "arena_llm_operations_total", "LLM operations by operation and result.", "operation", "result", registry.llmOperations)
	writeMoveCounter(writer, registry.moves)
	writePairCounter(writer, "arena_score_change_points_total", "Absolute score point changes by attribute and direction.", "attribute", "direction", registry.scoreChanges)
	writeSingleCounter(writer, "arena_sessions_started_total", "Negotiation sessions started by scenario.", "scenario", registry.sessionsStart)
	writePairCounter(writer, "arena_sessions_completed_total", "Negotiation sessions completed by status and outcome.", "status", "outcome", registry.sessionsEnd)
}

func writeHTTPMetrics(writer io.Writer, requests map[httpRequestKey]uint64, durations map[httpDurationKey]*histogram) {
	_, _ = io.WriteString(writer, "# HELP arena_http_requests_total HTTP requests by method, route and status.\n# TYPE arena_http_requests_total counter\n")
	requestKeys := make([]httpRequestKey, 0, len(requests))
	for key := range requests {
		requestKeys = append(requestKeys, key)
	}
	sort.Slice(requestKeys, func(i, j int) bool {
		return fmt.Sprint(requestKeys[i]) < fmt.Sprint(requestKeys[j])
	})
	for _, key := range requestKeys {
		_, _ = fmt.Fprintf(writer, "arena_http_requests_total{method=%s,route=%s,status=%q} %d\n", quote(key.Method), quote(key.Route), strconv.Itoa(key.Status), requests[key])
	}

	_, _ = io.WriteString(writer, "# HELP arena_http_request_duration_seconds HTTP request duration.\n# TYPE arena_http_request_duration_seconds histogram\n")
	durationKeys := make([]httpDurationKey, 0, len(durations))
	for key := range durations {
		durationKeys = append(durationKeys, key)
	}
	sort.Slice(durationKeys, func(i, j int) bool {
		return fmt.Sprint(durationKeys[i]) < fmt.Sprint(durationKeys[j])
	})
	for _, key := range durationKeys {
		item := durations[key]
		for index, upperBound := range durationBuckets {
			_, _ = fmt.Fprintf(writer, "arena_http_request_duration_seconds_bucket{method=%s,route=%s,le=%q} %d\n", quote(key.Method), quote(key.Route), strconv.FormatFloat(upperBound, 'g', -1, 64), item.Buckets[index])
		}
		_, _ = fmt.Fprintf(writer, "arena_http_request_duration_seconds_bucket{method=%s,route=%s,le=\"+Inf\"} %d\n", quote(key.Method), quote(key.Route), item.Count)
		_, _ = fmt.Fprintf(writer, "arena_http_request_duration_seconds_sum{method=%s,route=%s} %s\n", quote(key.Method), quote(key.Route), strconv.FormatFloat(item.Sum, 'g', -1, 64))
		_, _ = fmt.Fprintf(writer, "arena_http_request_duration_seconds_count{method=%s,route=%s} %d\n", quote(key.Method), quote(key.Route), item.Count)
	}
}

func writePairCounter(writer io.Writer, name, help, firstLabel, secondLabel string, values map[labelPair]uint64) {
	_, _ = fmt.Fprintf(writer, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	keys := make([]labelPair, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, key := range keys {
		_, _ = fmt.Fprintf(writer, "%s{%s=%s,%s=%s} %d\n", name, firstLabel, quote(key.First), secondLabel, quote(key.Second), values[key])
	}
}

func writeSingleCounter(writer io.Writer, name, help, label string, values map[string]uint64) {
	_, _ = fmt.Fprintf(writer, "# HELP %s %s\n# TYPE %s counter\n", name, help, name)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		_, _ = fmt.Fprintf(writer, "%s{%s=%s} %d\n", name, label, quote(key), values[key])
	}
}

func writeMoveCounter(writer io.Writer, values map[moveKey]uint64) {
	const name = "arena_negotiation_moves_total"
	_, _ = io.WriteString(writer, "# HELP "+name+" Processed negotiation moves by intent, technique and reply source.\n# TYPE "+name+" counter\n")
	keys := make([]moveKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, key := range keys {
		_, _ = fmt.Fprintf(writer, "%s{intent=%s,technique=%s,reply_source=%s} %d\n", name, quote(key.Intent), quote(key.Technique), quote(key.ReplySource), values[key])
	}
}

func stableLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func quote(value string) string {
	return strconv.Quote(value)
}
