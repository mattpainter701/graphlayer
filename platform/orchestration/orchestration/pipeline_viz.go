package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PipelineStatus represents the status of a single job in the pipeline.
type PipelineStatus string

const (
	PipelineStatusSuccess  PipelineStatus = "success"
	PipelineStatusFailed   PipelineStatus = "failed"
	PipelineStatusRunning  PipelineStatus = "running"
	PipelineStatusSkipped  PipelineStatus = "skipped"
	PipelineStatusPending  PipelineStatus = "pending"
)

// PipelineRunLog captures the complete execution log of a collection run.
type PipelineRunLog struct {
	RunID       string                `json:"run_id"`
	StartedAt   time.Time             `json:"started_at"`
	CompletedAt time.Time             `json:"completed_at"`
	Duration    time.Duration         `json:"duration"`
	TotalJobs   int                   `json:"total_jobs"`
	Successful  int                   `json:"successful"`
	Failed      int                   `json:"failed"`
	Skipped     int                   `json:"skipped"`
	JobEntries  []*PipelineJobEntry   `json:"job_entries"`
}

// PipelineJobEntry represents a single job's execution record in the pipeline.
type PipelineJobEntry struct {
	JobGUID    string          `json:"job_guid"`
	DeviceType string          `json:"device_type"`
	DeviceID   string          `json:"device_id"`
	Status     PipelineStatus  `json:"status"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    time.Time       `json:"ended_at"`
	Duration   time.Duration   `json:"duration"`
	Attempts   int             `json:"attempts"`
	Error      string          `json:"error,omitempty"`
	Skipped    bool            `json:"skipped"`
	SkipReason string          `json:"skip_reason,omitempty"`
}

// PipelineFormatter generates human-readable pipeline visualizations.
type PipelineFormatter struct {
	ShowTimestamps bool
	ShowDurations  bool
	ColorEnabled   bool
	Width          int
}

// DefaultPipelineFormatter returns a formatter with sensible defaults.
func DefaultPipelineFormatter() *PipelineFormatter {
	return &PipelineFormatter{
		ShowTimestamps: true,
		ShowDurations:  true,
		ColorEnabled:   true,
		Width:          80,
	}
}

// Format generates a human-readable pipeline visualization string.
func (pf *PipelineFormatter) Format(log *PipelineRunLog) string {
	var sb strings.Builder

	// Header
	sb.WriteString(pf.formatHeader(log))
	sb.WriteString("\n")

	// Timeline of jobs
	for i, entry := range log.JobEntries {
		sb.WriteString(pf.formatEntry(i+1, entry))
		sb.WriteString("\n")
	}

	// Summary footer
	sb.WriteString(pf.formatFooter(log))

	return sb.String()
}

// FormatJSON returns the pipeline log as formatted JSON.
func (pf *PipelineFormatter) FormatJSON(log *PipelineRunLog) (string, error) {
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// formatHeader builds the pipeline run header.
func (pf *PipelineFormatter) formatHeader(log *PipelineRunLog) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat("=", pf.Width))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  PIPELINE RUN: %s\n", log.RunID))
	sb.WriteString(fmt.Sprintf("  Started:  %s\n", log.StartedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Finished: %s\n", log.CompletedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Duration: %s\n", log.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("  Jobs: %d total | %d success | %d failed | %d skipped\n",
		log.TotalJobs, log.Successful, log.Failed, log.Skipped))
	sb.WriteString(strings.Repeat("=", pf.Width))
	return sb.String()
}

// formatEntry formats a single pipeline job entry.
func (pf *PipelineFormatter) formatEntry(index int, entry *PipelineJobEntry) string {
	var sb strings.Builder

	statusIcon := pf.statusIcon(entry.Status)
	statusColor := pf.statusColor(entry.Status)

	// Job header line
	sb.WriteString(fmt.Sprintf("  %s [%d] %s — %s",
		statusIcon, index, entry.DeviceID, entry.DeviceType))

	if pf.ShowDurations {
		sb.WriteString(fmt.Sprintf(" (%s)", entry.Duration.Round(time.Millisecond)))
	}
	sb.WriteString("\n")

	// Status line
	sb.WriteString(fmt.Sprintf("       Status:  %s%s%s",
		statusColor, entry.Status, pf.resetColor()))

	if entry.Attempts > 1 {
		sb.WriteString(fmt.Sprintf(" (after %d attempts)", entry.Attempts))
	}
	sb.WriteString("\n")

	// Timestamps
	if pf.ShowTimestamps {
		sb.WriteString(fmt.Sprintf("       Started:  %s\n", entry.StartedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("       Ended:    %s\n", entry.EndedAt.Format(time.RFC3339)))
	}

	// Skipped reason
	if entry.Skipped {
		sb.WriteString(fmt.Sprintf("       Skipped:  %s\n", entry.SkipReason))
	}

	// Error
	if entry.Error != "" {
		sb.WriteString(fmt.Sprintf("       Error:    %s\n", entry.Error))
	}

	return sb.String()
}

// formatFooter builds the pipeline run summary footer.
func (pf *PipelineFormatter) formatFooter(log *PipelineRunLog) string {
	var sb strings.Builder
	sb.WriteString(strings.Repeat("-", pf.Width))
	sb.WriteString("\n")

	successRate := 0.0
	if log.TotalJobs > 0 {
		successRate = float64(log.Successful) / float64(log.TotalJobs) * 100
	}

	sb.WriteString(fmt.Sprintf("  Success Rate: %.1f%%\n", successRate))

	// Per-device-type summary
	typeStats := make(map[string]struct{ total, success, failed, skipped int })
	for _, entry := range log.JobEntries {
		stats := typeStats[entry.DeviceType]
		stats.total++
		switch entry.Status {
		case PipelineStatusSuccess:
			stats.success++
		case PipelineStatusFailed:
			stats.failed++
		case PipelineStatusSkipped:
			stats.skipped++
		}
		typeStats[entry.DeviceType] = stats
	}

	if len(typeStats) > 1 {
		sb.WriteString("  By Device Type:\n")
		for dtype, stats := range typeStats {
			sb.WriteString(fmt.Sprintf("    %s: %d/%d success\n", dtype, stats.success, stats.total))
		}
	}

	sb.WriteString(strings.Repeat("=", pf.Width))
	return sb.String()
}

// statusIcon returns a visual icon for the pipeline status.
func (pf *PipelineFormatter) statusIcon(status PipelineStatus) string {
	if !pf.ColorEnabled {
		switch status {
		case PipelineStatusSuccess:
			return "[OK]"
		case PipelineStatusFailed:
			return "[FAIL]"
		case PipelineStatusSkipped:
			return "[SKIP]"
		case PipelineStatusRunning:
			return "[...]"
		default:
			return "[ ? ]"
		}
	}

	switch status {
	case PipelineStatusSuccess:
		return "✅"
	case PipelineStatusFailed:
		return "❌"
	case PipelineStatusSkipped:
		return "⏭️"
	case PipelineStatusRunning:
		return "🔄"
	default:
		return "❓"
	}
}

// statusColor returns ANSI color codes for terminal rendering.
func (pf *PipelineFormatter) statusColor(status PipelineStatus) string {
	if !pf.ColorEnabled {
		return ""
	}
	switch status {
	case PipelineStatusSuccess:
		return "\033[32m" // Green
	case PipelineStatusFailed:
		return "\033[31m" // Red
	case PipelineStatusSkipped:
		return "\033[33m" // Yellow
	case PipelineStatusRunning:
		return "\033[36m" // Cyan
	default:
		return ""
	}
}

// resetColor returns the ANSI reset code.
func (pf *PipelineFormatter) resetColor() string {
	if !pf.ColorEnabled {
		return ""
	}
	return "\033[0m"
}

// Visualize formats the current pipeline log and returns a human-readable string.
func (log *PipelineRunLog) Visualize() string {
	return DefaultPipelineFormatter().Format(log)
}

// VisualizeJSON returns the pipeline log as JSON.
func (log *PipelineRunLog) VisualizeJSON() (string, error) {
	return DefaultPipelineFormatter().FormatJSON(log)
}

// Timeline builds a chronological timeline of all job entries.
func (log *PipelineRunLog) Timeline() []*PipelineJobEntry {
	// Already in order, return a copy
	entries := make([]*PipelineJobEntry, len(log.JobEntries))
	copy(entries, log.JobEntries)
	return entries
}

// FailedJobs returns only the failed job entries.
func (log *PipelineRunLog) FailedJobs() []*PipelineJobEntry {
	var failed []*PipelineJobEntry
	for _, entry := range log.JobEntries {
		if entry.Status == PipelineStatusFailed {
			failed = append(failed, entry)
		}
	}
	return failed
}

// SuccessfulJobs returns only the successful job entries.
func (log *PipelineRunLog) SuccessfulJobs() []*PipelineJobEntry {
	var ok []*PipelineJobEntry
	for _, entry := range log.JobEntries {
		if entry.Status == PipelineStatusSuccess {
			ok = append(ok, entry)
		}
	}
	return ok
}

// SkippedJobs returns only the skipped job entries.
func (log *PipelineRunLog) SkippedJobs() []*PipelineJobEntry {
	var skipped []*PipelineJobEntry
	for _, entry := range log.JobEntries {
		if entry.Status == PipelineStatusSkipped {
			skipped = append(skipped, entry)
		}
	}
	return skipped
}
