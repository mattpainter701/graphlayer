package appid

import (
	"strings"
)

/*
Health assessment for App-ID and ISDB caches.

Health states:
- healthy: Fresh cache with complete data
- stale: Old cache but still usable
- critically_stale: Very old cache, should refresh
- incomplete: Partial fetch with insufficient data
- unhealthy: Errors or corruption
- empty: No cache exists

Uses conservative thresholds to avoid false negatives.
*/

// HealthChecker assesses cache health
type HealthChecker struct {
	logger Logger
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger Logger) *HealthChecker {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &HealthChecker{
		logger: logger,
	}
}

// ========================================
// App-ID Health Assessment
// ========================================

// AssessAppIDHealth determines the health status of an App-ID cache
func (h *HealthChecker) AssessAppIDHealth(cache *AppIDCache, metadata *CacheMetadata) HealthStatus {
	// Empty check
	if cache == nil || len(cache.Applications) == 0 {
		return HealthStatusEmpty
	}

	if metadata == nil {
		h.logger.Warn("Missing metadata for health assessment, assuming healthy")
		return HealthStatusHealthy
	}

	// Check staleness first
	if metadata.StalnessDays > CriticallyStaleDays {
		h.logger.Warn("Cache critically stale: %d days old (threshold: %d)",
			metadata.StalnessDays, CriticallyStaleDays)
		return HealthStatusCriticallyStale
	}

	if metadata.StalnessDays > StaleDays {
		h.logger.Info("Cache stale: %d days old (threshold: %d)",
			metadata.StalnessDays, StaleDays)
		return HealthStatusStale
	}

	// Check completeness (conservative threshold)
	if len(cache.Applications) < MinHealthyAppCount {
		h.logger.Warn("Cache incomplete: only %d apps (threshold: %d)",
			len(cache.Applications), MinHealthyAppCount)
		return HealthStatusIncomplete
	}

	// Check recent fetch status
	if len(metadata.FetchHistory) > 0 {
		lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]

		// Check for errors in last fetch
		if lastFetch.Status == FetchStatusFailed {
			h.logger.Warn("Last fetch failed: %s", lastFetch.Error)
			return HealthStatusUnhealthy
		}

		// Check for repeated errors
		recentErrorCount := metadata.CountRecentErrors(3) // Last 3 fetches
		if recentErrorCount >= 2 {
			h.logger.Warn("Multiple recent fetch errors: %d of last 3 fetches", recentErrorCount)
			return HealthStatusUnhealthy
		}

		// Check for incomplete partial fetch
		if lastFetch.Status == FetchStatusPartialIncomplete {
			// If we have decent coverage despite partial fetch, consider it usable but stale
			if len(cache.Applications) >= MinHealthyAppCount {
				h.logger.Info("Partial fetch but sufficient data (%d apps)", len(cache.Applications))
				return HealthStatusStale
			} else {
				h.logger.Warn("Partial fetch with insufficient data (%d apps)", len(cache.Applications))
				return HealthStatusIncomplete
			}
		}

		// Check completeness percentage for partial fetches
		if lastFetch.Completeness > 0 && lastFetch.Completeness < MinCompletionPercentage {
			h.logger.Warn("Cache below completeness threshold: %.1f%% (threshold: %.1f%%)",
				lastFetch.Completeness*100, MinCompletionPercentage*100)
			return HealthStatusIncomplete
		}
	}

	// All checks passed
	h.logger.Debug("Cache healthy: %d apps, %d days old", len(cache.Applications), metadata.StalnessDays)
	return HealthStatusHealthy
}

// ========================================
// ISDB Health Assessment
// ========================================

// AssessISDBHealth determines the health status of an ISDB cache
func (h *HealthChecker) AssessISDBHealth(cache *ISDBCache, metadata *CacheMetadata) HealthStatus {
	// Empty check
	if cache == nil || len(cache.Services) == 0 {
		return HealthStatusEmpty
	}

	if metadata == nil {
		h.logger.Warn("Missing metadata for ISDB health assessment, assuming healthy")
		return HealthStatusHealthy
	}

	// Check staleness
	if metadata.StalnessDays > CriticallyStaleDays {
		h.logger.Warn("ISDB cache critically stale: %d days old (threshold: %d)",
			metadata.StalnessDays, CriticallyStaleDays)
		return HealthStatusCriticallyStale
	}

	if metadata.StalnessDays > StaleDays {
		h.logger.Info("ISDB cache stale: %d days old (threshold: %d)",
			metadata.StalnessDays, StaleDays)
		return HealthStatusStale
	}

	// Check completeness (conservative threshold for ISDB)
	if len(cache.Services) < MinHealthyServiceCount {
		h.logger.Warn("ISDB cache incomplete: only %d services (threshold: %d)",
			len(cache.Services), MinHealthyServiceCount)
		return HealthStatusIncomplete
	}

	// Check recent fetch status
	if len(metadata.FetchHistory) > 0 {
		lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]

		if lastFetch.Status == FetchStatusFailed {
			h.logger.Warn("Last ISDB fetch failed: %s", lastFetch.Error)
			return HealthStatusUnhealthy
		}

		// Check for repeated errors
		recentErrorCount := metadata.CountRecentErrors(3)
		if recentErrorCount >= 2 {
			h.logger.Warn("Multiple recent ISDB fetch errors: %d of last 3 fetches", recentErrorCount)
			return HealthStatusUnhealthy
		}

		// Check for incomplete partial fetch
		if lastFetch.Status == FetchStatusPartialIncomplete {
			if len(cache.Services) >= MinHealthyServiceCount {
				h.logger.Info("Partial ISDB fetch but sufficient data (%d services)", len(cache.Services))
				return HealthStatusStale
			} else {
				h.logger.Warn("Partial ISDB fetch with insufficient data (%d services)", len(cache.Services))
				return HealthStatusIncomplete
			}
		}

		// Check completeness percentage
		if lastFetch.Completeness > 0 && lastFetch.Completeness < MinCompletionPercentage {
			h.logger.Warn("ISDB cache below completeness threshold: %.1f%% (threshold: %.1f%%)",
				lastFetch.Completeness*100, MinCompletionPercentage*100)
			return HealthStatusIncomplete
		}
	}

	// All checks passed
	h.logger.Debug("ISDB cache healthy: %d services, %d days old", len(cache.Services), metadata.StalnessDays)
	return HealthStatusHealthy
}

