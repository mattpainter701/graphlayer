package scoring

import (
	"math"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// DecayConfidence reduces the node's confidence score linearly over time.
// currentTime and decayHours are expressed in hours (float64).
// At lastSeen the stored confidence is kept; after decayHours it becomes 0.
func DecayConfidence(node *models.TopologyNode, currentTime, decayHours float64) {
	if node == nil || node.LastSeen.IsZero() || decayHours <= 0 {
		return
	}
	lastSeenHours := float64(node.LastSeen.Unix()) / 3600.0
	elapsed := currentTime - lastSeenHours
	if elapsed <= 0 {
		return
	}
	if elapsed >= decayHours {
		node.Confidence = 0
		return
	}
	factor := 1.0 - (elapsed / decayHours)
	node.Confidence = math.Max(0, node.Confidence*factor)
}

// ApplyDecayToSlice applies DecayConfidence to every node in the slice.
// currentTime and decayHours are the same for all nodes.
func ApplyDecayToSlice(nodes []models.TopologyNode, currentTime, decayHours float64) {
	for i := range nodes {
		DecayConfidence(&nodes[i], currentTime, decayHours)
	}
}
