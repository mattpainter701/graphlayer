package appid

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
Merger implements intelligent cache merging with union strategy.

Key principles:
- Union merge: Never delete entries unless full fetch succeeded
- Device data is source of truth (prefer new data on conflicts)
- Preserve existing data on partial fetches
- Track all changes for audit trail
*/

// CacheMerger handles merging of App-ID and ISDB caches
type CacheMerger struct {
	logger Logger
}

// NewCacheMerger creates a new cache merger
func NewCacheMerger(logger Logger) *CacheMerger {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &CacheMerger{
		logger: logger,
	}
}

// ========================================
// App-ID Merge
// ========================================

// MergeAppID merges new App-ID data with existing cache
//
// Merge Strategy:
// - If existing is nil/empty: full replace (first fetch)
// - Union merge: Add new apps, update changed apps
// - Only remove apps if fetchStatus == full_success
// - On conflicts: prefer new data (device is source of truth)
func (m *CacheMerger) MergeAppID(existing *AppIDCache, newData *AppIDCache, fetchMeta FetchMetadata) (*AppIDCache, *MergeReport, error) {
	report := NewMergeReport(fetchMeta.Source)

	// Validation
	if newData == nil {
		return nil, nil, fmt.Errorf("new data is nil")
	}

	// If existing is empty, this is first fetch - use new data as-is
	if existing == nil || len(existing.Applications) == 0 {
		m.logger.Info("First fetch for device, using new data as-is (%d apps)", len(newData.Applications))
		report.FullReplace = true
		for appName := range newData.Applications {
			report.Added = append(report.Added, appName)
		}
		report.NewTotalCount = len(newData.Applications)
		return newData, report, nil
	}

	report.OldTotalCount = len(existing.Applications)

	// Clone existing cache to avoid modifying original
	merged := existing.Clone()

	// Merge strategy: Add new apps and update existing ones
	for appName, newApp := range newData.Applications {
		existingApp, exists := merged.Applications[appName]

		if exists {
			// Application exists - check if it changed
			if !appsEqual(existingApp, newApp) {
				// Conflict: use new data (device is source of truth)
				m.logger.Debug("App updated: %s", appName)

				// Track conflict if significantly different
				if shouldTrackConflict(existingApp, newApp) {
					report.AddConflict(
						appName,
						"data_mismatch",
						summarizeApp(existingApp),
						summarizeApp(newApp),
						"using_device_data",
					)
				}

				merged.Applications[appName] = newApp
				report.Updated = append(report.Updated, appName)
			} else {
				report.Unchanged++
			}
		} else {
			// New application - add it
			m.logger.Debug("App added: %s", appName)
			merged.Applications[appName] = newApp
			report.Added = append(report.Added, appName)
		}
	}

	// Handle removals (only if this was a full fetch)
	if fetchMeta.Status == FetchStatusFullSuccess {
		for appName := range existing.Applications {
			if _, exists := newData.Applications[appName]; !exists {
				// App no longer in device database - remove it
				m.logger.Debug("App removed: %s", appName)
				delete(merged.Applications, appName)
				report.Removed = append(report.Removed, appName)
			}
		}
	} else {
		// Partial fetch - keep existing apps not in new data
		preservedCount := len(existing.Applications) - len(newData.Applications)
		if preservedCount > 0 {
			warning := fmt.Sprintf("Partial fetch detected, preserved %d existing apps not in new data", preservedCount)
			m.logger.Warn(warning)
			report.AddWarning(warning)
		}
	}

	// Update metadata in merged cache
	merged.Version = newData.Version
	merged.LastUpdated = newData.LastUpdated
	merged.Source = newData.Source

	report.NewTotalCount = len(merged.Applications)

	m.logger.Info("App-ID merge complete: %s", report.Summary())

	return merged, report, nil
}

// ========================================
// ISDB Merge
// ========================================

