package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mattpainter701/graphlayer/platform/orchestration/diff"
)

func main() {
	jsonFlag := flag.Bool("json", false, "output as JSON")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: graphlayer diff [--json] <old_topology.json> <new_topology.json>\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	oldPath := args[0]
	newPath := args[1]

	oldT, err := loadTopology(oldPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading old topology: %v\n", err)
		os.Exit(1)
	}
	newT, err := loadTopology(newPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading new topology: %v\n", err)
		os.Exit(1)
	}

	result := diff.DiffTopologies(oldT, newT)

	if *jsonFlag {
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
		return
	}

	printTable(result)
}

func loadTopology(path string) (diff.Topology, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return diff.Topology{}, err
	}
	var t diff.T
