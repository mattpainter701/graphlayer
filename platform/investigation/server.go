package investigation

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
	"github.com/mattpainter701/graphlayer/platform/rustengine"
)

//go:embed web/*
var assets embed.FS

const maxConfigBytes = 2 << 20
const maxRequestBytes = 4 << 20
const sessionCookie = "graphlayer_local_session"

type App struct {
	store           *Store
	host            string
	session         string
	csrf            string
	importing       chan struct{}
	previewMu       sync.Mutex
	previews        map[string]*importPreview
	expiredPreviews map[string]time.Time
	static          fs.FS
}

// ValidateListenAddress accepts explicit numeric loopback addresses only.
// This app intentionally has no public bind or multi-user deployment mode.
func ValidateListenAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	n, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || n < 0 || n > 65535 {
		return errors.New("listen address must be an explicit loopback IP and numeric port")
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() || ip.Zone() != "" || ip.Is4In6() {
		return errors.New("only native numeric loopback listen addresses are supported")
	}
	return nil
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func NewHandler(store *Store, host string) (*App, error) {
	if store == nil {
		return nil, errors.New("workspace is required")
	}
	if err := ValidateListenAddress(host); err != nil {
		return nil, err
	}
	session, err := randomToken()
	if err != nil {
		return nil, errors.New("cannot initialize local session")
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, errors.New("cannot initialize local session")
	}
	static, err := fs.Sub(assets, "web")
	if err != nil {
		return nil, err
	}
	return &App{store: store, host: host, session: session, csrf: csrf, importing: make(chan struct{}, 1), previews: make(map[string]*importPreview), expiredPreviews: make(map[string]time.Time), static: static}, nil
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; font-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	if !a.trustedRequest(r) {
		apiError(w, http.StatusForbidden, "local_access", "Use this app's loopback address in the same browser origin.")
		return
	}
	if r.URL.Path == "/api/session" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: a.session, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, 200, map[string]any{"csrf_token": a.csrf})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(a.session)) != 1 {
			apiError(w, 403, "session_required", "Reload the app to initialize local browser access.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("Origin") != "http://"+a.host || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(a.csrf)) != 1 {
				apiError(w, 403, "request_verification", "Request verification failed. Reload the app and try again.")
				return
			}
		}
		a.api(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodError(w)
		return
	}
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	path = strings.TrimPrefix(path, "/assets")
	contentTypes := map[string]string{
		"/index.html":   "text/html; charset=utf-8",
		"/app.js":       "text/javascript; charset=utf-8",
		"/shell.js":     "text/javascript; charset=utf-8",
		"/topology.js":  "text/javascript; charset=utf-8",
		"/d3.min.js":    "text/javascript; charset=utf-8",
		"/style.css":    "text/css; charset=utf-8",
		"/shell.css":    "text/css; charset=utf-8",
		"/topology.css": "text/css; charset=utf-8",
	}
	contentType, allowed := contentTypes[path]
	if !allowed {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(a.static, strings.TrimPrefix(path, "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(200)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (a *App) trustedRequest(r *http.Request) bool {
	if r.Host != a.host {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !ip.IsLoopback() {
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "http" || u.Host != a.host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return false
		}
	}
	return true
}

func (a *App) api(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 2 && parts[1] == "topology-view" {
		if r.Method != http.MethodPost {
			methodError(w)
		} else {
			a.projectTopology(w, r)
		}
		return
	}
	if a.workspaceAPI(w, r, parts) || a.batchImportAPI(w, r, parts) {
		return
	}
	if a.networkAPI(w, r, parts) {
		return
	}
	if len(parts) == 3 && parts[1] == "sample" && parts[2] == "path" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		writeJSON(w, 200, map[string]string{"name": "routing-nat.asa.txt", "config": string(firewall.SamplePathASA())})
		return
	}
	if len(parts) == 2 && parts[1] == "path-investigations" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"investigations": a.store.PathSummaries()})
		return
	}
	if len(parts) == 3 && parts[1] == "path-investigations" && validID(parts[2]) {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		inv, err := a.store.PathInvestigation(parts[2])
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected path investigation is unavailable.")
			return
		}
		writeJSON(w, 200, map[string]any{"investigation": inv})
		return
	}
	if len(parts) == 3 && parts[1] == "sample" && parts[2] == "services" {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		writeJSON(w, 200, map[string]string{"name": "services.asa.txt", "config": string(firewall.SampleServicesASA())})
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "sample":
			if r.Method != http.MethodGet {
				methodError(w)
				return
			}
			writeJSON(w, 200, map[string]string{"name": "enterprise.asa.txt", "config": string(firewall.SampleASA())})
			return
		case "snapshots":
			if r.Method != http.MethodGet {
				methodError(w)
				return
			}
			writeJSON(w, 200, map[string]any{"snapshots": a.store.Summaries()})
			return
		case "investigations":
			if r.Method != http.MethodGet {
				methodError(w)
				return
			}
			writeJSON(w, 200, map[string]any{"investigations": a.store.Investigations()})
			return
		case "imports":
			if r.Method != http.MethodPost {
				methodError(w)
				return
			}
			a.importConfig(w, r)
			return
		}
	}
	if len(parts) >= 3 && parts[1] == "snapshots" && validID(parts[2]) {
		if len(parts) == 4 && parts[3] == "path-evaluate" {
			if r.Method != http.MethodPost {
				methodError(w)
				return
			}
			a.evaluatePath(w, r, parts[2])
			return
		}
		if len(parts) == 4 && parts[3] == "evaluate" {
			if r.Method != http.MethodPost {
				methodError(w)
				return
			}
			a.evaluate(w, r, parts[2])
			return
		}
		if len(parts) == 3 {
			if r.Method != http.MethodGet {
				methodError(w)
				return
			}
			snap, err := a.store.Snapshot(parts[2])
			if err != nil {
				apiError(w, 404, "snapshot_not_found", "The selected snapshot is unavailable.")
				return
			}
			writeJSON(w, 200, map[string]any{"snapshot": snap})
			return
		}
	}
	if len(parts) == 3 && parts[1] == "investigations" && validID(parts[2]) {
		if r.Method != http.MethodGet {
			methodError(w)
			return
		}
		inv, err := a.store.Investigation(parts[2])
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected investigation is unavailable.")
			return
		}
		writeJSON(w, 200, map[string]any{"investigation": inv})
		return
	}
	apiError(w, 404, "not_found", "This resource is unavailable.")
}

