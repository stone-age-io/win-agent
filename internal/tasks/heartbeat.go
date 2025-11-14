package tasks

import (
	"time"
)

// Heartbeat represents a heartbeat message
type Heartbeat struct {
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

// CreateHeartbeat creates a new heartbeat message
func (e *Executor) CreateHeartbeat(version string) *Heartbeat {
	return &Heartbeat{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   version,
	}
}
