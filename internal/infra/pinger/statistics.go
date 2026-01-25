package pinger

import (
	"slices"
	"sort"
	"sync"
	"time"
)

const (
	// SuccessLatencyBufferSize is the number of successful ping latencies to track
	SuccessLatencyBufferSize = 100

	// ErrorLatencyBufferSize is the number of error ping latencies to track
	ErrorLatencyBufferSize = 10

	// PercentileMax is the maximum percentile value (100%)
	PercentileMax = 100.0

	// PercentileP99Threshold is the threshold for P99 percentile calculation
	PercentileP99Threshold = 99.0

	// MedianDivisor is used for calculating median index
	MedianDivisor = 2

	// PercentileP80 is the 80th percentile
	PercentileP80 = 80.0

	// PercentileP90 is the 90th percentile
	PercentileP90 = 90.0

	// PercentileP99 is the 99th percentile
	PercentileP99 = 99.0
)

// ErrorSnapshot represents a snapshot of an error occurrence
type ErrorSnapshot struct {
	Timestamp time.Time
	Latency   time.Duration
	Error     error
}

// LatencyBuffer is a circular buffer for storing time.Duration values
type LatencyBuffer struct {
	mu       sync.RWMutex
	buffer   []time.Duration
	capacity int
	index    int
	count    int
}

// NewLatencyBuffer creates a new latency buffer with the specified capacity
func NewLatencyBuffer(capacity int) *LatencyBuffer {
	return &LatencyBuffer{
		buffer:   make([]time.Duration, 0, capacity),
		capacity: capacity,
	}
}

// Add adds a duration to the buffer
func (lb *LatencyBuffer) Add(d time.Duration) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.count < lb.capacity {
		lb.buffer = append(lb.buffer, d)
		lb.count++
	} else {
		lb.buffer[lb.index] = d
		lb.index = (lb.index + 1) % lb.capacity
	}
}

// GetAll returns a copy of all durations in the buffer
func (lb *LatencyBuffer) GetAll() []time.Duration {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if lb.count == 0 {
		return nil
	}

	result := make([]time.Duration, lb.count)
	if lb.count < lb.capacity {
		copy(result, lb.buffer)
	} else {
		// Copy from index to end, then from start to index
		copy(result, lb.buffer[lb.index:])
		copy(result[lb.capacity-lb.index:], lb.buffer[:lb.index])
	}

	return result
}

// Len returns the number of durations in the buffer
func (lb *LatencyBuffer) Len() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	return lb.count
}

// Stats tracks statistics for a single pinger
type Stats struct {
	Name              string
	LastRun           time.Time
	LastError         error
	LastErrorSnapshot *ErrorSnapshot
	SuccessLatencies  *LatencyBuffer
	ErrorLatencies    *LatencyBuffer
	mu                sync.RWMutex
}

// NewPingerStats creates a new PingerStats instance
func NewPingerStats(name string) *Stats {
	return &Stats{
		Name:             name,
		SuccessLatencies: NewLatencyBuffer(SuccessLatencyBufferSize),
		ErrorLatencies:   NewLatencyBuffer(ErrorLatencyBufferSize),
	}
}

// LatencyMetrics contains calculated latency statistics
type LatencyMetrics struct {
	Count   int
	Median  time.Duration
	Average time.Duration
	P80     time.Duration
	P90     time.Duration
	P99     time.Duration
}

// Statistics contains computed statistics for a pinger
type Statistics struct {
	IsReady           bool
	IsHealthy         bool
	LastRun           time.Time
	LastError         error
	LastErrorSnapshot *ErrorSnapshot
	SuccessCount      int
	ErrorCount        int
	SuccessLatencies  LatencyMetrics
	ErrorLatencies    LatencyMetrics
}

// CalculatePercentile calculates the percentile value from a sorted slice of durations
// Uses floor method for most percentiles, with special handling for very high percentiles
func CalculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	if percentile < 0 {
		percentile = 0
	}

	if percentile >= PercentileMax {
		return latencies[len(latencies)-1]
	}

	// For percentiles < 100, use floor: index = floor((n-1) * p / 100)
	// Special case: for p99, round up to get the maximum value
	indexFloat := float64(len(latencies)-1) * percentile / PercentileMax
	index := int(indexFloat)

	// Round up for p99 to get the last element
	if percentile >= PercentileP99Threshold {
		index = len(latencies) - 1
	}

	if index < 0 {
		index = 0
	}

	if index >= len(latencies) {
		index = len(latencies) - 1
	}

	return latencies[index]
}

// CalculateMedian calculates the median value from a sorted slice of durations
func CalculateMedian(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	mid := len(sorted) / MedianDivisor
	if len(sorted)%MedianDivisor == 0 {
		return (sorted[mid-1] + sorted[mid]) / MedianDivisor
	}

	return sorted[mid]
}

// CalculateAverage calculates the average value from a slice of durations
func CalculateAverage(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range latencies {
		sum += d
	}

	return sum / time.Duration(len(latencies))
}

// calculateLatencyMetrics calculates latency metrics from a slice of durations
func calculateLatencyMetrics(latencies []time.Duration) LatencyMetrics {
	if len(latencies) == 0 {
		return LatencyMetrics{}
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	slices.Sort(sorted)

	return LatencyMetrics{
		Count:   len(sorted),
		Median:  CalculateMedian(sorted),
		Average: CalculateAverage(sorted),
		P80:     CalculatePercentile(sorted, PercentileP80),
		P90:     CalculatePercentile(sorted, PercentileP90),
		P99:     CalculatePercentile(sorted, PercentileP99),
	}
}

// GetStatistics computes and returns statistics from PingerStats
func GetStatistics(stats *Stats, info *pingerInfo) *Statistics {
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	successLatencies := stats.SuccessLatencies.GetAll()
	errorLatencies := stats.ErrorLatencies.GetAll()

	var lastErrorSnapshot *ErrorSnapshot
	if stats.LastErrorSnapshot != nil {
		lastErrorSnapshot = &ErrorSnapshot{
			Timestamp: stats.LastErrorSnapshot.Timestamp,
			Latency:   stats.LastErrorSnapshot.Latency,
			Error:     stats.LastErrorSnapshot.Error,
		}
	}

	// Calculate IsReady: if readyCritical is false, always true; otherwise based on LastError
	isReady := !info.readyCritical || stats.LastError == nil

	// Calculate IsHealthy: if healthCritical is false, always true; otherwise based on LastError
	isHealthy := !info.healthCritical || stats.LastError == nil

	return &Statistics{
		IsReady:           isReady,
		IsHealthy:         isHealthy,
		LastRun:           stats.LastRun,
		LastError:         stats.LastError,
		LastErrorSnapshot: lastErrorSnapshot,
		SuccessCount:      len(successLatencies),
		ErrorCount:        len(errorLatencies),
		SuccessLatencies:  calculateLatencyMetrics(successLatencies),
		ErrorLatencies:    calculateLatencyMetrics(errorLatencies),
	}
}
