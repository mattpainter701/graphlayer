package investigation

import "github.com/mattpainter701/graphlayer/platform/rustengine"

func validatePathConfig(snapshot *Snapshot) error {
	return rustengine.Validate("validate_path_config", map[string]any{"config": snapshot})
}
