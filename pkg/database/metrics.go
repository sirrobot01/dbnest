package database

import (
	"sync"
	"time"
)

const (
	// MaxHistoryPoints is the maximum number of metrics points to keep per database
	MaxHistoryPoints = 60 // 1 hour at 1-minute intervals
)

// MetricsPoint represents a single metrics snapshot
type MetricsPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpuPercent"`
	MemoryUsage   int64     `json:"memoryUsage"`
	MemoryLimit   int64     `json:"memoryLimit"`
	MemoryPercent float64   `json:"memoryPercent"`
	StorageUsed   int64     `json:"storageUsed"`
	Connections   int       `json:"connections"`
	NetworkRx     int64     `json:"networkRx"`
	NetworkTx     int64     `json:"networkTx"`
}

// MetricsHistory stores historical metrics for databases
type MetricsHistory struct {
	mu      sync.RWMutex
	history map[string][]MetricsPoint // database ID -> metrics points
}

// NewMetricsHistory creates a new metrics history store
func NewMetricsHistory() *MetricsHistory {
	return &MetricsHistory{
		history: make(map[string][]MetricsPoint),
	}
}

// Record adds a new metrics point for a database
func (mh *MetricsHistory) Record(dbID string, point MetricsPoint) {
	mh.mu.Lock()
	defer mh.mu.Unlock()

	points := mh.history[dbID]
	
	// Add new point
	points = append(points, point)
	
	// Keep only the last MaxHistoryPoints
	if len(points) > MaxHistoryPoints {
		points = points[len(points)-MaxHistoryPoints:]
	}
	
	mh.history[dbID] = points
}

// Get returns the metrics history for a database
func (mh *MetricsHistory) Get(dbID string) []MetricsPoint {
	mh.mu.RLock()
	defer mh.mu.RUnlock()

	points := mh.history[dbID]
	if points == nil {
		return []MetricsPoint{}
	}
	
	// Return a copy to avoid race conditions
	result := make([]MetricsPoint, len(points))
	copy(result, points)
	return result
}

// Delete removes the metrics history for a database
func (mh *MetricsHistory) Delete(dbID string) {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	delete(mh.history, dbID)
}

// Clear removes all metrics history
func (mh *MetricsHistory) Clear() {
	mh.mu.Lock()
	defer mh.mu.Unlock()
	mh.history = make(map[string][]MetricsPoint)
}