// ========================================
// Health Status Helpers
// ========================================

// IsUsable returns true if the cache is usable (not empty or unhealthy)
func IsUsable(status HealthStatus) bool {
	return status == HealthStatusHealthy ||
	       status == HealthStatusStale ||
	       status == HealthStatusCriticallyStale
}

// ShouldRefresh returns true if the cache should be refreshed
func ShouldRefresh(status HealthStatus) bool {
	return status == HealthStatusStale ||
	       status == HealthStatusCriticallyStale ||
	       status == HealthStatusIncomplete ||
	       status == HealthStatusUnhealthy ||
	       status == HealthStatusEmpty
}

// MustRefresh returns true if the cache MUST be refreshed (unusable)
func MustRefresh(status HealthStatus) bool {
	return status == HealthStatusUnhealthy ||
	       status == HealthStatusEmpty
}

// GetHealthDescription returns a human-readable description of health status
func GetHealthDescription(status HealthStatus) string {
	switch status {
	case HealthStatusHealthy:
		return "Cache is fresh and complete"
	case HealthStatusStale:
		return "Cache is old but usable (consider refreshing)"
	case HealthStatusCriticallyStale:
		return "Cache is very old (should refresh soon)"
	case HealthStatusIncomplete:
		return "Cache has incomplete data (partial fetch)"
	case HealthStatusUnhealthy:
		return "Cache has errors or corruption (refresh required)"
	case HealthStatusEmpty:
		return "No cache exists (initial fetch required)"
	default:
		return "Unknown health status"
	}
}

// ========================================
// Quick Health Check (without metadata)
// ========================================

// QuickAppIDHealthCheck performs basic health check without metadata
// Used when metadata is not available but we still need a health assessment
func QuickAppIDHealthCheck(cache *AppIDCache) HealthStatus {
	if cache == nil || len(cache.Applications) == 0 {
		return HealthStatusEmpty
	}

	// Basic completeness check
	if len(cache.Applications) < MinHealthyAppCount {
		return HealthStatusIncomplete
	}

	// No metadata, assume healthy if we have enough apps
	return HealthStatusHealthy
}

// QuickISDBHealthCheck performs basic health check without metadata
func QuickISDBHealthCheck(cache *ISDBCache) HealthStatus {
	if cache == nil || len(cache.Services) == 0 {
		return HealthStatusEmpty
	}

	// Basic completeness check
	if len(cache.Services) < MinHealthyServiceCount {
		return HealthStatusIncomplete
	}

	// No metadata, assume healthy if we have enough services
	return HealthStatusHealthy
}

// ========================================
// Detailed Health Report
// ========================================

// HealthReport provides detailed health information
type HealthReport struct {
	Status       HealthStatus `json:"status"`
	Description  string       `json:"description"`
	IsUsable     bool         `json:"is_usable"`
	ShouldRefresh bool        `json:"should_refresh"`
	MustRefresh  bool         `json:"must_refresh"`

	// Metrics
	TotalEntries  int     `json:"total_entries"`
	StalnessDays  int     `json:"staleness_days"`
	Completeness  float64 `json:"completeness,omitempty"`

	// Recent activity
	LastFetchStatus string   `json:"last_fetch_status,omitempty"`
	RecentErrors    []string `json:"recent_errors,omitempty"`

	// Recommendations
	Recommendations []string `json:"recommendations"`
}

