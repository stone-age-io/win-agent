package tasks

import (
	"fmt"
	"io"
	"net/http"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// SystemMetrics represents system metrics collected from windows_exporter
type SystemMetrics struct {
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
	MemoryFreeGB         float64 `json:"memory_free_gb"`
	DiskFreePercent      float64 `json:"disk_free_percent"`
	DiskReadBytesPerSec  float64 `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec float64 `json:"disk_write_bytes_per_sec"`
	Timestamp            string  `json:"timestamp"`
}

// metricsCache stores previous counter values for rate calculation
type metricsCache struct {
	lastDiskReadBytes  float64
	lastDiskWriteBytes float64
	lastTimestamp      time.Time
}

var cache = &metricsCache{}

// ScrapeMetrics fetches and parses metrics from windows_exporter
func (e *Executor) ScrapeMetrics(exporterURL string) (*SystemMetrics, error) {
	// Fetch metrics from windows_exporter
	resp, err := http.Get(exporterURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse metrics using expfmt
	metrics, err := parsePrometheusMetrics(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	metrics.Timestamp = time.Now().UTC().Format(time.RFC3339)
	return metrics, nil
}

// parsePrometheusMetrics parses Prometheus format metrics using expfmt
func parsePrometheusMetrics(reader io.Reader) (*SystemMetrics, error) {
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metric families: %w", err)
	}

	metrics := &SystemMetrics{}

	// Extract CPU usage (calculate from idle time)
	if family, ok := metricFamilies["windows_cpu_time_total"]; ok {
		for _, m := range family.Metric {
			mode := getLabelValue(m.Label, "mode")
			if mode == "idle" && m.Counter != nil {
				// CPU usage = 100 - idle percentage
				// Note: This is a simplified calculation
				// For accurate usage, we'd need to track rates over time
				idlePercent := m.Counter.GetValue()
				if idlePercent <= 100 {
					metrics.CPUUsagePercent = 100 - idlePercent
				}
			}
		}
	}

	// Extract memory free bytes and convert to GB
	if family, ok := metricFamilies["windows_os_physical_memory_free_bytes"]; ok {
		if len(family.Metric) > 0 && family.Metric[0].Gauge != nil {
			bytes := family.Metric[0].Gauge.GetValue()
			metrics.MemoryFreeGB = bytes / 1024 / 1024 / 1024
		}
	}

	// Extract disk free space for C: drive
	if family, ok := metricFamilies["windows_logical_disk_free_bytes"]; ok {
		var freeBytes, totalBytes float64
		foundFree, foundTotal := false, false

		for _, m := range family.Metric {
			volume := getLabelValue(m.Label, "volume")
			if volume == "C:" && m.Gauge != nil {
				freeBytes = m.Gauge.GetValue()
				foundFree = true
				break
			}
		}

		// Get total size to calculate percentage
		if totalFamily, ok := metricFamilies["windows_logical_disk_size_bytes"]; ok {
			for _, m := range totalFamily.Metric {
				volume := getLabelValue(m.Label, "volume")
				if volume == "C:" && m.Gauge != nil {
					totalBytes = m.Gauge.GetValue()
					foundTotal = true
					break
				}
			}
		}

		if foundFree && foundTotal && totalBytes > 0 {
			metrics.DiskFreePercent = (freeBytes / totalBytes) * 100
		}
	}

	// Extract disk I/O rates (read and write)
	now := time.Now()
	timeDelta := now.Sub(cache.lastTimestamp).Seconds()

	if timeDelta > 0 {
		// Read bytes
		if family, ok := metricFamilies["windows_logical_disk_read_bytes_total"]; ok {
			for _, m := range family.Metric {
				volume := getLabelValue(m.Label, "volume")
				if volume == "C:" && m.Counter != nil {
					currentRead := m.Counter.GetValue()
					if cache.lastTimestamp.IsZero() {
						// First reading, just store it
						cache.lastDiskReadBytes = currentRead
					} else {
						// Calculate rate
						delta := currentRead - cache.lastDiskReadBytes
						metrics.DiskReadBytesPerSec = delta / timeDelta
						cache.lastDiskReadBytes = currentRead
					}
					break
				}
			}
		}

		// Write bytes
		if family, ok := metricFamilies["windows_logical_disk_write_bytes_total"]; ok {
			for _, m := range family.Metric {
				volume := getLabelValue(m.Label, "volume")
				if volume == "C:" && m.Counter != nil {
					currentWrite := m.Counter.GetValue()
					if cache.lastTimestamp.IsZero() {
						// First reading, just store it
						cache.lastDiskWriteBytes = currentWrite
					} else {
						// Calculate rate
						delta := currentWrite - cache.lastDiskWriteBytes
						metrics.DiskWriteBytesPerSec = delta / timeDelta
						cache.lastDiskWriteBytes = currentWrite
					}
					break
				}
			}
		}
	}

	cache.lastTimestamp = now

	return metrics, nil
}

// getLabelValue extracts a label value from a metric's label pairs
func getLabelValue(labels []*dto.LabelPair, name string) string {
	for _, label := range labels {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}

// MetricsError represents an error that occurred during metrics collection
type MetricsError struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

// CreateMetricsError creates an error message for metrics failures
func CreateMetricsError(err error) *MetricsError {
	return &MetricsError{
		Status:    "error",
		Error:     err.Error(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}