// MergeISDB merges new ISDB data with existing cache
//
// Same strategy as App-ID merge but for ISDB services
func (m *CacheMerger) MergeISDB(existing *ISDBCache, newData *ISDBCache, fetchMeta FetchMetadata) (*ISDBCache, *MergeReport, error) {
	report := NewMergeReport(fetchMeta.Source)

	// Validation
	if newData == nil {
		return nil, nil, fmt.Errorf("new data is nil")
	}

	// If existing is empty, this is first fetch - use new data as-is
	if existing == nil || len(existing.Services) == 0 {
		m.logger.Info("First ISDB fetch for device, using new data as-is (%d services)", len(newData.Services))
		report.FullReplace = true
		for serviceID := range newData.Services {
			report.Added = append(report.Added, serviceID)
		}
		report.NewTotalCount = len(newData.Services)
		return newData, report, nil
	}

	report.OldTotalCount = len(existing.Services)

	// Clone existing cache
	merged := existing.Clone()

	// Merge services (by ID)
	for serviceID, newService := range newData.Services {
		existingService, exists := merged.Services[serviceID]

		if exists {
			// Service exists - check if it changed
			if !servicesEqual(existingService, newService) {
				m.logger.Debug("ISDB service updated: %s (ID: %s)", newService.Name, serviceID)

				if shouldTrackServiceConflict(existingService, newService) {
					report.AddConflict(
						newService.Name,
						"data_mismatch",
						summarizeService(existingService),
						summarizeService(newService),
						"using_device_data",
					)
				}

				merged.Services[serviceID] = newService
				// Update name index as well
				merged.ServicesByName[strings.ToLower(newService.Name)] = newService
				report.Updated = append(report.Updated, newService.Name)
			} else {
				report.Unchanged++
			}
		} else {
			// New service - add it
			m.logger.Debug("ISDB service added: %s (ID: %s)", newService.Name, serviceID)
			merged.Services[serviceID] = newService
			merged.ServicesByName[strings.ToLower(newService.Name)] = newService
			report.Added = append(report.Added, newService.Name)
		}
	}

	// Handle removals (only if this was a full fetch)
	if fetchMeta.Status == FetchStatusFullSuccess {
		for serviceID, service := range existing.Services {
			if _, exists := newData.Services[serviceID]; !exists {
				m.logger.Debug("ISDB service removed: %s (ID: %s)", service.Name, serviceID)
				delete(merged.Services, serviceID)
				delete(merged.ServicesByName, strings.ToLower(service.Name))
				report.Removed = append(report.Removed, service.Name)
			}
		}
	} else {
		// Partial fetch - preserve existing services
		preservedCount := len(existing.Services) - len(newData.Services)
		if preservedCount > 0 {
			warning := fmt.Sprintf("Partial fetch detected, preserved %d existing services not in new data", preservedCount)
			m.logger.Warn(warning)
			report.AddWarning(warning)
		}
	}

	// Update metadata
	merged.Version = newData.Version
	merged.Date = newData.Date
	merged.Server = newData.Server

	report.NewTotalCount = len(merged.Services)

	m.logger.Info("ISDB merge complete: %s", report.Summary())

	return merged, report, nil
}

// ========================================
// Helper Functions
// ========================================

// appsEqual compares two App-ID entries for equality
func appsEqual(a, b *AppIDEntry) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare key fields that matter for functionality
	if a.Name != b.Name ||
	   a.Category != b.Category ||
	   a.DefaultProtocol != b.DefaultProtocol ||
	   a.Risk != b.Risk {
		return false
	}

	// Compare port arrays
	if len(a.DefaultPorts) != len(b.DefaultPorts) {
		return false
	}
	for i := range a.DefaultPorts {
		if a.DefaultPorts[i] != b.DefaultPorts[i] {
			return false
		}
	}

	return true
}

// servicesEqual compares two ISDB service entries for equality
func servicesEqual(a, b *ISDBServiceEntry) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare key fields
	if a.ID != b.ID ||
	   a.Name != b.Name {
		return false
	}

	// Compare protocol arrays
	if len(a.Protocols) != len(b.Protocols) {
		return false
	}
	for i := range a.Protocols {
		if a.Protocols[i] != b.Protocols[i] {
			return false
		}
	}

	// Compare port arrays
	if len(a.Ports) != len(b.Ports) {
		return false
	}
	for i := range a.Ports {
		if a.Ports[i] != b.Ports[i] {
			return false
		}
	}

	return true
}

// shouldTrackConflict determines if a conflict is significant enough to track
func shouldTrackConflict(oldApp, newApp *AppIDEntry) bool {
	// Track if risk level changed significantly
	if abs(oldApp.Risk - newApp.Risk) >= 2 {
		return true
	}

	// Track if ports changed
	if len(oldApp.DefaultPorts) != len(newApp.DefaultPorts) {
		return true
	}

	// Track if protocol changed
	if oldApp.DefaultProtocol != newApp.DefaultProtocol {
		return true
	}

	return false
}

