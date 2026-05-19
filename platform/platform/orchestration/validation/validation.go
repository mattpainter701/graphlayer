// Package validation provides topology validation and error reporting utilities.
package validation

import (
	"fmt"
	"strings"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

// ValidationError describes a single problem found during topology validation.
type ValidationError struct {
	Type    string `json:"type"`    // "node" or "edge"
	ID      string `json:"id"`      // the affected node/edge identifier
	Field   string `json:"field"`   // the field that failed validation
	Message string `json:"message"` // human-readable description of the problem
}

// ValidateTopology runs all validation checks against the supplied topology and
// returns every problem found.  An empty slice means the topology is valid.
//
// Checks performed:
//   - Required fields (ID, Name, Type) are non-empty on every node
//   - No duplicate node IDs
//   - Confidence scores lie in the [0, 1] interval (unset / zero is permitted)
//   - Every edge Source / Target references an existing node ID
func ValidateTopology(topology models.Topology) []ValidationError {
	var errs []ValidationError

	// ---- nodes ---------------------------------------------------------------
	nodeIdx := make(map[string]int) // ID -> position in slice (for duplicate detection)

	for i := range topology.Nodes {
		n := &topology.Nodes[i]
		id := strings.TrimSpace(n.ID)

		// required fields
		if id == "" {
			errs = append(errs, ValidationError{
				Type: "node", ID: id, Field: "ID",
				Message: "Node ID is required and must not be empty",
			})
		}
		if strings.TrimSpace(n.Name) == "" {
			errs = append(errs, ValidationError{
				Type: "node", ID: id, Field: "Name",
				Message: "Node Name is required and must not be empty",
			})
		}
		if strings.TrimSpace(n.Type) == "" {
			errs = append(errs, ValidationError{
				Type: "node", ID: id, Field: "Type",
				Message: "Node Type is required and must not be empty",
			})
		}

		// duplicate detection
		if id != "" {
			if prev, ok := nodeIdx[id]; ok {
				errs = append(errs, ValidationError{
					Type: "node", ID: id, Field: "ID",
					Message: fmt.Sprintf("Duplicate node ID %q (first seen at index %d)", id, prev),
				})
			} else {
				nodeIdx[id] = i
			}
		}

		// confidence range
		if n.Confidence < 0 || n.Confidence > 1 {
			errs = append(errs, ValidationError{
				Type: "node", ID: id, Field: "Confidence",
				Message: fmt.Sprintf("Confidence score %.3f is outside [0, 1]", n.Confidence),
			})
		}
	}

	// ---- edges ---------------------------------------------------------------
	for i := range topology.Edges {
		e := &topology.Edges[i]
		id := strings.TrimSpace(e.ID)

		// source reference
		if strings.TrimSpace(e.Source) != "" {
			if _, ok := nodeIdx[strings.TrimSpace(e.Source)]; !ok {
				errs = append(errs, ValidationError{
					Type: "edge", ID: id, Field: "Source",
					Message: fmt.Sprintf("Edge references non-existent source node %q", e.Source),
				})
			}
		} else {
			errs = append(errs, ValidationError{
				Type: "edge", ID: id, Field: "Source",
				Message: "Edge source is empty",
			})
		}

		// target reference
		if strings.TrimSpace(e.Target) != "" {
			if _, ok := nodeIdx[strings.TrimSpace(e.Target)]; !ok {
				errs = append(errs, ValidationError{
					Type: "edge", ID: id, Field: "Target",
					Message: fmt.Sprintf("Edge references non-existent target node %q", e.Target),
				})
			}
		} else {
			errs = append(errs, ValidationError{
				Type: "edge", ID: id, Field: "Target",
				Message: "Edge target is empty",
			})
		}

		// confidence range
		if e.Confidence < 0 || e.Confidence > 1 {
			errs = append(errs, ValidationError{
				Type: "edge", ID: id, Field: "Confidence",
				Message: fmt.Sprintf("Confidence score %.3f is outside [0, 1]", e.Confidence),
			})
		}

		// optional: edge required fields (ID is useful for reporting)
		if id == "" {
			errs = append(errs, ValidationError{
				Type: "edge", ID: id, Field: "ID",
				Message: "Edge ID is empty (makes error reporting harder)",
			})
		}
	}

	return errs
}
