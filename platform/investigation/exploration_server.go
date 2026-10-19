package investigation

import (
	"errors"
	"net/http"
)

func (a *App) explorationAPI(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if len(parts) == 2 && parts[1] == "network-explorations" {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, map[string]any{"explorations": a.store.ExplorationSummaries()})
		case http.MethodPost:
			a.exploreNetwork(w, r)
		default:
			methodError(w)
		}
		return true
	}
	if len(parts) == 3 && parts[1] == "network-explorations" && validID(parts[2]) {
		if r.Method != http.MethodGet {
			methodError(w)
			return true
		}
		inv, err := a.store.NetworkExploration(parts[2])
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected network exploration is unavailable.")
		} else {
			writeJSON(w, 200, map[string]any{"exploration": inv})
		}
		return true
	}
	return false
}

func (a *App) exploreNetwork(w http.ResponseWriter, r *http.Request) {
	var req NetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	inv, err := a.store.ExploreNetwork(req)
	if analysisUnavailable(w, err) {
		return
	}
	switch {
	case errors.Is(err, ErrPathUnavailable):
		apiError(w, 409, "path_unavailable", "Reimport older snapshots to create path metadata.")
	case errors.Is(err, ErrInvalidFlow):
		apiError(w, 400, "invalid_network_path", "Use bounded snapshot-pinned topology and a concrete IPv4 flow.")
	case errors.Is(err, ErrNotFound):
		apiError(w, 404, "scope_not_found", "A selected snapshot is unavailable.")
	case errors.Is(err, ErrQuota):
		apiError(w, 429, "workspace_full", "The shared local investigation history has reached its limit.")
	case errors.Is(err, ErrRecordTooLarge):
		apiError(w, 413, "path_too_large", "The complete exploration exceeds the artifact budget. No partial record was saved.")
	case err != nil:
		apiError(w, 500, "workspace_write_failed", "The local workspace could not save this exploration.")
	default:
		writeJSON(w, 200, map[string]any{"exploration": inv})
	}
}
