package orchestrator

import (
	"sync"
	"time"
)

// HealthMonitor tracks collection health metrics across connector jobs.
type HealthMonitor struct {
	mu          sync.RWMutex
	connectors  map[string]*ConnectorHealth // GUID -> health
	runHistory  []*HealthReport
	maxHistory  int
}

// ConnectorHealth represents the health state of a single connector.
type ConnectorHealth struct {
	ConnectorGUID   string        `json:"connector_guid"`
	DeviceType      string        `json:"device_type"`
	DeviceID        string        `json:"device_id"`
	TotalRuns       int           `json:"total_runs"`
	SuccessfulRuns  int           `json:"successful_runs"`
	FailedRuns      int           `json:"failed_runs"`
	LastSuccessAt   *time.Time    `json:"last_success_at,omitempty"`
	LastFailureAt   *time.Time    `json:"last_failure_at,omitempty"`
	LastError       string        `json:"last_error,omitempty"`
	AvgDuration     time.Duration `json:"avg_duration"`
	MaxDuration     time.Duration `json:"max_duration"`
	MinDuration     time.Duration `json:"min_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
	SuccessRate     float64       `json:"success_rate"`
	DataFreshness   time.Duration `json:"data_freshness"` // Time since last successful collection
	FirstSeen       time.Time     `json:"first_seen"`
}

// HealthReport summarizes health across all connectors at a point in time.
type HealthReport struct {
	Timestamp       time.Time                    `json:"timestamp"`
	RunID           string                       `json:"run_id,omitempty"`
	EntryName       string                       `json:"entry_name,omitempty"`
	TotalConnectors int                          `json:"total_connectors"`
	HealthyCount    int                          `json:"healthy_count"`
	DegradedCount   int                          `json:"degraded_count"`
	FailedCount     int                          `json:"failed_count"`
	OverallSuccess  float64                      `json:"overall_success_rate"`
	Connectors      map[string]*ConnectorHealth  `json:"connectors"`
	Summary         string                       `json:"summary"`
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor() *HealthMonitor {
	return &HealthMonitor{
		connectors: make(map[string]*ConnectorHealth),
		runHistory: make([]*HealthReport, 0),
		maxHistory: 100,
	}
}

// RecordJobResult updates health metrics for a single job result.
func (hm *HealthMonitor) RecordJobResult(result *JobRunResult) {
	if result == nil || result.Job == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	guid := result.Job.GUID
	health, exists := hm.connectors[guid]
	if !exists {
		health = &ConnectorHealth{
			ConnectorGUID: guid,
			DeviceType:    string(result.Job.DeviceType),
			DeviceID:      result.Job.Config.Host,
			FirstSeen:     time.Now(),
		}
		hm.connectors[guid] = health
	}

	health.TotalRuns++
	if result.Success {
		health.SuccessfulRuns++
		now := time.Now()
		health.LastSuccessAt = &now
		health.DataFreshness = 0
	} else {
		health.FailedRuns++
		now := time.Now()
		health.LastFailureAt = &now
		health.LastError = result.Error
	}

	// Duration tracking
	health.TotalDuration += result.Duration
	if result.Duration > health.MaxDuration {
		health.MaxDuration = result.Duration
	}
	if health.MinDuration == 0 || result.Duration < health.MinDuration {
		health.MinDuration = result.Duration
	}
	if health.TotalRuns > 0 {
		health.AvgDuration = health.TotalDuration / time.Duration(health.TotalRuns)
	}

	// Success rate
	if health.TotalRuns > 0 {
		health.SuccessRate = float64(health.SuccessfulRuns) / float64(health.TotalRuns)
	}

	// Update data freshness
	if health.LastSuccessAt != nil {
		health.DataFreshness = time.Since(*health.LastSuccessAt)
	}
}

// RecordSchedulerRun records all results from a scheduled run.
func (hm *HealthMonitor) RecordSchedulerRun(entryName string, result *RunResult) {
	if result == nil {
		return
	}

	for _, jr := range result.Results {
		hm.RecordJobResult(jr)
	}

	report := hm.GenerateReport()
	report.RunID = result.RunID
	report.EntryName = entryName
	result.HealthReport = report

	hm.mu.Lock()
	hm.runHistory = append(hm.runHistory, report)
	if len(hm.runHistory) > hm.maxHistory {
		hm.runHistory = hm.runHistory[len(hm.runHistory)-hm.maxHistory:]
	}
	hm.mu.Unlock()
}

// GenerateReport creates a health report summarizing all tracked connectors.
func (hm *HealthMonitor) GenerateReport() *HealthReport {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	report := &HealthReport{
		Timestamp:       time.Now(),
		TotalConnectors: len(hm.connectors),
		Connectors:      make(map[string]*ConnectorHealth),
	}

	totalSuccessRate := 0.0
	for guid, health := range hm.connectors {
		// Copy health data
		h := *health
		report.Connectors[guid] = &h

		if health.SuccessRate >= 0.9 {
			report.HealthyCount++
		} else if health.SuccessRate >= 0.5 {
			report.DegradedCount++
		} else {
			report.FailedCount++
		}
		totalSuccessRate += health.SuccessRate
	}

	if report.TotalConnectors > 0 {
		report.OverallSuccess = totalSuccessRate / float64(report.TotalConnectors)
	}

	// Build summary
	if report.FailedCount > 0 {
		report.Summary = "CRITICAL: connectors failing"
	} else if report.DegradedCount > 0 {
		report.Summary = "WARNING: connectors degraded"
	} else if report.HealthyCount == report.TotalConnectors {
		report.Summary = "HEALTHY: all connectors operating normally"
	} else {
		report.Summary = "UNKNOWN: no connector data"
	}

	return report
}

// GetConnectorHealth returns health for a specific connector.
func (hm *HealthMonitor) GetConnectorHealth(guid string) *ConnectorHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.connectors[guid]
}

// GetAllHealth returns all connector health records.
func (hm *HealthMonitor) GetAllHealth() map[string]*ConnectorHealth {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	result := make(map[string]*ConnectorHealth, len(hm.connectors))
	for k, v := range hm.connectors {
		result[k] = v
	}
	return result
}

// GetRunHistory returns recent health reports.
func (hm *HealthMonitor) GetRunHistory(limit int) []*HealthReport {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	if limit <= 0 || limit > len(hm.runHistory) {
		limit = len(hm.runHistory)
	}
	start := len(hm.runHistory) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*HealthReport, limit)
	copy(result, hm.runHistory[start:])
	return result
}

// Reset clears all health tracking data.
func (hm *HealthMonitor) Reset() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.connectors = make(map[string]*ConnectorHealth)
	hm.runHistory = make([]*HealthReport, 0)
}