// Topology views are disposable projections. Saved requests, source evidence and
// investigation identities are never rewritten by presentation preferences.
type topologyViewRequest struct {
	Request         NetworkRequest    `json:"request"`
	ExplorationID   string            `json:"exploration_id,omitempty"`
	InvestigationID string            `json:"investigation_id,omitempty"`
	BranchIndex     int               `json:"branch_index,omitempty"`
	ReturnIndex     int               `json:"return_index,omitempty"`
	Roles           map[string]string `json:"roles,omitempty"`
}

func (a *App) projectTopology(w http.ResponseWriter, r *http.Request) {
	var view topologyViewRequest
	if !decodeJSON(w, r, &view) {
		return
	}
	if view.ExplorationID != "" && view.InvestigationID != "" || view.BranchIndex < 0 || view.ReturnIndex < 0 || len(view.Roles) > 18 {
		apiError(w, 400, "invalid_topology_view", "Use one saved investigation and bounded presentation preferences.")
		return
	}
	var result any
	if view.ExplorationID != "" {
		if !validID(view.ExplorationID) {
			apiError(w, 400, "invalid_topology_view", "Use a saved exploration identity.")
			return
		}
		saved, err := a.store.NetworkExploration(view.ExplorationID)
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected exploration is unavailable.")
			return
		}
		view.Request, result = saved.Request, saved.Result
	} else if view.InvestigationID != "" {
		if !validID(view.InvestigationID) {
			apiError(w, 400, "invalid_topology_view", "Use a saved investigation identity.")
			return
		}
		saved, err := a.store.NetworkInvestigation(view.InvestigationID)
		if err != nil {
			apiError(w, 404, "investigation_not_found", "The selected investigation is unavailable.")
			return
		}
		view.Request, result = saved.Request, saved.Result
	}
	if len(view.Request.Nodes) == 0 || len(view.Request.Nodes) > 16 || len(view.Request.Links) > 32 {
		apiError(w, 400, "invalid_topology_view", "Use 1–16 snapshot-pinned devices and at most 32 links.")
		return
	}
	// Only server-owned sanitized snapshots enter the worker. Clients submit
	// snapshot references, never replacement configs, evidence or traffic counters.
	nodes := make([]map[string]any, 0, len(view.Request.Nodes))
	for _, ref := range view.Request.Nodes {
		if !validID(ref.SnapshotID) || !validNetworkNodeID(ref.NodeID) {
			apiError(w, 400, "invalid_topology_view", "Select a saved snapshot and a valid ID for every device.")
			return
		}
		snapshot, err := a.store.Snapshot(ref.SnapshotID)
		if err != nil {
			apiError(w, 404, "snapshot_not_found", "A selected snapshot is unavailable.")
			return
		}
		nodes = append(nodes, map[string]any{"id": ref.NodeID, "scope": pathInput(snapshot, "", policy.Flow{}).Scope, "config": snapshot.Config})
	}
	input := map[string]any{
		"request": map[string]any{"nodes": nodes, "links": view.Request.Links, "source": view.Request.Source, "destination": view.Request.Destination, "flow": view.Request.Flow},
		"result":  result, "branch_index": view.BranchIndex, "return_index": view.ReturnIndex, "roles": view.Roles,
	}
	if view.Roles == nil {
		input["roles"] = map[string]string{}
	}
	var projection map[string]json.RawMessage
	err := rustengine.Call(r.Context(), "project_topology", input, &projection)
	if analysisUnavailable(w, err) {
		return
	}
	if err != nil {
		apiError(w, 400, "invalid_topology_view", "The topology draft cannot be projected. Review device IDs, links and view selections.")
		return
	}
	if len(projection["topology"]) == 0 || bytes.Equal(projection["topology"], []byte("null")) {
		apiError(w, 503, "analysis_engine_unavailable", "The topology projection is unavailable. Check the required Rust worker.")
		return
	}
	writeJSON(w, 200, projection)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		apiError(w, 400, "json_required", "Send a JSON request.")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			apiError(w, 413, "input_too_large", "The request exceeds the local input limit.")
		} else {
			apiError(w, 400, "invalid_request", "The request could not be read.")
		}
		return false
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		apiError(w, 400, "invalid_json", "A JSON object is required.")
		return false
	}
	// Reject duplicate or case-alias fields instead of silently accepting a
	// later value for a security-relevant scope or predicate.
	if rejectDuplicateKeys(json.NewDecoder(bytes.NewReader(data))) != nil {
		apiError(w, 400, "invalid_json", "Use unique JSON field names.")
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if dec.Decode(dst) != nil {
		apiError(w, 400, "invalid_json", "The JSON fields are invalid.")
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		apiError(w, 400, "invalid_json", "Send one JSON object only.")
		return false
	}
	return true
}

