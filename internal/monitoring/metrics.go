package monitoring

import (
	"expvar"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Metrics holds all application metrics
type Metrics struct {
	// API request metrics
	apiRequests        *expvar.Int
	apiRequestsTotal   *expvar.Int
	apiErrors          *expvar.Int
	apiDuration        *expvar.Float
	apiRequestsByPath  *expvar.Map
	apiDurationByPath  *expvar.Map // Average duration per endpoint
	apiDurationSumByPath map[string]int64 // Sum of durations for averaging
	apiCountByPath       map[string]int64 // Count for averaging

	// Cache metrics
	cacheHits   *expvar.Int
	cacheMisses *expvar.Int

	// Logger for API requests
	requestLogger *log.Logger
	logFile       *os.File
	logMutex      sync.Mutex
}

var (
	globalMetrics *Metrics
	once          sync.Once
)

// Initialize sets up the metrics system
func Initialize(logDir string) (*Metrics, error) {
	var initErr error
	once.Do(func() {
		// Create log directory if it doesn't exist
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log directory: %w", err)
			return
		}

		// Open log file for API requests
		logPath := filepath.Join(logDir, fmt.Sprintf("api-requests-%s.log", time.Now().Format("2006-01-02")))
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			initErr = fmt.Errorf("failed to open log file: %w", err)
			return
		}

		// Create logger
		requestLogger := log.New(logFile, "", log.LstdFlags)

		// Initialize metrics
		globalMetrics = &Metrics{
			apiRequests:          expvar.NewInt("api.requests.inflight"),
			apiRequestsTotal:     expvar.NewInt("api.requests.total"),
			apiErrors:            expvar.NewInt("api.requests.errors"),
			apiDuration:          expvar.NewFloat("api.requests.duration_ms"),
			apiRequestsByPath:    expvar.NewMap("api.requests.by_path"),
			apiDurationByPath:    expvar.NewMap("api.duration.by_path"),
			apiDurationSumByPath: make(map[string]int64),
			apiCountByPath:       make(map[string]int64),
			cacheHits:            expvar.NewInt("cache.hits"),
			cacheMisses:          expvar.NewInt("cache.misses"),
			requestLogger:        requestLogger,
			logFile:              logFile,
		}

		// Publish custom metrics
		expvar.Publish("api.cache_hit_ratio", expvar.Func(func() interface{} {
			hits := globalMetrics.cacheHits.Value()
			misses := globalMetrics.cacheMisses.Value()
			total := hits + misses
			if total == 0 {
				return 0.0
			}
			return float64(hits) / float64(total) * 100.0
		}))
	})

	if initErr != nil {
		return nil, initErr
	}

	return globalMetrics, nil
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordAPIRequest logs and tracks an API request
func (m *Metrics) RecordAPIRequest(method, path string, statusCode int, duration time.Duration, err error) {
	if m == nil {
		return
	}

	// Update metrics
	m.apiRequestsTotal.Add(1)
	m.apiDuration.Set(float64(duration.Milliseconds()))

	// Track by path
	pathKey := fmt.Sprintf("%s %s", method, path)
	m.apiRequestsByPath.Add(pathKey, 1)

	// Track average duration per endpoint
	m.logMutex.Lock()
	m.apiDurationSumByPath[pathKey] += duration.Milliseconds()
	m.apiCountByPath[pathKey]++
	avgDuration := m.apiDurationSumByPath[pathKey] / m.apiCountByPath[pathKey]
	m.logMutex.Unlock()

	// Update expvar map with average duration
	m.apiDurationByPath.Set(pathKey, new(expvar.Int))
	if v := m.apiDurationByPath.Get(pathKey); v != nil {
		if intVar, ok := v.(*expvar.Int); ok {
			intVar.Set(avgDuration)
		}
	}

	if err != nil || statusCode >= 400 {
		m.apiErrors.Add(1)
	}

	// Log the request
	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	logEntry := fmt.Sprintf("[%s] %s %s | Status: %d | Duration: %dms",
		time.Now().Format("15:04:05"),
		method,
		path,
		statusCode,
		duration.Milliseconds())

	if err != nil {
		logEntry += fmt.Sprintf(" | Error: %v", err)
	}

	if m.requestLogger != nil {
		m.requestLogger.Println(logEntry)
	}
}

// LogRequestBody logs the request body for debugging
func (m *Metrics) LogRequestBody(method, path string, body string) {
	if m == nil || m.requestLogger == nil {
		return
	}

	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	m.requestLogger.Printf("[%s] %s %s Request Body:\n%s",
		time.Now().Format("15:04:05"),
		method,
		path,
		body)
}

// LogResponseBody logs the response body for debugging
func (m *Metrics) LogResponseBody(method, path string, statusCode int, body string) {
	if m == nil || m.requestLogger == nil {
		return
	}

	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	m.requestLogger.Printf("[%s] %s %s Response (status %d):\n%s",
		time.Now().Format("15:04:05"),
		method,
		path,
		statusCode,
		body)
}

// StartAPIRequest marks the start of an API request
func (m *Metrics) StartAPIRequest() {
	if m != nil {
		m.apiRequests.Add(1)
	}
}

// EndAPIRequest marks the end of an API request
func (m *Metrics) EndAPIRequest() {
	if m != nil {
		m.apiRequests.Add(-1)
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	if m != nil {
		m.cacheHits.Add(1)
	}
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	if m != nil {
		m.cacheMisses.Add(1)
	}
}

// Close closes the log file
func (m *Metrics) Close() error {
	if m != nil && m.logFile != nil {
		return m.logFile.Close()
	}
	return nil
}
