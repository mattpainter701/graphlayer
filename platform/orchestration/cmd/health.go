package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattpainter701/graphlayer/platform/orchestration/orchestration"
	"github.com/spf13/cobra"
)

var healthInput string

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Generate a health check summary across vendor connectors",
	Long: `Read connector health data from a JSON file and display a formatted
health summary table showing last-collection timestamps, error counts,
and status per vendor.

The input JSON must match the HealthReport format produced by HealthMonitor.GenerateReport().`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHealth(cmd)
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.Flags().StringVar(&healthInput, "input", "", "Path to health report JSON file")
	_ = healthCmd.MarkFlagRequired("input")
}

func runHealth(cmd *cobra.Command) error {
	data, err := os.ReadFile(healthInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var report orchestration.HealthReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("failed to parse health report JSON: %w", err)
	}

	out := cmd.OutOrStdout()

	// Summary header
	fmt.Fprintf(out, "=== Health Check Summary ===\n")
	fmt.Fprintf(out, "Timestamp:     %s\n", report.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(out, "Connectors:    %d total\n", report.TotalConnectors)
	fmt.Fprintf(out, "Status:        %s\n", report.Summary)
	fmt.Fprintf(out, "\n")

	// Table header
	fmt.Fprintf(out, "%-20s %-12s %-10s %-8s %-15s %-25s\n",
		"VENDOR", "DEVICE_ID", "SUCCESS%", "RUNS", "LAST_SUCCESS", "ERROR")
	fmt.Fprintf(out, "%s\n", strings.Repeat("-", 95))

	// Sort connectors by device type for consistent output
	type sortedHealth struct {
		GUID   string
		Health *orchestration.ConnectorHealth
	}
	sorted := make([]sortedHealth, 0, len(report.Connectors))
	for guid, h := range report.Connectors {
		sorted = append(sorted, sortedHealth{GUID: guid, Health: h})
	}
	// Simple sort by device type
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Health.DeviceType > sorted[j].Health.DeviceType {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for _, s := range sorted {
		h := s.Health
		lastSuccess := "never"
		if h.LastSuccessAt != nil {
			lastSuccess = h.LastSuccessAt.Format("2006-01-02 15:04")
		}

		errMsg := h.LastError
		if len(errMsg) > 22 {
			errMsg = errMsg[:22] + ".."
		}

		fmt.Fprintf(out, "%-20s %-12s %6.1f%% %5d %15s %-25s\n",
			h.DeviceType,
			h.DeviceID,
			h.SuccessRate*100,
			h.TotalRuns,
			lastSuccess,
			errMsg,
		)
	}

	// Footer with degraded/failed counts
	fmt.Fprintf(out, "\nHealthy: %d | Degraded: %d | Failed: %d | Overall: %.1f%%\n",
		report.HealthyCount,
		report.DegradedCount,
		report.FailedCount,
		report.OverallSuccess*100,
	)

	return nil
}
