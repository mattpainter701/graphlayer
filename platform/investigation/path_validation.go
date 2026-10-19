package investigation

import "github.com/mattpainter701/graphlayer/platform/rustengine"

func validatePathConfig(snapshot *Snapshot) error {
	var result struct {
		Valid bool `json:"valid"`
	}
	return rustengine.Evaluate("validate_path_config", map[string]any{"config": snapshot}, &result)
}
