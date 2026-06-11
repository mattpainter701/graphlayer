// Command graphlayer-configdiff compares two config snapshots and reports changes.
//
// Usage:
//
//	graphlayer-configdiff --old=<old.json> --new=<new.json> [--json] [--log=<log.json>]
//	graphlayer-configdiff --self-test
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mattpainter701/graphlayer/platform/configdiff"
)

func main() {
	oldPath := flag.String("old", "", "Path to old config snapshot JSON")
	newPath := flag.String("new", "", "Path to new config snapshot JSON")
	jsonOut := flag.Bool("json", false, "Output as JSON")
	logPath := flag.String("log", "", "Append change log to file (JSON lines)")
	selfTest := flag.Bool("self-test", false, "Run self-test with synthetic data")
	flag.Parse()

	if *selfTest {
		runSelfTest(*jsonOut)
		return
	}

	if *oldPath == "" || *newPath == "" {
		fmt.Fprintln(os.Stderr, "Both --old and --new are required")
		flag.Usage()
		os.Exit(1)
	}

	oldData, err := os.ReadFile(*oldPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading old snapshot: %v\n", err)
		os.Exit(1)
	}
	newData, err := os.ReadFile(*newPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading new snapshot: %v\n", err)
		os.Exit(1)
	}

	oldSnap, err := configdiff.LoadSnapshot(oldData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing old snapshot: %v\n", err)
		os.Exit(1)
	}
	newSnap, err := configdiff.LoadSnapshot(newData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing new snapshot: %v\n", err)
		os.Exit(1)
	}

	result := configdiff.Diff(oldSnap, newSnap)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		printTextReport(result)
	}

	if *logPath != "" {
		appendLog(*logPath, result)
	}
}

func printTextReport(r *configdiff.DiffResult) {
	fmt.Printf("Config Diff: %s → %s\n", r.OldRoundID, r.NewRoundID)
	fmt.Printf("Time range: %s → %s\n", r.OldTime.Format(time.RFC3339), r.NewTime.Format(time.RFC3339))
	fmt.Println("---")
	fmt.Printf("Summary: %d added, %d removed, %d modified, %d total changes\n",
		r.Summary.DevicesAdded, r.Summary.DevicesRemoved, r.Summary.DevicesModified, r.Summary.TotalChanges)
	fmt.Println()
	for _, c := range r.Changes {
		switch c.Type {
		case configdiff.ChangeTypeAdded:
			fmt.Printf("  [+] %s (%s)\n", c.DeviceID, c.Hostname)
		case configdiff.ChangeTypeRemoved:
			fmt.Printf("  [-] %s (%s)\n", c.DeviceID, c.Hostname)
		case configdiff.ChangeTypeHashChange:
			fmt.Printf("  [~] %s: config changed (%s → %s)\n", c.DeviceID, shortHash(c.OldValue), shortHash(c.NewValue))
		case configdiff.ChangeTypeVLANChange:
			if c.OldValue == "" {
				fmt.Printf("  [~] %s: %s → %s\n", c.DeviceID, c.Field, c.NewValue)
			} else if c.NewValue == "" {
				fmt.Printf("  [~] %s: %s → (removed)\n", c.DeviceID, c.Field)
			} else {
				fmt.Printf("  [~] %s: %s %s → %s\n", c.DeviceID, c.Field, c.OldValue, c.NewValue)
			}
		case configdiff.ChangeTypeIFChange:
			if c.OldValue == "" {
				fmt.Printf("  [~] %s: %s added (%s)\n", c.DeviceID, c.Field, c.NewValue)
			} else if c.NewValue == "" {
				fmt.Printf("  [~] %s: %s removed\n", c.DeviceID, c.Field)
			} else {
				fmt.Printf("  [~] %s: %s %s → %s\n", c.DeviceID, c.Field, c.OldValue, c.NewValue)
			}
		default:
			fmt.Printf("  [?] %s: %s %s → %s\n", c.DeviceID, c.Field, c.OldValue, c.NewValue)
		}
	}
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8] + "…"
	}
	return h
}

func appendLog(path string, r *configdiff.DiffResult) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot open log file: %v\n", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"old_round": r.OldRoundID,
		"new_round": r.NewRoundID,
		"changes":   r.Summary,
	}
	enc.Encode(entry)
}

func runSelfTest(jsonOut bool) {
	now := time.Now().UTC()
	old := &configdiff.ConfigSnapshot{
		RoundID:   "baseline",
		Timestamp: now.Add(-time.Hour),
		Devices: map[string]configdiff.DeviceConfigSnapshot{
			"sw-core-01": {
				DeviceID: "sw-core-01", Hostname: "sw-core-01", Vendor: "cisco",
				OSName: "IOS-XE", OSVersion: "17.6", ConfigHash: configdiff.ComputeConfigHash("old-config"),
				VLANS: map[int]configdiff.VLANDiff{10: {ID: 10, Name: "DATA", State: "active"}},
				Interfaces: []configdiff.InterfaceDiff{{Name: "Gi0/1", AdminUp: true, Mode: "access"}},
			},
		},
	}
	new := &configdiff.ConfigSnapshot{
		RoundID:   "current",
		Timestamp: now,
		Devices: map[string]configdiff.DeviceConfigSnapshot{
			"sw-core-01": {
				DeviceID: "sw-core-01", Hostname: "sw-core-01", Vendor: "cisco",
				OSName: "IOS-XE", OSVersion: "17.6", ConfigHash: configdiff.ComputeConfigHash("new-config"),
				VLANS: map[int]configdiff.VLANDiff{10: {ID: 10, Name: "DATA", State: "active"}, 20: {ID: 20, Name: "VOICE", State: "active"}},
				Interfaces: []configdiff.InterfaceDiff{{Name: "Gi0/1", AdminUp: false, Mode: "trunk"}},
			},
			"sw-access-02": {
				DeviceID: "sw-access-02", Hostname: "sw-access-02", Vendor: "cisco",
				OSName: "IOS", OSVersion: "15.2", ConfigHash: configdiff.ComputeConfigHash("access-config"),
			},
		},
	}

	result := configdiff.Diff(old, new)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Println("=== Self-Test: Config Diff ===")
		printTextReport(result)
		fmt.Println()
		fmt.Println("Expected: 1 added device, 1 modified device, 4 changes (added+hash+vlan+if)")
		if result.Summary.DevicesAdded != 1 || result.Summary.DevicesModified != 1 {
			fmt.Println("FAIL: summary mismatch")
			os.Exit(1)
		}
		if result.Summary.TotalChanges != 4 {
			fmt.Printf("FAIL: expected 4 total changes, got %d\n", result.Summary.TotalChanges)
			os.Exit(1)
		}
		fmt.Println("PASS")
	}
}
