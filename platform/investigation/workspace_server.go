package investigation

import (
	"errors"
	"net/http"
)

func (a *App) workspaceAPI(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if len(parts) < 2 || parts[1] != "workspace" && parts[1] != "sites" && parts[1] != "snapshot-sets" {
		return false
	}
	if len(parts) == 2 && parts[1] == "workspace" {
		switch r.Method {
		case http.MethodGet:
			workspace, sets := a.store.WorkspaceOverview()
			writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace, "snapshot_sets": sets})
		case http.MethodPut:
			var req struct {
				Name             string `json:"name"`
				ExpectedRevision *int   `json:"expected_revision"`
			}
			if !decodeJSON(w, r, &req) {
				return true
			}
			if req.ExpectedRevision == nil || *req.ExpectedRevision < 0 {
				apiError(w, http.StatusBadRequest, "invalid_workspace", "A nonnegative expected revision is required.")
				return true
			}
			workspace, err := a.store.RenameWorkspace(req.Name, *req.ExpectedRevision)
			if workspaceStoreError(w, err) {
				return true
			}
			writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace})
		default:
			methodError(w)
		}
		return true
	}
	if len(parts) == 2 && parts[1] == "sites" {
		if r.Method != http.MethodPost {
			methodError(w)
			return true
		}
		var req struct {
			Name             string `json:"name"`
			Description      string `json:"description"`
			ExpectedRevision *int   `json:"expected_revision"`
		}
		if !decodeJSON(w, r, &req) {
			return true
		}
		if req.ExpectedRevision == nil || *req.ExpectedRevision < 0 {
			apiError(w, http.StatusBadRequest, "invalid_workspace", "A nonnegative expected revision is required.")
			return true
		}
		site, err := a.store.CreateSite(req.Name, req.Description, *req.ExpectedRevision)
		if workspaceStoreError(w, err) {
			return true
		}
		writeJSON(w, http.StatusCreated, map[string]any{"site": site, "workspace": a.store.WorkspaceInfo()})
		return true
	}
	if parts[1] == "snapshot-sets" {
		if len(parts) == 2 {
			if r.Method != http.MethodGet {
				methodError(w)
				return true
			}
			writeJSON(w, http.StatusOK, map[string]any{"snapshot_sets": a.store.SnapshotSets()})
			return true
		}
		if len(parts) == 3 && validID(parts[2]) {
			if r.Method != http.MethodGet {
				methodError(w)
				return true
			}
			set, err := a.store.SnapshotSet(parts[2])
			if err != nil {
				apiError(w, http.StatusNotFound, "snapshot_set_not_found", "The selected snapshot set is unavailable.")
				return true
			}
			writeJSON(w, http.StatusOK, map[string]any{"snapshot_set": set})
			return true
		}
		if len(parts) == 4 && validID(parts[2]) && parts[3] == "activate" {
			if r.Method != http.MethodPost {
				methodError(w)
				return true
			}
			var req struct {
				ExpectedRevision *int `json:"expected_revision"`
			}
			if !decodeJSON(w, r, &req) {
				return true
			}
			if req.ExpectedRevision == nil || *req.ExpectedRevision < 0 {
				apiError(w, http.StatusBadRequest, "invalid_workspace", "A nonnegative expected revision is required.")
				return true
			}
			workspace, err := a.store.ActivateSnapshotSet(parts[2], *req.ExpectedRevision)
			if workspaceStoreError(w, err) {
				return true
			}
			set, err := a.store.SnapshotSet(parts[2])
			if err != nil {
				apiError(w, http.StatusNotFound, "snapshot_set_not_found", "The selected snapshot set is unavailable.")
				return true
			}
			writeJSON(w, http.StatusOK, map[string]any{"workspace": workspace, "snapshot_set": set})
			return true
		}
		apiError(w, http.StatusNotFound, "not_found", "This resource is unavailable.")
		return true
	}
	apiError(w, http.StatusNotFound, "not_found", "This resource is unavailable.")
	return true
}

func workspaceStoreError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrWorkspaceConflict):
		apiError(w, http.StatusConflict, "workspace_conflict", "The workspace changed or the name is already in use. Reload and try again.")
	case errors.Is(err, ErrWorkspaceInvalid):
		apiError(w, http.StatusBadRequest, "invalid_workspace", "The workspace name, site, or set details are invalid.")
	case errors.Is(err, ErrNotFound):
		apiError(w, http.StatusNotFound, "snapshot_set_not_found", "The selected snapshot set is unavailable.")
	case errors.Is(err, ErrQuota):
		apiError(w, http.StatusTooManyRequests, "workspace_full", "The local workspace has reached its limit.")
	case errors.Is(err, ErrRecordTooLarge):
		apiError(w, http.StatusRequestEntityTooLarge, "snapshot_too_large", "The expanded snapshot exceeds the 8 MiB artifact budget.")
	default:
		apiError(w, http.StatusInternalServerError, "workspace_write_failed", "The local workspace could not save this change.")
	}
	return true
}
