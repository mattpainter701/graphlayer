package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Schedule defines a recurring collection schedule using cron-like syntax.
// Fields are minute, hour, day of month, month, day of week (0=Sunday).
// "*" means every. A nil Schedule means run-once (not recurring).
type Schedule struct {
	Minute     string `json:"minute"`      // 0-59, comma-separated, */N, or *
	Hour       string `json:"hour"`        // 0-23
	DayOfMonth string `json:"day_of_month"` // 1-31
	Month      string `json:"month"`        // 1-12
	DayOfWeek  string `json:"day_of_week"`   // 0-6 (0=Sunday)
}

// IsRecurring returns true if the schedule represents a cron-style recurring pattern.
func (s *Schedule) IsRecurring() bool {
	if s == nil {
		return false
	}
	return s.Minute != "" || s.Hour != "" || s.DayOfMonth != "" || s.Month != "" || s.DayOfWeek != ""
}

// Scheduler manages recurring connector collection jobs with cron-style scheduling.
// Each schedule entry maps to a set of connector jobs that are executed together.
type Scheduler struct {
	orchestrator *ConnectorOrchestrator
	entries      map[string]*SchedulerEntry
	config        *SchedulerConfig
	mu            sync.RWMutex
	stopCh        chan struct{}
	running       bool
	healthMonitor *HealthMonitor
}

// SchedulerConfig configures the scheduler.
type SchedulerConfig struct {
	// DefaultRunConfig applied to all scheduled runs.
	DefaultRunConfig *RunConfig `json:"default_run_config"`

	// CheckInterval is how often the scheduler checks for due entries.
	CheckInterval time.Duration `json:"check_interval"`

	// MaxConcurrentRuns limits parallel scheduled runs (across different entries).
	MaxConcurrentRuns int `json:"max_concurrent_runs"`

	// RunTimeout per scheduled run; 0 means no timeout.
	RunTimeout time.Duration `json:"run_timeout"`

	// EnableHealthMonitoring enables per-run health tracking.
	EnableHealthMonitoring bool `json:"enable_health_monitoring"`
}

// DefaultSchedulerConfig returns sensible scheduler defaults.
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		DefaultRunConfig:       DefaultRunConfig(),
		CheckInterval:          60 * time.Second,
		MaxConcurrentRuns:      3,
		RunTimeout:             30 * time.Minute,
		EnableHealthMonitoring: true,
	}
}

// SchedulerEntry represents a named scheduled collection group.
type SchedulerEntry struct {
	Name        string           `json:"name"`
	Schedule    *Schedule        `json:"schedule"`
	Jobs        []*ConnectorJob  `json:"jobs"`
	RunConfig   *RunConfig       `json:"run_config,omitempty"`
	Enabled     bool             `json:"enabled"`
	LastRunAt   *time.Time       `json:"last_run_at,omitempty"`
	LastRunID   string           `json:"last_run_id,omitempty"`
	NextRunAt   *time.Time       `json:"next_run_at,omitempty"`
	LastResult  *RunResult       `json:"last_result,omitempty"`
	RunCount    int              `json:"run_count"`
	ErrorCount  int              `json:"error_count"`
}

// NewScheduler creates a new scheduler with the given orchestrator and config.
func NewScheduler(orchestrator *ConnectorOrchestrator, config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	var hm *HealthMonitor
	if config.EnableHealthMonitoring {
		hm = NewHealthMonitor()
	}

	return &Scheduler{
		orchestrator:  orchestrator,
		entries:       make(map[string]*SchedulerEntry),
		config:        config,
		stopCh:        make(chan struct{}),
		healthMonitor: hm,
	}
}

// AddEntry registers a scheduled collection entry.
func (s *Scheduler) AddEntry(entry *SchedulerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Name == "" {
		return fmt.Errorf("scheduler entry name is required")
	}
	if _, exists := s.entries[entry.Name]; exists {
		return fmt.Errorf("scheduler entry %q already exists", entry.Name)
	}

	// Calculate next run time
	if entry.Schedule.IsRecurring() && entry.Enabled {
		nextRun := calculateNextRun(entry.Schedule, time.Now())
		entry.NextRunAt = &nextRun
	}

	s.entries[entry.Name] = entry
	return nil
}

// RemoveEntry removes a scheduled collection entry.
func (s *Scheduler) RemoveEntry(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[name]; !exists {
		return fmt.Errorf("scheduler entry %q not found", name)
	}
	delete(s.entries, name)
	return nil
}

// GetEntry returns a scheduler entry by name.
func (s *Scheduler) GetEntry(name string) (*SchedulerEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, exists := s.entries[name]
	if !exists {
		return nil, fmt.Errorf("scheduler entry %q not found", name)
	}
	return entry, nil
}

// ListEntries returns all scheduler entries.
func (s *Scheduler) ListEntries() []*SchedulerEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := make([]*SchedulerEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Start begins the scheduler loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.loop(ctx)
	return nil
}

// Stop halts the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

