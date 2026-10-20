// Command graphlayer-compare runs the reference comparison harness.
// Sprint 10: Per-release comparison tool.
//
// Usage:
//
//	graphlayer-compare --graphlayer=<reference.json> --reference=<reference.json> [--report=report.md]
//	graphlayer-compare --ref=<ref.json> --reference=<reference.json> [--report=report.md]
//	graphlayer-compare --self-test
//
// When --self-test is passed, it runs a comparison against the built-in
// multi-vendor reference topology and prints the report to stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/mattpainter701/graphlayer/platform/comparison"
)

func main() {
	graphLayerPath := flag.String("graphlayer", "", "Path to GraphLayer reference-format JSON export")
	referencePath := flag.String("reference", "", "Path to reference-format JSON export")
	refPath := flag.String("ref", "", "Path to reference topology JSON (use as GraphLayer baseline)")
	reportPath := flag.String("report", "", "Path to write markdown report (default: stdout)")
	selfTest := flag.Bool("self-test", false, "Run self-test using built-in multi-vendor reference")
	jsonOut := flag.Bool("json", false, "Output results as JSON instead of markdown")
	glVersion := flag.String("gl-version", "dev", "GraphLayer version string")
	referenceVersion := flag.String("reference-version", "reference", "reference version string")
	flag.Parse()

	if *selfTest {
		runSelfTest(*jsonOut, *glVersion, *referenceVersion)
		return
	}

	var glTopo, referenceTopo *comparison.ReferenceExportTopology

	// Load GraphLayer topology
	if *graphLayerPath != "" {
		var err error
		glTopo, err = loadReferenceTopology(*graphLayerPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading GraphLayer topology: %v\n", err)
			os.Exit(1)
		}
	} else if *refPath != "" {
		ref, err := comparison.LoadReferenceTopology(*refPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading reference topology: %v\n", err)
			os.Exit(1)
		}
		glTopo = &ref.Topology
	} else {
		fmt.Fprintln(os.Stderr, "Either --graphlayer or --ref is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load reference topology
	if *referencePath != "" {
		var err error
		referenceTopo, err = loadReferenceTopology(*referencePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading reference topology: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "--reference is required")
		flag.Usage()
		os.Exit(1)
	}

	// Run comparison
	metrics := comparison.CompareTopologies(glTopo, referenceTopo)
	report := comparison.GenerateReport(
		fmt.Sprintf("GraphLayer %s vs reference Comparison", *glVersion),
		*glVersion,
		*referenceVersion,
		metrics,
	)

	// Output
	var output string
	if *jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		output = string(data)
	} else {
		output = report.RenderMarkdown()
	}

	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, []byte(output), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Report written to %s\n", *reportPath)
	} else {
		fmt.Print(output)
	}
}

func runSelfTest(jsonOut bool, glVersion, referenceVersion string) {
	ref := comparison.NewMultiVendorReferenceTopology()

	metrics := comparison.CompareTopologies(&ref.Topology, &ref.Topology)
	report := comparison.GenerateReport(
		fmt.Sprintf("GraphLayer %s vs reference — Self-Test", glVersion),
		glVersion,
		referenceVersion,
		metrics,
	)

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Print(report.RenderMarkdown())
	}
}

func loadReferenceTopology(path string) (*comparison.ReferenceExportTopology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var topo comparison.ReferenceExportTopology
	if err := json.Unmarshal(data, &topo); err != nil {
		return nil, err
	}
	return &topo, nil
}
