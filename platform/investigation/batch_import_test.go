package investigation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func TestBatchPreviewIsSanitizedAndDoesNotPublish(t *testing.T) {
	b, store := newTestBrowser(t)
	config := string(firewall.SampleASA())
	secret := "raw-secret-that-must-not-escape-preview"
	invalid := "unsupported-format\npassword " + secret
	body, _ := json.Marshal(map[string]any{"items": []any{
		map[string]any{"name": "valid.cfg", "config": config, "vendor": "auto"},
		map[string]any{"name": "bad.cfg", "config": invalid, "vendor": "auto"},
		map[string]any{"name": "same-content.cfg", "config": config, "vendor": "auto"},
	}})
	resp, err := b.request(http.MethodPost, "/api/import-previews", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preview status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	data := readBody(t, resp)
	if strings.Contains(data, secret) || strings.Contains(data, "unsupported-format") {
		t.Fatalf("preview reflected raw input or parser detail: %s", data)
	}
	var envelope struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		t.Fatal(err)
	}
	p := envelope.Preview
	if p.Valid || len(p.Items) != 3 || p.Items[0].Status != "ready" || p.Items[0].Existing || p.Items[1].Status != "invalid" || p.Items[1].ErrorCode == "" || p.Items[2].Status != "duplicate" {
		t.Fatalf("unexpected preview result: %#v", p)
	}
	if len(p.Items[0].SourcePreview) > maxPreviewSourceLine || len(p.Items[0].Warnings) > maxPreviewWarnings {
		t.Fatalf("preview bounds were not applied: %#v", p.Items[0])
	}
	for _, line := range p.Items[0].SourcePreview {
		if len(line.Text) > maxPreviewLineBytes {
			t.Fatalf("preview line exceeds its byte limit: %d", len(line.Text))
		}
	}
	if got := len(store.Summaries()); got != 0 {
		t.Fatalf("preview persisted %d snapshots", got)
	}
}

