package investigation

import "github.com/mattpainter701/graphlayer/platform/rustengine"

func validateInventory(snapshot *Snapshot) error {
	return rustengine.Validate("validate_inventory", map[string]any{"config": snapshot})
}
