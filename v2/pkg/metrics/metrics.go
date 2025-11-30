// Package metrics provides Prometheus metrics for GoSQLPage v2.1.
package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Registry holds all metrics.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
	gauges   map[string]*Gauge
	histograms map[string]*Histogram
}

// Counter is a monotonically increasing metric.
type Counter struct {
	name   string
	help   string
	labels []string
	values map[string]float64
	mu     sync.Mutex
}

// Gauge is a metric that can go up and down.
type Gauge struct {
	name   string
	help   string
	labels []string
	values map[string]float64
	mu     sync.Mutex
}

// Histogram tracks value distributions.
type Histogram struct {
	name    string
	help    string
	labels  []string
	buckets []float64
	values  map[string]*histogramValue
	mu      sync.Mutex
}

type histogramValue struct {
	counts []uint64
	sum    float64
	count  uint64
}

// NewRegistry creates a new metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// Counter methods

// NewCounter creates a new counter.
func (r *Registry) NewCounter(name, help string, labels []string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	c := &Counter{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]float64),
	}
	r.counters[name] = c
	return c
}

// Inc increments the counter by 1.
func (c *Counter) Inc(labelValues ...string) {
	c.Add(1, labelValues...)
}

// Add adds the given value to the counter.
func (c *Counter) Add(v float64, labelValues ...string) {
	key := labelsToKey(labelValues)
	c.mu.Lock()
	c.values[key] += v
	c.mu.Unlock()
}

// Gauge methods

// NewGauge creates a new gauge.
func (r *Registry) NewGauge(name, help string, labels []string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	g := &Gauge{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]float64),
	}
	r.gauges[name] = g
	return g
}

