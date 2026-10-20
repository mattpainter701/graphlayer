package investigation

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func layerResponse(t *testing.T, browser *testBrowser, endpoint string, input any, verified bool) (int, map[string]any) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	response, err := browser.request(http.MethodPost, endpoint, data, verified)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, responseJSON(t, response)
}

func TestLayerTopologyUsesRustAndLeavesWorkspaceUnchanged(t *testing.T) {
	browser, store := newTestBrowser(t)
	before := store.WorkspaceInfo()
	status, sample := layerResponse(t, browser, "/api/sample/topology-layers", map[string]any{}, true)
	if status != http.StatusOK {
		t.Fatalf("sample status %d: %v", status, sample)
	}
	dataset, ok := sample["dataset"].(map[string]any)
	if !ok || dataset["synthetic"] != true || len(dataset["hosts"].([]any)) != 1000 {
		t.Fatal("missing explicitly synthetic 1,000-host corpus")
	}
	for _, view := range []string{"l2", "l3", "zone"} {
		status, output := layerResponse(t, browser, "/api/topology-layers", map[string]any{"dataset": dataset, "view": view}, true)
		if status != http.StatusOK {
			t.Fatalf("%s status %d: %v", view, status, output)
		}
		counts := output["counts"].(map[string]any)
		if counts["hosts"] != float64(1000) || len(output["nodes"].([]any)) > 140 {
			t.Fatalf("unbounded or incomplete %s projection", view)
		}
	}
	if !reflect.DeepEqual(before, store.WorkspaceInfo()) || len(store.Summaries()) != 0 || len(store.Investigations()) != 0 {
		t.Fatal("transient auxiliary projection changed saved workspace state")
	}
}

func TestLayerTopologyAccessInputAndMissingWorker(t *testing.T) {
	browser, _ := newTestBrowser(t)
	for _, endpoint := range []string{"/api/sample/topology-layers", "/api/topology-layers"} {
		if status, _ := layerResponse(t, browser, endpoint, map[string]any{}, false); status != http.StatusForbidden {
			t.Fatalf("%s accepted unverified POST: %d", endpoint, status)
		}
		response, err := browser.request(http.MethodGet, endpoint, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s accepted GET", endpoint)
		}
	}
	if status, _ := layerResponse(t, browser, "/api/topology-layers", map[string]any{"dataset": nil, "view": "l2"}, true); status != http.StatusBadRequest {
		t.Fatalf("accepted absent dataset: %d", status)
	}
	oversized := map[string]any{"dataset": strings.Repeat("x", maxRequestBytes)}
	if status, _ := layerResponse(t, browser, "/api/topology-layers", oversized, true); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("accepted oversized request: %d", status)
	}
	t.Setenv("GRAPHLAYER_ENGINE", filepath.Join(t.TempDir(), "missing-worker"))
	if status, _ := layerResponse(t, browser, "/api/sample/topology-layers", map[string]any{}, true); status != http.StatusServiceUnavailable {
		t.Fatalf("missing Rust worker did not fail closed: %d", status)
	}
}
