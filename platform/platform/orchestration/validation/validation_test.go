package validation_test

import (
	"testing"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
	"github.com/mattpainter701/graphlayer/platform/platform/orchestration/validation"
)

func TestValidateTopology_ValidEmpty(t *testing.T) {
	topo := models.Topology{}
	errs := validation.ValidateTopology(topo)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors for empty topology, got %d: %+v", len(errs), errs)
	}
}

func TestValidateTopology_ValidSimple(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "n1", Name: "Router1", Type: "router", Confidence: 0.95},
			{ID: "n2", Name: "Switch1", Type: "switch", Confidence: 0.80},
		},
		Edges: []models.Edge{
			{ID: "e1", Source: "n1", Target: "n2", Confidence: 0.99},
		},
	}
	errs := validation.ValidateTopology(topo)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d: %+v", len(errs), errs)
	}
}

func TestValidateTopology_MissingRequiredFields(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "", Name: "", Type: ""}, // all three missing
			{ID: "n2", Name: "", Type: "switch"}, // name missing
		},
	}
	errs := validation.ValidateTopology(topo)
	if len(errs) < 3 {
		t.Fatalf("expected at least 3 required-field errors, got %d: %+v", len(errs), errs)
	}
	// verify we have errors for each empty field
	fields := make(map[string]bool)
	for _, e := range errs {
		fields[e.Field] = true
	}
	for _, f := range []string{"ID", "Name", "Type"} {
		if !fields[f] {
			t.Errorf("expected an error for field %q", f)
		}
	}
}

func TestValidateTopology_DuplicateNodeIDs(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "dup", Name: "First", Type: "router"},
			{ID: "dup", Name: "Second", Type: "switch"},
		},
	}
	errs := validation.ValidateTopology(topo)
	found := false
	for _, e := range errs {
		if e.Field == "ID" && e.ID == "dup" && e.Type == "node" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a duplicate-ID error for node 'dup', got: %+v", errs)
	}
}

func TestValidateTopology_ConfidenceOutOfRange(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "n1", Name: "R1", Type: "router", Confidence: -0.5},
			{ID: "n2", Name: "R2", Type: "router", Confidence: 1.5},
			{ID: "n3", Name: "R3", Type: "router", Confidence: 0.5}, // ok
		},
		Edges: []models.Edge{
			{ID: "e1", Source: "n1", Target: "n2", Confidence: 2.0},
			{ID: "e2", Source: "n1", Target: "n3", Confidence: -0.1},
		},
	}
	errs := validation.ValidateTopology(topo)
	// Expect: n1 (-0.5), n2 (1.5), e1 (2.0), e2 (-0.1) = 4 out-of-range errors
	outOfRange := 0
	for _, e := range errs {
		if e.Field == "Confidence" {
			outOfRange++
		}
	}
	if outOfRange < 4 {
		t.Fatalf("expected at least 4 confidence errors, got %d: %+v", outOfRange, errs)
	}
}

func TestValidateTopology_MissingEdgeReferences(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "n1", Name: "R1", Type: "router"},
			{ID: "n2", Name: "R2", Type: "router"},
		},
		Edges: []models.Edge{
			{ID: "e1", Source: "n1", Target: "n3"},       // n3 missing
			{ID: "e2", Source: "ghost", Target: "n2"},    // ghost missing
			{ID: "e3", Source: "n1", Target: "n2"},       // ok
		},
	}
	errs := validation.ValidateTopology(topo)
	missingRefs := 0
	for _, e := range errs {
		if e.Field == "Source" || e.Field == "Target" {
			missingRefs++
		}
	}
	if missingRefs < 2 {
		t.Fatalf("expected at least 2 missing-reference errors, got %d: %+v", missingRefs, errs)
	}
}

func TestValidateTopology_EdgeEmptySourceOrTarget(t *testing.T) {
	topo := models.Topology{
		Nodes: []models.Node{
			{ID: "n1", Name: "R1", Type: "router"},
		},
		Edges: []models.Edge{
			{ID: "e1", Source: "", Target: "n1"},
			{ID: "e2", Source: "n1", Target: ""},
		},
	}
	errs := validation.ValidateTopology(topo)
	emptySrcDst := 0
	for _, e := range errs {
		if (e.Field == "Source" || e.Field == "Target") && e.Type == "edge" {
			emptySrcDst++
		}
	}
	if emptySrcDst < 2 {
		t.Fatalf("expected 2 empty source/target errors, got %d: %+v", emptySrcDst, errs)
	}
}
