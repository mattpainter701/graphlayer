package investigation

import (
	"errors"
	"net/http"
)

func (a *App) networkAPI(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if len(parts) == 3 && parts[1] == "sample" && parts[2] == "network" {
		if r.Method != http.MethodGet {
			methodError(w)
		} else {
			writeJSON(w, 200, networkSample())
		}
		return true
	}
	if len(parts) == 2 && parts[1] == "network-investigations" {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"investigations": a.store.NetworkSummaries()})
		case http.MethodPost:
			a.evaluateNetwork(w, r)
		default:
			methodError(w)
		}
		return true
	}
	if len(parts) == 3 && parts[1] == "network-investigations" && validID(parts[2]) {
		if r.Method != http.MethodGet {
			methodError(w)
			return true
		}
		inv, err := a.store.NetworkInvestigation(parts[2])
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected multi-firewall investigation is unavailable.")
		} else {
			writeJSON(w, 200, map[string]any{"investigation": inv})
		}
		return true
	}
	return false
}

func (a *App) evaluateNetwork(w http.ResponseWriter, r *http.Request) {
	var req NetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	inv, err := a.store.EvaluateNetwork(req)
	switch {
	case errors.Is(err, ErrPathUnavailable):
		apiError(w, 409, "path_unavailable", "Reimport the original configuration for each older snapshot to create path metadata.")
	case errors.Is(err, ErrInvalidFlow):
		apiError(w, 400, "invalid_network_path", "Use 1–16 distinct snapshots, unique node IDs, at most 32 explicit links, modeled interface endpoints and a concrete IPv4 flow.")
	case errors.Is(err, ErrNotFound):
		apiError(w, 404, "scope_not_found", "A selected snapshot is unavailable.")
	case errors.Is(err, ErrQuota):
		apiError(w, 429, "workspace_full", "The shared local investigation history has reached its limit.")
	case errors.Is(err, ErrRecordTooLarge):
		apiError(w, 413, "path_too_large", "The complete network trace exceeds the artifact budget. No partial investigation was saved.")
	case err != nil:
		apiError(w, 500, "workspace_write_failed", "The local workspace could not save this multi-firewall investigation.")
	default:
		writeJSON(w, 200, map[string]any{"investigation": inv})
	}
}
