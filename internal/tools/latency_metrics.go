package tools

import (
	"sort"
	"sync"
	"time"
)

var toolLatencyBucketsMS = []int64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

type toolLatencyState struct {
	Count       int64
	ErrorCount  int64
	TotalMS     int64
	MaxMS       int64
	InFlight    int64
	MaxInFlight int64
	Buckets     []int64
}

type ToolLatencyMetrics struct {
	Tool         string  `json:"tool"`
	Count        int64   `json:"count"`
	ErrorCount   int64   `json:"error_count"`
	ErrorRate    float64 `json:"error_rate"`
	AvgMS        float64 `json:"avg_ms"`
	P50MS        int64   `json:"p50_ms"`
	P95MS        int64   `json:"p95_ms"`
	MaxMS        int64   `json:"max_ms"`
	InFlight     int64   `json:"in_flight"`
	MaxInFlight  int64   `json:"max_in_flight"`
	BucketUpper  []int64 `json:"bucket_upper_ms"`
	BucketCounts []int64 `json:"bucket_counts"`
}

type toolLatencyCollector struct {
	mu    sync.RWMutex
	items map[string]*toolLatencyState
}

func newToolLatencyCollector() *toolLatencyCollector {
	return &toolLatencyCollector{
		items: make(map[string]*toolLatencyState),
	}
}

var globalToolLatencyCollector = newToolLatencyCollector()

func (c *toolLatencyCollector) begin(tool string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.items[tool]
	if st == nil {
		st = &toolLatencyState{Buckets: make([]int64, len(toolLatencyBucketsMS))}
		c.items[tool] = st
	}
	st.InFlight++
	if st.InFlight > st.MaxInFlight {
		st.MaxInFlight = st.InFlight
	}
}

func (c *toolLatencyCollector) done(tool string, duration time.Duration, isError bool) {
	ms := duration.Milliseconds()
	if ms < 0 {
		ms = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	st := c.items[tool]
	if st == nil {
		st = &toolLatencyState{Buckets: make([]int64, len(toolLatencyBucketsMS))}
		c.items[tool] = st
	}

	if st.InFlight > 0 {
		st.InFlight--
	}
	st.Count++
	if isError {
		st.ErrorCount++
	}
	st.TotalMS += ms
	if ms > st.MaxMS {
		st.MaxMS = ms
	}

	idx := len(toolLatencyBucketsMS) - 1
	for i, b := range toolLatencyBucketsMS {
		if ms <= b {
			idx = i
			break
		}
	}
	st.Buckets[idx]++
}

func (c *toolLatencyCollector) snapshot() []ToolLatencyMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]ToolLatencyMetrics, 0, len(c.items))
	for tool, st := range c.items {
		if st == nil || st.Count == 0 {
			continue
		}
		p50 := percentileFromBuckets(st.Buckets, 0.50)
		p95 := percentileFromBuckets(st.Buckets, 0.95)
		errRate := 0.0
		if st.Count > 0 {
			errRate = float64(st.ErrorCount) / float64(st.Count)
		}
		buckets := make([]int64, len(st.Buckets))
		copy(buckets, st.Buckets)
		upper := make([]int64, len(toolLatencyBucketsMS))
		copy(upper, toolLatencyBucketsMS)

		out = append(out, ToolLatencyMetrics{
			Tool:         tool,
			Count:        st.Count,
			ErrorCount:   st.ErrorCount,
			ErrorRate:    errRate,
			AvgMS:        float64(st.TotalMS) / float64(st.Count),
			P50MS:        p50,
			P95MS:        p95,
			MaxMS:        st.MaxMS,
			InFlight:     st.InFlight,
			MaxInFlight:  st.MaxInFlight,
			BucketUpper:  upper,
			BucketCounts: buckets,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].P95MS == out[j].P95MS {
			return out[i].Count > out[j].Count
		}
		return out[i].P95MS > out[j].P95MS
	})
	return out
}

func percentileFromBuckets(buckets []int64, p float64) int64 {
	var total int64
	for _, c := range buckets {
		total += c
	}
	if total == 0 {
		return 0
	}
	target := int64(float64(total)*p + 0.999999)
	if target < 1 {
		target = 1
	}
	var acc int64
	for i, c := range buckets {
		acc += c
		if acc >= target {
			return toolLatencyBucketsMS[i]
		}
	}
	return toolLatencyBucketsMS[len(toolLatencyBucketsMS)-1]
}

// RecordToolLatencyStart increments in-flight counters for a tool execution.
func RecordToolLatencyStart(tool string) {
	globalToolLatencyCollector.begin(tool)
}

// RecordToolLatencyDone finalizes a tool execution sample.
func RecordToolLatencyDone(tool string, duration time.Duration, isError bool) {
	globalToolLatencyCollector.done(tool, duration, isError)
}

// SnapshotToolLatencyMetrics returns aggregated histogram-like latency metrics per tool.
func SnapshotToolLatencyMetrics() []ToolLatencyMetrics {
	return globalToolLatencyCollector.snapshot()
}