func TestBatchPreviewPublishRevisionAndIdempotency(t *testing.T) {
	b, store := newTestBrowser(t)
	workspace := store.WorkspaceInfo()
	siteBody, _ := json.Marshal(map[string]any{"name": "HQ", "description": "", "expected_revision": workspace.Revision})
	resp, err := b.request(http.MethodPost, "/api/sites", siteBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("site create status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var siteResponse struct {
		Site      Site      `json:"site"`
		Workspace Workspace `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &siteResponse); err != nil {
		t.Fatal(err)
	}
	if siteResponse.Site.ID == "" || siteResponse.Workspace.Revision != workspace.Revision+1 {
		t.Fatalf("unexpected site response: %#v", siteResponse)
	}
	config := string(firewall.SampleASA())
	previewBody, _ := json.Marshal(map[string]any{"items": []any{map[string]any{"name": "edge.cfg", "config": config, "vendor": "auto"}}})
	resp, err = b.request(http.MethodPost, "/api/import-previews", previewBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preview status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var previewEnvelope struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &previewEnvelope); err != nil {
		t.Fatal(err)
	}
	preview := previewEnvelope.Preview
	if !preview.Valid || len(preview.Items) != 1 || preview.Items[0].Status != "ready" {
		t.Fatalf("unexpected valid preview: %#v", preview)
	}
	staleBody, _ := json.Marshal(map[string]any{"set_name": "Edge", "site_id": siteResponse.Site.ID, "expected_revision": workspace.Revision})
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+preview.ID+"/publish", staleBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusConflict || errorCode(t, resp) != "workspace_conflict" {
		t.Fatalf("stale revision status=%d", resp.StatusCode)
	}

	publishBody, _ := json.Marshal(map[string]any{"set_name": "Edge", "site_id": siteResponse.Site.ID, "expected_revision": siteResponse.Workspace.Revision})
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+preview.ID+"/publish", publishBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("publish status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var published struct {
		SnapshotSet SnapshotSet `json:"snapshot_set"`
		Workspace   Workspace   `json:"workspace"`
		Created     bool        `json:"created"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &published); err != nil {
		t.Fatal(err)
	}
	if !published.Created || published.Workspace.ActiveSetID != published.SnapshotSet.ID || len(published.SnapshotSet.Members) != 1 || len(store.Summaries()) != 1 {
		t.Fatalf("unexpected publication: %#v", published)
	}

	// Publish a different set so the cached retry cannot pass by reactivating A.
	existingSnapshot, err := store.Snapshot(published.SnapshotSet.Members[0].SnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	setB, createdB, err := store.PublishSnapshotSet("Second set", siteResponse.Site.ID, []Snapshot{*existingSnapshot}, published.Workspace.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if !createdB {
		t.Fatal("second set was not created")
	}
	workspaceB := store.WorkspaceInfo()
	if workspaceB.ActiveSetID != setB.ID || setB.ID == published.SnapshotSet.ID {
		t.Fatalf("second set did not become active: set=%#v workspace=%#v", setB, workspaceB)
	}
	currentRevision := workspaceB.Revision
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+preview.ID+"/publish", publishBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("idempotent retry status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var retry struct {
		SnapshotSet SnapshotSet `json:"snapshot_set"`
		Workspace   Workspace   `json:"workspace"`
		Created     bool        `json:"created"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &retry); err != nil {
		t.Fatal(err)
	}
	if retry.SnapshotSet.ID != published.SnapshotSet.ID || retry.Workspace.Revision != currentRevision || retry.Workspace.ActiveSetID != setB.ID || !retry.Created {
		t.Fatalf("unexpected retry response: %#v", retry)
	}
	workspaceAfterRetry := store.WorkspaceInfo()
	if workspaceAfterRetry.Revision != currentRevision || workspaceAfterRetry.ActiveSetID != setB.ID {
		t.Fatalf("idempotent retry changed the current workspace: %#v", workspaceAfterRetry)
	}
	otherBody, _ := json.Marshal(map[string]any{"set_name": "Different", "site_id": siteResponse.Site.ID, "expected_revision": currentRevision})
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+preview.ID+"/publish", otherBody, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("different retry binding status=%d", resp.StatusCode)
	}
}

func TestWorkspaceAndPreviewBoundaryErrors(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, tc := range []struct {
		body string
		want string
	}{{`{"name":"Renamed"}`, "invalid_workspace"}, {`{"name":"Renamed","name":"Other","expected_revision":0}`, "invalid_json"}} {
		resp, err := b.request(http.MethodPut, "/api/workspace", []byte(tc.body), true)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusBadRequest || errorCode(t, resp) != tc.want {
			t.Fatalf("workspace error %q status=%d", tc.want, resp.StatusCode)
		}
	}
	resp, err := b.request(http.MethodPost, "/api/import-previews/"+strings.Repeat("0", 64)+"/publish", []byte(`{"set_name":"Set","site_id":"`+strings.Repeat("1", 64)+`","expected_revision":0}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound || errorCode(t, resp) != "preview_not_found" {
		t.Fatalf("unknown preview status=%d", resp.StatusCode)
	}
	resp, err = b.request(http.MethodPost, "/api/import-previews", []byte(`{"items":[],"items":[]}`), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, resp) != "invalid_json" {
		t.Fatalf("duplicate JSON field status=%d", resp.StatusCode)
	}
	resp, err = b.request(http.MethodPost, "/api/import-previews", []byte(`{"items":[{"name":"x","config":"x","vendor":"auto"}]}`), false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", resp.StatusCode)
	}
}

func TestBatchPreviewExpiry(t *testing.T) {
	b, _ := newTestBrowser(t)
	previewBody := mustJSON(map[string]any{"items": []any{map[string]any{"name": "edge.cfg", "config": string(firewall.SampleASA()), "vendor": "auto"}}})
	resp, err := b.request(http.MethodPost, "/api/import-previews", previewBody, true)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &envelope); err != nil {
		t.Fatal(err)
	}
	app := b.ts.Config.Handler.(*App)
	app.previewMu.Lock()
	app.previews[envelope.Preview.ID].response.ExpiresAt = time.Now().UTC().Add(-time.Second)
	app.previewMu.Unlock()
	body := mustJSON(map[string]any{"set_name": "Set", "site_id": strings.Repeat("1", 64), "expected_revision": 0})
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+envelope.Preview.ID+"/publish", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusGone || errorCode(t, resp) != "preview_expired" {
		t.Fatalf("expired preview status=%d", resp.StatusCode)
	}
}

func TestBatchPreviewIsLostOnServerRestart(t *testing.T) {
	b, store := newTestBrowser(t)
	body := mustJSON(map[string]any{"items": []any{map[string]any{"name": "edge.cfg", "config": string(firewall.SampleASA()), "vendor": "auto"}}})
	resp, err := b.request(http.MethodPost, "/api/import-previews", body, true)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &envelope); err != nil {
		t.Fatal(err)
	}
	dir := store.dir
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restartedStore, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restartedStore.Close() })
	ts := httptest.NewUnstartedServer(http.NotFoundHandler())
	app, err := NewHandler(restartedStore, ts.Listener.Addr().String())
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	ts.Config.Handler = app
	ts.Start()
	t.Cleanup(ts.Close)
	restarted := &testBrowser{ts: ts, client: ts.Client(), host: ts.Listener.Addr().String()}
	resp, err = restarted.request(http.MethodGet, "/api/session", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == sessionCookie {
			restarted.cookie = cookie
		}
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	restarted.csrf = session.CSRF
	request := mustJSON(map[string]any{"set_name": "After restart", "site_id": "", "expected_revision": 0})
	resp, err = restarted.request(http.MethodPost, "/api/import-previews/"+envelope.Preview.ID+"/publish", request, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound || errorCode(t, resp) != "preview_not_found" {
		t.Fatalf("restart cache loss status=%d", resp.StatusCode)
	}
}

func TestBatchPublishUnassignedAndExistingSnapshotKeepsUploadLabel(t *testing.T) {
	b, store := newTestBrowser(t)
	config := string(firewall.SampleASA())
	_, _, err := store.ImportVendor("canonical-original.cfg", []byte(config), "auto")
	if err != nil {
		t.Fatal(err)
	}
	body := mustJSON(map[string]any{"items": []any{map[string]any{"name": "renamed-upload.cfg", "config": config, "vendor": "auto"}}})
	resp, err := b.request(http.MethodPost, "/api/import-previews", body, true)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Preview.Valid || !envelope.Preview.Items[0].Existing {
		t.Fatalf("expected reusable existing snapshot: %#v", envelope.Preview)
	}
	publish := mustJSON(map[string]any{"set_name": "Unassigned", "site_id": "", "expected_revision": 0})
	resp, err = b.request(http.MethodPost, "/api/import-previews/"+envelope.Preview.ID+"/publish", publish, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unassigned publish status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	var published struct {
		SnapshotSet SnapshotSet `json:"snapshot_set"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &published); err != nil {
		t.Fatal(err)
	}
	if published.SnapshotSet.SiteID != "" || len(published.SnapshotSet.Members) != 1 || published.SnapshotSet.Members[0].Name != "renamed-upload.cfg" {
		t.Fatalf("uploaded label or unassigned site was lost: %#v", published.SnapshotSet)
	}
	stored, err := store.Snapshot(published.SnapshotSet.Members[0].SnapshotID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "canonical-original.cfg" {
		t.Fatalf("existing snapshot name changed: %q", stored.Name)
	}
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func errorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var value struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value.Error.Code
}
