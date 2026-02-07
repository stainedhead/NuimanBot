package analytics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Config defines analytics configuration.
type Config struct {
	Enabled       bool
	ServiceName   string
	FlushInterval time.Duration // How often to flush events
	BatchSize     int           // Maximum events before auto-flush
}

// Event represents a tracked event.
type Event struct {
	Name       string
	UserID     string
	Timestamp  time.Time
	Properties map[string]any
}

// Metric represents a tracked metric.
type Metric struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Tags      map[string]string
}

// Statistics provides analytics statistics.
type Statistics struct {
	TotalEvents  int64
	TotalMetrics int64
	UniqueUsers  int64
	StartTime    time.Time
}

var (
	globalConfig   Config
	initialized    bool
	mu             sync.RWMutex
	eventBuffer    []Event
	metricBuffer   []Metric
	bufferMu       sync.Mutex
	stats          Statistics
	statsMu        sync.RWMutex
	uniqueUsers    map[string]bool
	stopChan       chan struct{}
	flushWg        sync.WaitGroup
)

// Initialize sets up the analytics system.
func Initialize(config Config) error {
	mu.Lock()
	defer mu.Unlock()

	globalConfig = config
	initialized = true
	eventBuffer = make([]Event, 0, config.BatchSize)
	metricBuffer = make([]Metric, 0, config.BatchSize)
	uniqueUsers = make(map[string]bool)
	stats = Statistics{
		StartTime: time.Now(),
	}
	stopChan = make(chan struct{})

	if config.Enabled {
		// Start background flusher if interval is set
		if config.FlushInterval > 0 {
			flushWg.Add(1)
			go backgroundFlusher(config.FlushInterval)
		}

		slog.Info("Analytics initialized",
			"service", config.ServiceName,
			"flush_interval", config.FlushInterval,
			"batch_size", config.BatchSize,
		)
	} else {
		slog.Info("Analytics disabled")
	}

	return nil
}

// Shutdown cleanly shuts down analytics.
func Shutdown() error {
	mu.Lock()
	if initialized && globalConfig.Enabled {
		close(stopChan)
		mu.Unlock()

		// Wait for flusher to finish
		flushWg.Wait()

		// Final flush
		flush()

		mu.Lock()
		slog.Info("Analytics shutdown")
	}

	initialized = false
	eventBuffer = nil
	metricBuffer = nil
	uniqueUsers = nil
	mu.Unlock()

	return nil
}

// TrackEvent tracks an event.
func TrackEvent(ctx context.Context, event Event) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	batchSize := globalConfig.BatchSize
	mu.RUnlock()

	if !enabled {
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Add to buffer
	bufferMu.Lock()
	eventBuffer = append(eventBuffer, event)
	shouldFlush := len(eventBuffer) >= batchSize
	bufferMu.Unlock()

	// Update statistics
	statsMu.Lock()
	stats.TotalEvents++
	if event.UserID != "" && !uniqueUsers[event.UserID] {
		uniqueUsers[event.UserID] = true
		stats.UniqueUsers++
	}
	statsMu.Unlock()

	// Auto-flush if batch size reached
	if shouldFlush && batchSize > 0 {
		flush()
	}

	slog.Debug("Event tracked",
		"name", event.Name,
		"user_id", event.UserID,
	)
}

// TrackMetric tracks a metric value.
func TrackMetric(ctx context.Context, metric Metric) {
	mu.RLock()
	enabled := initialized && globalConfig.Enabled
	batchSize := globalConfig.BatchSize
	mu.RUnlock()

	if !enabled {
		return
	}

	// Set timestamp if not provided
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	// Add to buffer
	bufferMu.Lock()
	metricBuffer = append(metricBuffer, metric)
	shouldFlush := len(metricBuffer) >= batchSize
	bufferMu.Unlock()

	// Update statistics
	statsMu.Lock()
	stats.TotalMetrics++
	statsMu.Unlock()

	// Auto-flush if batch size reached
	if shouldFlush && batchSize > 0 {
		flush()
	}

	slog.Debug("Metric tracked",
		"name", metric.Name,
		"value", metric.Value,
	)
}

// GetStatistics returns current analytics statistics.
func GetStatistics(ctx context.Context) *Statistics {
	statsMu.RLock()
	defer statsMu.RUnlock()

	// Return a copy
	return &Statistics{
		TotalEvents:  stats.TotalEvents,
		TotalMetrics: stats.TotalMetrics,
		UniqueUsers:  stats.UniqueUsers,
		StartTime:    stats.StartTime,
	}
}

// flush sends buffered events and metrics to the analytics backend.
// For MVP, this logs the data. In production, send to analytics service.
func flush() {
	bufferMu.Lock()
	events := eventBuffer
	metrics := metricBuffer
	eventBuffer = make([]Event, 0, globalConfig.BatchSize)
	metricBuffer = make([]Metric, 0, globalConfig.BatchSize)
	bufferMu.Unlock()

	if len(events) == 0 && len(metrics) == 0 {
		return
	}

	slog.Info("Analytics flush",
		"events", len(events),
		"metrics", len(metrics),
	)

	// TODO: In production, send to analytics service
	// - Send to Mixpanel, Amplitude, or custom analytics DB
	// - Batch events for efficiency
	// - Handle retries on failure
	// Example:
	// for _, event := range events {
	//     analyticsClient.Track(event.UserID, event.Name, event.Properties)
	// }
	// for _, metric := range metrics {
	//     analyticsClient.RecordMetric(metric.Name, metric.Value, metric.Tags)
	// }
}

// backgroundFlusher periodically flushes buffered data.
func backgroundFlusher(interval time.Duration) {
	defer flushWg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			flush()
		case <-stopChan:
			return
		}
	}
}
