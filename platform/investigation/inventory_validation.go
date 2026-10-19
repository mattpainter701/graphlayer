package investigation

import "github.com/mattpainter701/graphlayer/platform/rustengine"

func validateInventory(snapshot *Snapshot) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	return rustengine.Evaluate("validate_inventory", map[string]any{"config": snapshot}, &result)
}