func rejectDuplicateKeys(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok {
				return errors.New("invalid object key")
			}
			s = strings.ToLower(s)
			if seen[s] {
				return errors.New("duplicate field")
			}
			seen[s] = true
			if err = rejectDuplicateKeys(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := rejectDuplicateKeys(dec); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid delimiter")
	}
	_, err = dec.Token()
	return err
}

func (a *App) importConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Config string `json:"config"`
		Vendor string `json:"vendor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Config) > maxConfigBytes {
		apiError(w, 413, "config_too_large", "Configurations are limited to 2 MiB.")
		return
	}
	if strings.TrimSpace(req.Config) == "" || len(req.Name) > 128 || strings.ContainsAny(req.Name, "\r\n\x00") {
		apiError(w, 400, "invalid_import", "Provide a configuration and a short display name.")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Firewall configuration"
	}
	select {
	case a.importing <- struct{}{}:
	default:
		apiError(w, 429, "import_busy", "Another import is running. Try again shortly.")
		return
	}
	// Release parsing/persistence admission before publishing a response: a
	// client may begin its next sequential import as soon as it reads JSON.
	var snap *Snapshot
	var created bool
	var err error
	func() {
		defer func() { <-a.importing }()
		snap, created, err = a.store.ImportVendor(req.Name, []byte(req.Config), req.Vendor)
	}()
	if errors.Is(err, ErrRecordTooLarge) {
		apiError(w, 413, "snapshot_too_large", "The expanded snapshot exceeds the 8 MiB artifact budget. Earlier snapshots are unchanged.")
		return
	}
	if errors.Is(err, ErrQuota) {
		apiError(w, 429, "workspace_full", "The local workspace has reached its snapshot limit.")
		return
	}
	if errors.Is(err, firewall.ErrAnalysisEngine) {
		apiError(w, 503, "analysis_engine_unavailable", "The local analysis engine is unavailable. Check the application installation and retry. Earlier snapshots are unchanged.")
		return
	}
	if errors.Is(err, ErrInvalidConfig) {
		apiError(w, 422, "import_failed", "Use supported ASA running-config, PAN-OS set-format or FortiOS single-VDOM CLI configuration. Earlier snapshots are unchanged.")
		return
	}
	if err != nil {
		apiError(w, 500, "workspace_write_failed", "The local workspace could not save this import. Earlier snapshots are unchanged.")
		return
	}
	status := 200
	if created {
		status = 201
	}
	writeJSON(w, status, map[string]any{"snapshot": snap})
}

func (a *App) evaluate(w http.ResponseWriter, r *http.Request, snapshotID string) {
	var req struct {
		ChainID         string `json:"chain_id"`
		Source          string `json:"source"`
		Destination     string `json:"destination"`
		Protocol        string `json:"protocol"`
		SourcePort      int    `json:"source_port"`
		DestinationPort int    `json:"destination_port"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ChainID == "" {
		apiError(w, 400, "chain_required", "Select a chain from this snapshot.")
		return
	}
	flow := policy.Flow{Source: req.Source, Destination: req.Destination, Protocol: req.Protocol, SourcePort: req.SourcePort, DestinationPort: req.DestinationPort}
	inv, err := a.store.Evaluate(snapshotID, req.ChainID, flow)
	if analysisUnavailable(w, err) {
		return
	}
	if errors.Is(err, ErrRecordTooLarge) {
		apiError(w, 413, "investigation_too_large", "The investigation trace exceeds the 8 MiB artifact budget. No partial result was saved.")
		return
	}
	if errors.Is(err, ErrNotFound) {
		apiError(w, 404, "scope_not_found", "The selected snapshot or chain is unavailable.")
		return
	}
	if errors.Is(err, ErrQuota) {
		apiError(w, 429, "workspace_full", "The local investigation history has reached its limit.")
		return
	}
	if errors.Is(err, ErrInvalidFlow) {
		apiError(w, 400, "invalid_flow", "Use concrete, native IP addresses and valid protocol/port values for this policy investigation.")
		return
	}
	if err != nil {
		apiError(w, 500, "workspace_write_failed", "The local workspace could not save this investigation.")
		return
	}
	writeJSON(w, 200, map[string]any{"investigation": inv})
}

func methodError(w http.ResponseWriter) {
	apiError(w, 405, "method_not_allowed", "This method is not supported.")
}

func analysisUnavailable(w http.ResponseWriter, err error) bool {
	if !errors.Is(err, firewall.ErrAnalysisEngine) {
		return false
	}
	apiError(w, http.StatusServiceUnavailable, "analysis_engine_unavailable", "The required Rust analysis engine is unavailable. No partial investigation was saved.")
	return true
}

func apiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "Response encoding failed.", 500)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func HTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
}