// IsRunning returns whether the scheduler is active.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// RunNow triggers immediate execution of a named entry.
func (s *Scheduler) RunNow(ctx context.Context, name string) (*RunResult, error) {
	s.mu.RLock()
	entry, exists := s.entries[name]
	s.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("scheduler entry %q not found", name)
	}
	return s.executeEntry(ctx, entry)
}

// loop is the main scheduler loop, checking for due entries periodically.
func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Run concurrency limiter
	runSem := make(chan struct{}, s.config.MaxConcurrentRuns)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRun(ctx, runSem)
		}
	}
}

// checkAndRun checks for due entries and runs them.
func (s *Scheduler) checkAndRun(ctx context.Context, runSem chan struct{}) {
	s.mu.RLock()
	var dueEntries []*SchedulerEntry
	now := time.Now()
	for _, entry := range s.entries {
		if !entry.Enabled {
			continue
		}
		if entry.Schedule.IsRecurring() && entry.NextRunAt != nil && !now.Before(*entry.NextRunAt) {
			dueEntries = append(dueEntries, entry)
		}
	}
	s.mu.RUnlock()

	for _, entry := range dueEntries {
		// Non-blocking semaphore acquisition
		select {
		case runSem <- struct{}{}:
			go func(e *SchedulerEntry) {
				defer func() { <-runSem }()
				s.executeEntry(ctx, e)
			}(entry)
		default:
			// Max concurrent runs reached, skip this tick
			fmt.Printf("[WARN] Scheduler: max concurrent runs (%d) reached, deferring entry %q\n",
				s.config.MaxConcurrentRuns, entry.Name)
		}
	}
}

// executeEntry runs a scheduled entry's jobs and records the result.
func (s *Scheduler) executeEntry(ctx context.Context, entry *SchedulerEntry) (*RunResult, error) {
	now := time.Now()

	cfg := entry.RunConfig
	if cfg == nil {
		cfg = s.config.DefaultRunConfig
	}

	// Apply run timeout if configured
	var runCtx context.Context
	var cancel context.CancelFunc
	if s.config.RunTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, s.config.RunTimeout)
		defer cancel()
	} else {
		runCtx = ctx
	}

	result, err := s.orchestrator.RunAll(runCtx, entry.Jobs, cfg)

	s.mu.Lock()
	entry.LastRunAt = &now
	entry.RunCount++
	if err != nil || (result != nil && result.Failed > 0) {
		entry.ErrorCount++
	}
	if result != nil {
		entry.LastRunID = result.RunID
		entry.LastResult = result
	}

	// Schedule next run
	if entry.Schedule.IsRecurring() {
		nextRun := calculateNextRun(entry.Schedule, now)
		entry.NextRunAt = &nextRun
	} else {
		// One-shot: disable after running
		entry.Enabled = false
		entry.NextRunAt = nil
	}
	s.mu.Unlock()

	// Feed health monitor
	if s.healthMonitor != nil && result != nil {
		s.healthMonitor.RecordSchedulerRun(entry.Name, result)
	}

	return result, err
}

// calculateNextRun computes the next run time based on a cron-like schedule.
func calculateNextRun(schedule *Schedule, from time.Time) time.Time {
	if schedule == nil || !schedule.IsRecurring() {
		return time.Time{}
	}

	// Simple implementation: advance by specified intervals
	// For production, use a full cron parser library
	next := from.Add(1 * time.Minute) // Start at next minute boundary

	// Parse minute field
	if schedule.Minute != "" && schedule.Minute != "*" {
		if schedule.Minute == "0" {
			// Round to next hour if we're past minute 0
			if from.Minute() >= 0 {
				next = next.Truncate(time.Hour).Add(time.Hour)
			} else {
				next = next.Truncate(time.Hour)
			}
		}
		// "/N" pattern: every N minutes
		if len(schedule.Minute) > 2 && schedule.Minute[:2] == "*/" {
			// For simplicity, assume next minute interval
			next = from.Add(1 * time.Minute)
		}
	}

	// Parse hour field
	if schedule.Hour != "" && schedule.Hour != "*" {
		// Specified hour(s) — round to next matching hour
		if schedule.Hour == "0" {
			// Midnight: truncate to start of next day
			next = next.Truncate(24 * time.Hour).Add(24 * time.Hour)
		}
	}

	// For hourly runs
	if schedule.Minute == "0" && schedule.Hour == "*" {
		next = from.Truncate(time.Hour).Add(time.Hour)
	}

	return next
}

// GetHealthMonitor returns the scheduler's health monitor.
func (s *Scheduler) GetHealthMonitor() *HealthMonitor {
	return s.healthMonitor
}

// marshalSchedule returns a human-readable description of the schedule.
func marshalSchedule(s *Schedule) string {
	if s == nil || !s.IsRecurring() {
		return "once"
	}
	return fmt.Sprintf("%s %s %s %s %s",
		s.Minute, s.Hour, s.DayOfMonth, s.Month, s.DayOfWeek)
}

// GetScheduler returns the scheduler (convenience accessor).
func (co *ConnectorOrchestrator) GetScheduler() *Scheduler {
	// The orchestrator may or may not own a scheduler — this is a convenience helper.
	// In practice, orchestrator.scheduler is set externally via NewScheduler.
	return nil
}
