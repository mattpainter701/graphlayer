package models

import "time"

// TopologyNode represents a node in the network topology.
type TopologyNode struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Confidence float64   `json:"confidence"`
	LastSeen    time.Time `json:"last_seen"`
}
