package investigation

import (
	"errors"
	"net/http"

	"github.com/mattpainter701/graphlayer/platform/policy"
)

func (a *App) evaluatePath(w http.ResponseWriter, r *http.Request, snapshotID string) {
	var req struct {
		IngressInterface string `json:"ingress_interface"`
		Flow             struct {
			Source          string `json:"source"`
			Destination     string `json:"destination"`
			Protocol        string `json:"protocol"`
			SourcePort      int    `json:"source_port"`
			DestinationPort int    `json:"destination_port"`
		} `json:"flow"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	flow := policy.Flow{Source: req.Flow.Source, Destination: req.Flow.Destination, Protocol: req.Flow.Protocol, SourcePort: req.Flow.SourcePort, DestinationPort: req.Flow.DestinationPort}
	inv, err := a.store.EvaluatePath(snapshotID, req.IngressInterface, flow)
	if analysisUnavailable(w, err) {
		return
	}
	switch {
	case errors.Is(err, ErrPathUnavailable):
		apiError(w, 409, "path_unavailable", "Reimport the original configuration to create path metadata; older snapshots are preserved.")
	case errors.Is(err, ErrInvalidFlow):
		apiError(w, 400, "invalid_path", "Select a modeled ingress interface and use a concrete IPv4 flow with valid protocol and ports.")
	case errors.Is(err, ErrNotFound):
		apiError(w, 404, "scope_not_found", "The selected snapshot is unavailable.")
	case errors.Is(err, ErrQuota):
		apiError(w, 429, "workspace_full", "The local investigation history has reached its limit.")
	case errors.Is(err, ErrRecordTooLarge):
		apiError(w, 413, "path_too_large", "The path trace exceeds the artifact budget. No partial investigation was saved.")
	case err != nil:
		apiError(w, 500, "workspace_write_failed", "The local workspace could not save this path investigation.")
	default:
		writeJSON(w, 200, map[string]any{"investigation": inv})
	}
}