// shouldTrackServiceConflict determines if an ISDB conflict should be tracked
func shouldTrackServiceConflict(oldService, newService *ISDBServiceEntry) bool {
	// Track if protocols changed
	if len(oldService.Protocols) != len(newService.Protocols) {
		return true
	}

	// Track if ports changed
	if len(oldService.Ports) != len(newService.Ports) {
		return true
	}

	return false
}

// summarizeApp creates a brief summary of an App-ID entry for logging
func summarizeApp(app *AppIDEntry) string {
	if app == nil {
		return "nil"
	}
	return fmt.Sprintf("%s (risk:%d, proto:%s, ports:%v)",
		app.Name, app.Risk, app.DefaultProtocol, app.DefaultPorts)
}

// summarizeService creates a brief summary of an ISDB service for logging
func summarizeService(service *ISDBServiceEntry) string {
	if service == nil {
		return "nil"
	}
	return fmt.Sprintf("%s (id:%s, proto:%v, ports:%v)",
		service.Name, service.ID, service.Protocols, service.Ports)
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ========================================
// Clone Methods (need to add to cache types)
// ========================================

// Clone creates a deep copy of an App-ID cache
func (c *AppIDCache) Clone() *AppIDCache {
	if c == nil {
		return nil
	}

	clone := &AppIDCache{
		Applications: make(map[string]*AppIDEntry),
		Version:      c.Version,
		LastUpdated:  c.LastUpdated,
		Source:       c.Source,
	}

	// Deep copy applications
	for name, app := range c.Applications {
		appCopy := *app // Shallow copy of entry
		// Deep copy slices
		if len(app.DefaultPorts) > 0 {
			appCopy.DefaultPorts = make([]string, len(app.DefaultPorts))
			copy(appCopy.DefaultPorts, app.DefaultPorts)
		}
		clone.Applications[name] = &appCopy
	}

	return clone
}

// Clone creates a deep copy of an ISDB cache
func (c *ISDBCache) Clone() *ISDBCache {
	if c == nil {
		return nil
	}

	clone := &ISDBCache{
		Services:       make(map[string]*ISDBServiceEntry),
		ServicesByName: make(map[string]*ISDBServiceEntry),
		Version:        c.Version,
		Date:           c.Date,
		Server:         c.Server,
	}

	// Deep copy services
	for id, service := range c.Services {
		serviceCopy := *service // Shallow copy

		// Deep copy slices
		if len(service.Protocols) > 0 {
			serviceCopy.Protocols = make([]string, len(service.Protocols))
			copy(serviceCopy.Protocols, service.Protocols)
		}
		if len(service.Ports) > 0 {
			serviceCopy.Ports = make([]string, len(service.Ports))
			copy(serviceCopy.Ports, service.Ports)
		}
		if len(service.Categories) > 0 {
			serviceCopy.Categories = make([]string, len(service.Categories))
			copy(serviceCopy.Categories, service.Categories)
		}

		clone.Services[id] = &serviceCopy
		clone.ServicesByName[strings.ToLower(service.Name)] = &serviceCopy
	}

	return clone
}

// ========================================
// Merge Update Helper
// ========================================

// UpdateMetadataFromMerge updates cache metadata after a merge operation
func UpdateMetadataFromMerge(metadata *CacheMetadata, fetchResult *CollectionResult, mergeReport *MergeReport) *CacheMetadata {
	if metadata == nil {
		metadata = NewCacheMetadata(
			fetchResult.Metadata.DeviceID,
			fetchResult.Metadata.FirmwareVersion,
		)
	}

	// Update version information
	if fetchResult.Metadata.ContentVersion != "" {
		metadata.ContentVersion = fetchResult.Metadata.ContentVersion
	}

	// Update counts
	if fetchResult.AppIDCache != nil {
		metadata.TotalApplications = mergeReport.NewTotalCount
	} else if fetchResult.ISDBCache != nil {
		metadata.TotalServices = mergeReport.NewTotalCount
	}

	// Create fetch record from collection result
	fetchRecord := fetchResult.ToFetchRecord()

	// Add merge information to fetch record
	fetchRecord.EntriesAdded = len(mergeReport.Added)
	fetchRecord.EntriesUpdated = len(mergeReport.Updated)
	fetchRecord.EntriesRemoved = len(mergeReport.Removed)
	fetchRecord.EntriesUnchanged = mergeReport.Unchanged

	// Add fetch record to metadata
	metadata.AddFetchRecord(fetchRecord)

	// Update staleness
	metadata.UpdateStaleness()

	return metadata
}

// SerializeForDiff serializes an entry to JSON for comparison (debugging)
func SerializeForDiff(entry interface{}) string {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(data)
}