// Set sets the gauge value.
func (g *Gauge) Set(v float64, labelValues ...string) {
	key := labelsToKey(labelValues)
	g.mu.Lock()
	g.values[key] = v
	g.mu.Unlock()
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc(labelValues ...string) {
	g.Add(1, labelValues...)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec(labelValues ...string) {
	g.Add(-1, labelValues...)
}

// Add adds the given value to the gauge.
func (g *Gauge) Add(v float64, labelValues ...string) {
	key := labelsToKey(labelValues)
	g.mu.Lock()
	g.values[key] += v
	g.mu.Unlock()
}

// Histogram methods

// NewHistogram creates a new histogram.
func (r *Registry) NewHistogram(name, help string, labels []string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}

	h := &Histogram{
		name:    name,
		help:    help,
		labels:  labels,
		buckets: buckets,
		values:  make(map[string]*histogramValue),
	}
	r.histograms[name] = h
	return h
}

// DefaultBuckets are the default histogram buckets (in seconds).
var DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// Observe records a value in the histogram.
func (h *Histogram) Observe(v float64, labelValues ...string) {
	key := labelsToKey(labelValues)
	h.mu.Lock()
	defer h.mu.Unlock()

	hv, ok := h.values[key]
	if !ok {
		hv = &histogramValue{counts: make([]uint64, len(h.buckets))}
		h.values[key] = hv
	}

	for i, bucket := range h.buckets {
		if v <= bucket {
			hv.counts[i]++
		}
	}
	hv.sum += v
	hv.count++
}

// Timer is a helper for timing operations.
type Timer struct {
	histogram   *Histogram
	labelValues []string
	start       time.Time
}

// NewTimer starts a new timer.
func (h *Histogram) NewTimer(labelValues ...string) *Timer {
	return &Timer{
		histogram:   h,
		labelValues: labelValues,
		start:       time.Now(),
	}
}

// ObserveDuration records the elapsed time.
func (t *Timer) ObserveDuration() {
	t.histogram.Observe(time.Since(t.start).Seconds(), t.labelValues...)
}

// Helper functions

func labelsToKey(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	key := labels[0]
	for i := 1; i < len(labels); i++ {
		key += "," + labels[i]
	}
	return key
}

// Expose returns the metrics in Prometheus text format.
func (r *Registry) Expose() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var output string

	// Counters
	for _, c := range r.counters {
		output += fmt.Sprintf("# HELP %s %s\n", c.name, c.help)
		output += fmt.Sprintf("# TYPE %s counter\n", c.name)
		c.mu.Lock()
		for key, value := range c.values {
			labels := formatLabels(c.labels, key)
			output += fmt.Sprintf("%s%s %g\n", c.name, labels, value)
		}
		c.mu.Unlock()
	}

	// Gauges
	for _, g := range r.gauges {
		output += fmt.Sprintf("# HELP %s %s\n", g.name, g.help)
		output += fmt.Sprintf("# TYPE %s gauge\n", g.name)
		g.mu.Lock()
		for key, value := range g.values {
			labels := formatLabels(g.labels, key)
			output += fmt.Sprintf("%s%s %g\n", g.name, labels, value)
		}
		g.mu.Unlock()
	}

	// Histograms
	for _, h := range r.histograms {
		output += fmt.Sprintf("# HELP %s %s\n", h.name, h.help)
		output += fmt.Sprintf("# TYPE %s histogram\n", h.name)
		h.mu.Lock()
		for key, hv := range h.values {
			labels := formatLabels(h.labels, key)
			baseName := h.name
			if labels != "" {
				baseName = h.name + labels[:len(labels)-1] + ","
			} else {
				baseName = h.name + "{"
			}

			// Bucket counts
			cumulative := uint64(0)
			for i, bucket := range h.buckets {
				cumulative += hv.counts[i]
				output += fmt.Sprintf("%sle=\"%g\"} %d\n", baseName, bucket, cumulative)
			}
			output += fmt.Sprintf("%sle=\"+Inf\"} %d\n", baseName, hv.count)

			// Sum and count
			if labels != "" {
				output += fmt.Sprintf("%s_sum%s %g\n", h.name, labels, hv.sum)
				output += fmt.Sprintf("%s_count%s %d\n", h.name, labels, hv.count)
			} else {
				output += fmt.Sprintf("%s_sum %g\n", h.name, hv.sum)
				output += fmt.Sprintf("%s_count %d\n", h.name, hv.count)
			}
		}
		h.mu.Unlock()
	}

	return output
}

func formatLabels(names []string, key string) string {
	if len(names) == 0 || key == "" {
		return ""
	}

	values := splitKey(key)
	if len(values) != len(names) {
		return ""
	}

	result := "{"
	for i, name := range names {
		if i > 0 {
			result += ","
		}
		result += fmt.Sprintf("%s=\"%s\"", name, values[i])
	}
	result += "}"
	return result
}

func splitKey(key string) []string {
	if key == "" {
		return nil
	}
	var result []string
	current := ""
	for _, c := range key {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	result = append(result, current)
	return result
}

// Handler returns an HTTP handler for metrics.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(r.Expose()))
	})
}

// DefaultRegistry is the default metrics registry.
var DefaultRegistry = NewRegistry()

// GoSQLPage v2.1 Metrics

// RequestMetrics provides HTTP request metrics.
type RequestMetrics struct {
	Total    *Counter
	Duration *Histogram
	InFlight *Gauge
}

// NewRequestMetrics creates request metrics.
func NewRequestMetrics(r *Registry) *RequestMetrics {
	return &RequestMetrics{
		Total: r.NewCounter("gosqlpage_requests_total",
			"Total HTTP requests",
			[]string{"method", "path", "status"}),
		Duration: r.NewHistogram("gosqlpage_request_duration_seconds",
			"HTTP request duration in seconds",
			[]string{"method", "path"}, nil),
		InFlight: r.NewGauge("gosqlpage_requests_in_flight",
			"Current number of requests being processed",
			nil),
	}
}

// SessionMetrics provides session metrics.
type SessionMetrics struct {
	Active   *Gauge
	Created  *Counter
	Merged   *Counter
	Conflicts *Counter
}

