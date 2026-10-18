package investigation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

const (
	maxImportItems          = 8
	maxImportPreviews       = 4
	importPreviewTTL        = 15 * time.Minute
	maxPreviewWarnings      = 20
	maxPreviewSourceLine    = 60
	maxPreviewLineBytes     = 512
	maxPreviewSnapshotBytes = 8 << 20
)

type importPreviewItem struct {
	Name                   string                `json:"name"`
	Status                 string                `json:"status"`
	ErrorCode              string                `json:"error_code,omitempty"`
	SnapshotID             string                `json:"snapshot_id,omitempty"`
	Vendor                 string                `json:"vendor,omitempty"`
	Hostname               string                `json:"hostname,omitempty"`
	Context                string                `json:"context,omitempty"`
	ParserVersion          string                `json:"parser_version,omitempty"`
	ChainCount             int                   `json:"chain_count"`
	RuleCount              int                   `json:"rule_count"`
	WarningCount           int                   `json:"warning_count"`
	Warnings               []firewall.Warning    `json:"warnings"`
	Capabilities           []string              `json:"capabilities"`
	SourcePreview          []firewall.SourceLine `json:"source_preview"`
	SourceLineCount        int                   `json:"source_line_count"`
	SourcePreviewTruncated bool                  `json:"source_preview_truncated"`
	Existing               bool                  `json:"existing"`
}

type importPreviewResponse struct {
	ID         string              `json:"id"`
	CreatedAt  time.Time           `json:"created_at"`
	ExpiresAt  time.Time           `json:"expires_at"`
	Valid      bool                `json:"valid"`
	Items      []importPreviewItem `json:"items"`
	TotalBytes int                 `json:"total_bytes"`
}

type importPreview struct {
	response         importPreviewResponse
	snapshots        []Snapshot
	publishedName    string
	publishedSite    string
	publishedSet     *SnapshotSet
	publishedCreated bool
}

func (a *App) batchImportAPI(w http.ResponseWriter, r *http.Request, parts []string) bool {
	if len(parts) == 2 && parts[1] == "import-previews" {
		if r.Method != http.MethodPost {
			methodError(w)
			return true
		}
		a.createImportPreview(w, r)
		return true
	}
	if len(parts) == 4 && parts[1] == "import-previews" && validID(parts[2]) && parts[3] == "publish" {
		if r.Method != http.MethodPost {
			methodError(w)
			return true
		}
		a.publishImportPreview(w, r, parts[2])
		return true
	}
	return false
}

