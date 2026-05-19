package diff

// DiffTopologies computes the differences between two topologies.
func DiffTopologies(old, new Topology) DiffResult {
	// Build lookup maps for nodes
	oldNodes := map[string]Node{}
	for _, n := range old.Nodes {
		oldNodes[n.ID] = n
	}
	newNodes := map[string]Node{}
	for _, n := range new.Nodes {
		newNodes[n.ID] = n
	}

	var nodeDiffs []NodeDiff

	// Added and changed nodes
	for id, nn := range newNodes {
		on, exists := oldNodes[id]
		if !exists {
			nodeDiffs = append(nodeDiffs, NodeDiff{ID: id, ChangeType: Added, NewConfidence: nn.Confidence})
		} else if on.Confidence != nn.Confidence {
			nodeDiffs = append(nodeDiffs, NodeDiff{
				ID: id, ChangeType: Changed,
				OldConfidence: on.Confidence, NewConfidence: nn.Confidence,
			})
		}
	}

	// Removed nodes
	for id, on := range oldNodes {
		if _, exists := newNodes[id]; !exists {
			nodeDiffs = append(nodeDiffs, NodeDiff{ID: id, ChangeType: Removed, OldConfidence: on.Confidence})
		}
	}

	// Build lookup maps for edges
	oldEdges := map[string]Edge{}
	for _, e := range old.Edges {
		oldEdges[e.ID] = e
	}
	newEdges := map[string]Edge{}
	for _, e := range new.Edges {
		newEdges[e.ID] = e
	}

	var edgeDiffs []EdgeDiff

	// Added and changed edges
	for id, ne := range newEdges {
		oe, exists := oldEdges[id]
		if !exists {
			edgeDiffs = append(edgeDiffs, EdgeDiff{ID: id, ChangeType: Added, NewConfidence: ne.Confidence})
		} else if oe.Confidence != ne.Confidence {
			edgeDiffs = append(edgeDiffs, EdgeDiff{
				ID: id, ChangeType: Changed,
				OldConfidence: oe.Confidence, NewConfidence: ne.Confidence,
			})
		}
	}

	// Removed edges
	for id, oe := range oldEdges {
		if _, exists := newEdges[id]; !exists {
			edgeDiffs = append(edgeDiffs, EdgeDiff{ID: id, ChangeType: Removed, OldConfidence: oe.Confidence})
		}
	}

	return DiffResult{
		NodeDiffs: nodeDiffs,
		EdgeDiffs: edgeDiffs,
	}
}
