package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/orchestration"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	querySource          string
	queryDeviceName      string
	queryDeviceType      string
	queryConfidence      float64
	queryOutputFormat    string
)

// queryCmd represents the graphlayer query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query network topology",
	Long: `Query the network topology with optional filters for source, device name, device type, and confidence threshold.

Examples:
  graphlayer query --source cisco --device-name router1 --confidence 0.8
  graphlayer query --source paloalto --output yaml
  graphlayer query --type router --output json
  graphlayer query --type switch --source cisco
  graphlayer query --confidence 0.5`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runQuery()
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.Flags().StringVarP(&querySource, "source", "s", "", "Filter by source vendor (e.g., cisco, paloalto, fortinet)")
	queryCmd.Flags().StringVarP(&queryDeviceName, "device-name", "d", "", "Filter by device name")
	queryCmd.Flags().StringVarP(&queryDeviceType, "type", "t", "", "Filter by device type (router, switch, firewall)")
	queryCmd.Flags().Float64VarP(&queryConfidence, "confidence", "c", 0.0, "Minimum confidence threshold (0.0 - 1.0)")
	queryCmd.Flags().StringVarP(&queryOutputFormat, "output", "o", "json", "Output format (json or yaml)")
}

func runQuery() error {
	// Create orchestration engine
	engine := orchestration.NewOrchestrationEngine()

	// Build query filter
	filter := orchestration.TopologyFilter{
		Source:     querySource,
		DeviceName: queryDeviceName,
		DeviceType: queryDeviceType,
		Confidence: queryConfidence,
	}

	// Query topology
	topology, err := engine.QueryTopology(filter)
	if err != nil {
		return fmt.Errorf("failed to query topology: %w", err)
	}

	// Output results
	switch queryOutputFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(topology); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
	case "yaml":
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		if err := encoder.Encode(topology); err != nil {
			return fmt.Errorf("failed to encode YAML output: %w", err)
		}
	default:
		return fmt.Errorf("unsupported output format: %s (use json or yaml)", queryOutputFormat)
	}

	return nil
}