func (a *App) createImportPreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Name   string `json:"name"`
			Config string `json:"config"`
			Vendor string `json:"vendor"`
		} `json:"items"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Items) < 1 || len(req.Items) > maxImportItems {
		apiError(w, http.StatusBadRequest, "invalid_preview", "Provide between one and eight configuration files.")
		return
	}
	total := 0
	for _, item := range req.Items {
		total += len(item.Config)
		if len(item.Config) > maxConfigBytes || total > maxConfigBytes {
			apiError(w, http.StatusRequestEntityTooLarge, "config_too_large", "The combined configuration input is limited to 2 MiB.")
			return
		}
	}
	select {
	case a.importing <- struct{}{}:
	default:
		apiError(w, http.StatusTooManyRequests, "import_busy", "Another import is running. Try again shortly.")
		return
	}
	defer func() { <-a.importing }()

	now := time.Now().UTC()
	preview := &importPreview{response: importPreviewResponse{CreatedAt: now, ExpiresAt: now.Add(importPreviewTTL), Valid: true, Items: make([]importPreviewItem, 0, len(req.Items)), TotalBytes: total}}
	seen := make(map[string]bool)
	encodedTotal := 0
	for _, item := range req.Items {
		out, snap := a.previewItem(item.Name, item.Config, item.Vendor, seen)
		if out.Status == "ready" && snap != nil {
			encoded, err := json.Marshal(snap)
			if err != nil || encodedTotal+len(encoded) > maxPreviewSnapshotBytes {
				out.Status, out.ErrorCode = "invalid", "snapshot_too_large"
				snap = nil
			} else {
				encodedTotal += len(encoded)
				preview.snapshots = append(preview.snapshots, *snap)
				seen[snap.ID] = true
			}
		}
		if out.Status != "ready" {
			preview.response.Valid = false
		}
		preview.response.Items = append(preview.response.Items, out)
	}
	id, err := randomToken()
	if err != nil {
		apiError(w, http.StatusInternalServerError, "preview_failed", "The import preview could not be initialized.")
		return
	}
	preview.response.ID = id
	a.previewMu.Lock()
	a.prunePreviewsLocked(now)
	if len(a.previews) >= maxImportPreviews {
		a.evictOldestPreviewLocked()
	}
	a.previews[id] = preview
	a.previewMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"preview": preview.response})
}

func (a *App) previewItem(name, raw, vendor string, seen map[string]bool) (importPreviewItem, *Snapshot) {
	out := importPreviewItem{Name: name, Status: "invalid", Warnings: []firewall.Warning{}, Capabilities: []string{}, SourcePreview: []firewall.SourceLine{}}
	if !validUploadName(name) || strings.TrimSpace(raw) == "" || !utf8.ValidString(raw) {
		out.ErrorCode = "invalid_import"
		return out, nil
	}
	parsed, err := firewall.ParseConfig([]byte(raw), vendor)
	if err != nil || parsed == nil || len(parsed.Chains) == 0 || len(parsed.SourceLines) == 0 || parsed.ArtifactSHA256 != digest([]byte(raw)) {
		out.ErrorCode = "invalid_config"
		return out, nil
	}
	lines, err := json.Marshal(parsed.SourceLines)
	if err != nil {
		out.ErrorCode = "invalid_config"
		return out, nil
	}
	snap := &Snapshot{ID: snapshotID(*parsed), CreatedAt: time.Now().UTC(), Name: name, SanitizedSHA256: digest(lines), Config: *parsed}
	if err = validateSnapshot(snap); err != nil {
		out.ErrorCode = "invalid_config"
		return out, nil
	}
	if seen[snap.ID] {
		out.Status, out.ErrorCode, out.SnapshotID = "duplicate", "duplicate_in_batch", snap.ID
		return out, nil
	}
	if existing, err := a.store.Snapshot(snap.ID); err == nil {
		// Reuse the immutable stored parse while retaining this upload's filename
		// as the set member label. The stored snapshot record remains untouched.
		reusable := *existing
		reusable.Name = name
		snap = &reusable
		out.Existing = true
	}
	out.Status, out.SnapshotID = "ready", snap.ID
	out.Vendor, out.Hostname, out.Context, out.ParserVersion = snap.Vendor, snap.Hostname, snap.Context, snap.ParserVersion
	out.ChainCount = len(snap.Chains)
	out.SourceLineCount = len(snap.SourceLines)
	out.Capabilities = append([]string{}, snap.Capabilities...)
	warnings := append([]firewall.Warning{}, snap.Warnings...)
	rules := 0
	for _, chain := range snap.Chains {
		rules += len(chain.Rules)
		warnings = append(warnings, chain.Warnings...)
	}
	out.RuleCount = rules
	out.WarningCount = len(warnings)
	if len(warnings) > maxPreviewWarnings {
		warnings = warnings[:maxPreviewWarnings]
	}
	out.Warnings = warnings
	lineCount := len(snap.SourceLines)
	if lineCount > maxPreviewSourceLine {
		lineCount = maxPreviewSourceLine
		out.SourcePreviewTruncated = true
	}
	out.SourcePreview = make([]firewall.SourceLine, 0, lineCount)
	for _, line := range snap.SourceLines[:lineCount] {
		if len(line.Text) > maxPreviewLineBytes {
			out.SourcePreviewTruncated = true
		}
		line.Text = truncatePreviewLine(line.Text, maxPreviewLineBytes)
		out.SourcePreview = append(out.SourcePreview, line)
	}
	return out, snap
}

func truncatePreviewLine(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxBytes-3 {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "..."
}

func validUploadName(name string) bool {
	if !utf8.ValidString(name) || len(name) < 1 || len(name) > 128 || strings.TrimSpace(name) != name {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (a *App) publishImportPreview(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		SetName          string `json:"set_name"`
		SiteID           string `json:"site_id"`
		ExpectedRevision *int   `json:"expected_revision"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ExpectedRevision == nil || *req.ExpectedRevision < 0 || !validLabel(req.SetName, 1, 80) || (req.SiteID != "" && !validID(req.SiteID)) {
		apiError(w, http.StatusBadRequest, "invalid_preview", "Provide a set name, site, and nonnegative expected revision.")
		return
	}
	a.previewMu.Lock()
	defer a.previewMu.Unlock()
	a.prunePreviewsLocked(time.Now().UTC())
	preview := a.previews[id]
	if preview == nil {
		if _, expired := a.expiredPreviews[id]; expired {
			apiError(w, http.StatusGone, "preview_expired", "This preview has expired. Create a new preview.")
		} else {
			apiError(w, http.StatusNotFound, "preview_not_found", "This import preview is unavailable.")
		}
		return
	}
	if preview.publishedSet != nil {
		if preview.publishedName != req.SetName || preview.publishedSite != req.SiteID {
			apiError(w, http.StatusConflict, "workspace_conflict", "This preview was already published with a different set name or site.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"snapshot_set": preview.publishedSet, "workspace": a.store.WorkspaceInfo(), "created": preview.publishedCreated})
		return
	}
	if !preview.response.Valid || len(preview.snapshots) != len(preview.response.Items) {
		apiError(w, http.StatusBadRequest, "invalid_preview", "Every file in the preview must be valid before publishing.")
		return
	}
	set, created, err := a.store.PublishSnapshotSet(req.SetName, req.SiteID, preview.snapshots, *req.ExpectedRevision)
	if errors.Is(err, ErrWorkspaceConflict) {
		apiError(w, http.StatusConflict, "workspace_conflict", "The workspace changed or this set name is already in use. Reload and try again.")
		return
	}
	if err != nil {
		workspaceStoreError(w, err)
		return
	}
	preview.publishedName, preview.publishedSite = req.SetName, req.SiteID
	preview.publishedSet, preview.publishedCreated = set, created
	writeJSON(w, http.StatusCreated, map[string]any{"snapshot_set": set, "workspace": a.store.WorkspaceInfo(), "created": created})
}

func (a *App) prunePreviewsLocked(now time.Time) {
	if a.expiredPreviews == nil {
		a.expiredPreviews = make(map[string]time.Time)
	}
	for id, preview := range a.previews {
		if !now.Before(preview.response.ExpiresAt) {
			delete(a.previews, id)
			a.expiredPreviews[id] = preview.response.ExpiresAt
		}
	}
	for id, expiry := range a.expiredPreviews {
		if now.After(expiry.Add(24 * time.Hour)) {
			delete(a.expiredPreviews, id)
		}
	}
	for len(a.expiredPreviews) > 32 {
		for id := range a.expiredPreviews {
			delete(a.expiredPreviews, id)
			break
		}
	}
}

func (a *App) evictOldestPreviewLocked() {
	var oldestID string
	var oldest time.Time
	for id, preview := range a.previews {
		if oldestID == "" || preview.response.CreatedAt.Before(oldest) {
			oldestID, oldest = id, preview.response.CreatedAt
		}
	}
	if oldestID != "" {
		delete(a.previews, oldestID)
	}
}
