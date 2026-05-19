package scoring

import (
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/platform/models"
)

func TestDecayConfidence_ExactlyAtLastSeen(t *testing.T) {
	node := &models.TopologyNode{
		Confidence: 80.0,
		LastSeen:   time.Unix(3600*10, 0), // 10 hours after epoch
	}

	// currentTime = 10 hours -> elapsed = 0
	DecayConfidence(node, 10.0, 24.0)
	if node.Confidence != 80.0 {
		t.Errorf("expected confidence 80.0, got %f", node.Confidence)
	}
}

func TestDecayConfidence_HalfDecayHours(t *testing.T) {
	node := &models.TopologyNode{
		Confidence: 80.0,
		LastSeen:   time.Unix(3600*10, 0),
	}

	// currentTime = 22 hours -> elapsed = 12, half of 24
	DecayConfidence(node, 22.0, 24.0)
	expected := 40.0
	if node.Confidence != expected {
		t.Errorf("expected confidence %f, got %f", expected, node.Confidence)
	}
}

func TestDecayConfidence_AtDecayHours(t *testing.T) {
	node := &models.TopologyNode{
		Confidence: 80.0,
		LastSeen:   time.Unix(3600*10, 0),
	}

	// currentTime = 34 hours -> elapsed = 24
	DecayConfidence(node, 34.0, 24.0)
	if node.Confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", node.Confidence)
	}
}

func TestDecayConfidence_BeyondDecayHours(t *testing.T) {
	node := &models.TopologyNode{
		Confidence: 80.0,
		LastSeen:   time.Unix(3600*10, 0),
	}

	// currentTime = 50 hours -> elapsed = 40
	DecayConfidence(node, 50.0, 24.0)
	if node.Confidence != 0.0 {
		t.Errorf("expected confidence 0.0, got %f", node.Confidence)
	}
}

func TestDecayConfidence_ZeroLastSeen(t *testing.T) {
	node := &models.TopologyNode{
		Confidence: 80.0,
		// LastSeen zero value
	}
	DecayConfidence(node, 100.0, 24.0)
	if node.Confidence != 80.0 {
		t.Errorf("expected confidence unchanged 80.0, got %f", node.Confidence)
	}
}

func TestDecayConfidence_NilNode(t *testing.T) {
	// Should not panic
	DecayConfidence(nil, 100.0, 24.0)
}

func TestApplyDecayToSlice(t *testing.T) {
	nodes := []models.TopologyNode{
		{Confidence: 100.0, LastSeen: time.Unix(3600*0, 0)},
		{Confidence: 100.0, LastSeen: time.Unix(3600*12, 0)},
	}
	currentTime := float64(time.Now().Unix()) / 3600.0
	ApplyDecayToSlice(nodes, currentTime, 24.0)
	// Just ensure it runs without panicking and confidence values are altered
	// (specific values depend on currentTime – we only check that they are >= 0)
	for i, n := range nodes {
		if n.Confidence < 0 || n.Confidence > 100 {
			t.Errorf("node %d confidence out of [0,100]: %f", i, n.Confidence)
		}
	}
}
