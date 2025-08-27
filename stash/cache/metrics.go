package cache

import (
	"sync/atomic"
)

// Metrics tracks cache performance statistics.
type Metrics struct {
	// Cache tier counters
	L0Hits   *Counter // In-memory cache hits
	L1Hits   *Counter // Disk cache hits
	L2Misses *Counter // Cache misses

	// Bytes served per tier
	L0BytesServed  *Counter
	L1BytesServed  *Counter
	L2BytesFetched *Counter

	// Operation counters
	TotalReads *Counter

	// Gauges (updated periodically)
	DiskCacheUsageMB   *Gauge
	DiskCacheUsagePct  *Gauge
	MemCacheUsageMB    *Gauge
	MemCacheUsagePct   *Gauge
	L0HitRatio         *Gauge
	L1HitRatio         *Gauge
	L2MissRatio        *Gauge
	ReadThroughputMBps *Gauge

	// FUSE operation latencies (for future use)
	FUSEReadLatency    *Gauge
	FUSELookupLatency  *Gauge
	FUSEGetattrLatency *Gauge
}

// Counter is a monotonically increasing counter.
type Counter struct {
	value atomic.Int64
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.value.Add(1)
}

// Add increments the counter by delta.
func (c *Counter) Add(delta int) {
	c.value.Add(int64(delta))
}

// Get returns the current counter value.
func (c *Counter) Get() int64 {
	return c.value.Load()
}

// Gauge is a value that can go up or down.
type Gauge struct {
	value atomic.Uint64 // Store as bits to allow atomic float64 operations
}

// Update sets the gauge to the specified value.
func (g *Gauge) Update(value float64) {
	g.value.Store(uint64(value * 100)) // Store as centiprecision integer
}

// Get returns the current gauge value.
func (g *Gauge) Get() float64 {
	return float64(g.value.Load()) / 100
}

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		L0Hits:             &Counter{},
		L1Hits:             &Counter{},
		L2Misses:           &Counter{},
		L0BytesServed:      &Counter{},
		L1BytesServed:      &Counter{},
		L2BytesFetched:     &Counter{},
		TotalReads:         &Counter{},
		DiskCacheUsageMB:   &Gauge{},
		DiskCacheUsagePct:  &Gauge{},
		MemCacheUsageMB:    &Gauge{},
		MemCacheUsagePct:   &Gauge{},
		L0HitRatio:         &Gauge{},
		L1HitRatio:         &Gauge{},
		L2MissRatio:        &Gauge{},
		ReadThroughputMBps: &Gauge{},
		FUSEReadLatency:    &Gauge{},
		FUSELookupLatency:  &Gauge{},
		FUSEGetattrLatency: &Gauge{},
	}
}