// NewSessionMetrics creates session metrics.
func NewSessionMetrics(r *Registry) *SessionMetrics {
	return &SessionMetrics{
		Active: r.NewGauge("gosqlpage_sessions_active",
			"Number of active sessions", nil),
		Created: r.NewCounter("gosqlpage_sessions_created_total",
			"Total sessions created", nil),
		Merged: r.NewCounter("gosqlpage_sessions_merged_total",
			"Total sessions merged", nil),
		Conflicts: r.NewCounter("gosqlpage_sessions_conflicts_total",
			"Total session conflicts", nil),
	}
}

// MergerMetrics provides merger metrics.
type MergerMetrics struct {
	QueuePending    *Gauge
	QueueProcessing *Gauge
	QueueFailed     *Gauge
	MergesTotal     *Counter
	MergeDuration   *Histogram
}

// NewMergerMetrics creates merger metrics.
func NewMergerMetrics(r *Registry) *MergerMetrics {
	return &MergerMetrics{
		QueuePending: r.NewGauge("gosqlpage_merger_queue_pending",
			"Number of sessions pending merge", nil),
		QueueProcessing: r.NewGauge("gosqlpage_merger_queue_processing",
			"Number of sessions being merged", nil),
		QueueFailed: r.NewGauge("gosqlpage_merger_queue_failed",
			"Number of failed sessions", nil),
		MergesTotal: r.NewCounter("gosqlpage_merger_merges_total",
			"Total merge operations", []string{"status"}),
		MergeDuration: r.NewHistogram("gosqlpage_merger_duration_seconds",
			"Merge operation duration", nil, nil),
	}
}

// CacheMetrics provides cache metrics.
type CacheMetrics struct {
	Hits   *Counter
	Misses *Counter
	Size   *Gauge
	Evictions *Counter
}

// NewCacheMetrics creates cache metrics.
func NewCacheMetrics(r *Registry) *CacheMetrics {
	return &CacheMetrics{
		Hits: r.NewCounter("gosqlpage_cache_hits_total",
			"Cache hits", nil),
		Misses: r.NewCounter("gosqlpage_cache_misses_total",
			"Cache misses", nil),
		Size: r.NewGauge("gosqlpage_cache_size_bytes",
			"Current cache size in bytes", nil),
		Evictions: r.NewCounter("gosqlpage_cache_evictions_total",
			"Cache evictions", nil),
	}
}

// DatabaseMetrics provides database metrics.
type DatabaseMetrics struct {
	Size       *Gauge
	Queries    *Counter
	QueryDuration *Histogram
}

// NewDatabaseMetrics creates database metrics.
func NewDatabaseMetrics(r *Registry) *DatabaseMetrics {
	return &DatabaseMetrics{
		Size: r.NewGauge("gosqlpage_db_size_bytes",
			"Database size in bytes", []string{"db"}),
		Queries: r.NewCounter("gosqlpage_db_queries_total",
			"Database queries", []string{"db"}),
		QueryDuration: r.NewHistogram("gosqlpage_db_query_duration_seconds",
			"Database query duration", []string{"db"}, nil),
	}
}

// BotMetrics provides bot worker metrics.
type BotMetrics struct {
	RequestsProcessed *Counter
	RequestsFailed    *Counter
	ProcessingDuration *Histogram
}

// NewBotMetrics creates bot metrics.
func NewBotMetrics(r *Registry) *BotMetrics {
	return &BotMetrics{
		RequestsProcessed: r.NewCounter("gosqlpage_bot_requests_processed_total",
			"Bot requests processed", nil),
		RequestsFailed: r.NewCounter("gosqlpage_bot_requests_failed_total",
			"Bot requests failed", nil),
		ProcessingDuration: r.NewHistogram("gosqlpage_bot_processing_duration_seconds",
			"Bot request processing duration", nil, nil),
	}
}