// GenerateAppIDHealthReport creates a detailed health report for App-ID cache
func (h *HealthChecker) GenerateAppIDHealthReport(cache *AppIDCache, metadata *CacheMetadata) *HealthReport {
	status := h.AssessAppIDHealth(cache, metadata)

	report := &HealthReport{
		Status:        status,
		Description:   GetHealthDescription(status),
		IsUsable:      IsUsable(status),
		ShouldRefresh: ShouldRefresh(status),
		MustRefresh:   MustRefresh(status),
		Recommendations: []string{},
	}

	if cache != nil {
		report.TotalEntries = len(cache.Applications)
	}

	if metadata != nil {
		report.StalnessDays = metadata.StalnessDays

		if len(metadata.FetchHistory) > 0 {
			lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]
			report.LastFetchStatus = string(lastFetch.Status)
			report.Completeness = lastFetch.Completeness
		}

		report.RecentErrors = metadata.GetRecentErrors(3)
	}

	// Generate recommendations
	report.Recommendations = h.generateAppIDRecommendations(status, cache, metadata)

	return report
}

// GenerateISDBHealthReport creates a detailed health report for ISDB cache
func (h *HealthChecker) GenerateISDBHealthReport(cache *ISDBCache, metadata *CacheMetadata) *HealthReport {
	status := h.AssessISDBHealth(cache, metadata)

	report := &HealthReport{
		Status:        status,
		Description:   GetHealthDescription(status),
		IsUsable:      IsUsable(status),
		ShouldRefresh: ShouldRefresh(status),
		MustRefresh:   MustRefresh(status),
		Recommendations: []string{},
	}

	if cache != nil {
		report.TotalEntries = len(cache.Services)
	}

	if metadata != nil {
		report.StalnessDays = metadata.StalnessDays

		if len(metadata.FetchHistory) > 0 {
			lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]
			report.LastFetchStatus = string(lastFetch.Status)
			report.Completeness = lastFetch.Completeness
		}

		report.RecentErrors = metadata.GetRecentErrors(3)
	}

	// Generate recommendations
	report.Recommendations = h.generateISDBRecommendations(status, cache, metadata)

	return report
}

// generateAppIDRecommendations provides actionable recommendations
func (h *HealthChecker) generateAppIDRecommendations(status HealthStatus, cache *AppIDCache, metadata *CacheMetadata) []string {
	var recommendations []string

	switch status {
	case HealthStatusEmpty:
		recommendations = append(recommendations, "Perform initial App-ID collection from device")

	case HealthStatusIncomplete:
		recommendations = append(recommendations,
			"Retry full collection to get complete App-ID database",
			"Check network connectivity and API timeouts")

	case HealthStatusUnhealthy:
		recommendations = append(recommendations,
			"Fix errors preventing successful collection",
			"Check device API access and credentials")
		if metadata != nil && len(metadata.GetRecentErrors(1)) > 0 {
			recommendations = append(recommendations,
				"Review recent error: "+metadata.GetRecentErrors(1)[0])
		}

	case HealthStatusStale, HealthStatusCriticallyStale:
		recommendations = append(recommendations,
			"Refresh App-ID cache from device",
			"App-ID database may have been updated on device")

	case HealthStatusHealthy:
		if metadata != nil && metadata.StalnessDays > StaleDays/2 {
			recommendations = append(recommendations,
				"Consider scheduling refresh within next few weeks")
		}
	}

	// Check for partial fetches
	if metadata != nil && len(metadata.FetchHistory) > 0 {
		lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]
		if strings.Contains(string(lastFetch.Status), "partial") {
			recommendations = append(recommendations,
				"Last fetch was partial - increase timeout for full collection")
		}
	}

	return recommendations
}

// generateISDBRecommendations provides actionable recommendations for ISDB
func (h *HealthChecker) generateISDBRecommendations(status HealthStatus, cache *ISDBCache, metadata *CacheMetadata) []string {
	var recommendations []string

	switch status {
	case HealthStatusEmpty:
		recommendations = append(recommendations, "Perform initial ISDB collection from FortiGate")

	case HealthStatusIncomplete:
		recommendations = append(recommendations,
			"Retry full collection to get complete ISDB",
			"Check SSH connectivity and CLI timeouts",
			"Verify FortiGate has full ISDB (some models have mini version)")

	case HealthStatusUnhealthy:
		recommendations = append(recommendations,
			"Fix errors preventing successful ISDB collection",
			"Check SSH access and FortiGate CLI permissions")
		if metadata != nil && len(metadata.GetRecentErrors(1)) > 0 {
			recommendations = append(recommendations,
				"Review recent error: "+metadata.GetRecentErrors(1)[0])
		}

	case HealthStatusStale, HealthStatusCriticallyStale:
		recommendations = append(recommendations,
			"Refresh ISDB cache from FortiGate",
			"ISDB may have been updated via FortiGuard")

	case HealthStatusHealthy:
		if metadata != nil && metadata.StalnessDays > StaleDays/2 {
			recommendations = append(recommendations,
				"Consider scheduling ISDB refresh within next few weeks")
		}
	}

	// Check for partial fetches
	if metadata != nil && len(metadata.FetchHistory) > 0 {
		lastFetch := metadata.FetchHistory[len(metadata.FetchHistory)-1]
		if strings.Contains(string(lastFetch.Status), "partial") {
			recommendations = append(recommendations,
				"Last fetch was partial - use smaller chunk size or increase timeout")
		}
	}

	return recommendations
}
