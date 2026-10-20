package investigation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func topologyResponse(t *testing.T, browser *testBrowser, input any, verified bool) (int, map[string]any) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	response, err := browser.request(http.MethodPost, "/api/topology-view", data, verified)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var output map[string]any
	if err := json.NewDecoder(response.Body).Decode(&output); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, output
}

func TestTopologyViewUsesStoredEvidenceAndPreservesSavedAnalysis(t *testing.T) {
	browser, store := newTestBrowser(t)
	request := importExplorationSample(t, store)
	saved, err := store.ExploreNetwork(request)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(saved)
	snapshotBefore, _ := store.Snapshot(request.Nodes[0].SnapshotID)
	snapshotBytes, _ := json.Marshal(snapshotBefore)
	// A saved identity selects its server-owned request. Client replacements
	// cannot substitute snapshots or flow data in that saved investigation.
	view := topologyViewRequest{ExplorationID: saved.ID, Request: NetworkRequest{}, Roles: map[string]string{"north": "router", "@query-destination": "server"}}
	status, body := topologyResponse(t, browser, view, true)
	if status != http.StatusOK {
		t.Fatalf("projection status=%d body=%v", status, body)
	}
	topology := body["topology"].(map[string]any)
	nodes := topology["nodes"].([]any)
	if len(nodes) != 6 {
		t.Fatalf("expected four pinned devices and two labeled query endpoints, got %d", len(nodes))
	}
	roles := map[string]string{}
	for _, value := range nodes {
		node := value.(map[string]any)
		roles[node["node_id"].(string)] = node["role"].(string)
	}
	if roles["edge"] != "firewall" || roles["north"] != "router" || roles["@query-source"] != "host" || roles["@query-destination"] != "server" {
		t.Fatalf("incorrect display roles: %#v", roles)
	}
	facets := topology["facets"].(map[string]any)
	for _, name := range []string{"zone", "subnet", "vrf", "vdom"} {
		if facets[name].(map[string]any)["available"] != true {
			t.Errorf("missing source-backed %s facet", name)
		}
	}
	if topology["traffic"].(map[string]any)["available"] != false {
		t.Fatal("configuration evidence cannot invent traffic measurements")
	}
	owned := map[string]map[string]bool{}
	for _, ref := range request.Nodes {
		snapshot, _ := store.Snapshot(ref.SnapshotID)
		owned[ref.SnapshotID] = map[string]bool{}
		for _, evidence := range snapshot.Evidence {
			owned[ref.SnapshotID][evidence.ID] = true
		}
	}
	checked := 0
	for _, value := range topology["links"].([]any) {
		link := value.(map[string]any)
		for _, entry := range link["evidence_refs"].([]any) {
			evidence := entry.(map[string]any)
			if !owned[evidence["snapshot_id"].(string)][evidence["ref"].(string)] {
				t.Fatal("link evidence is not owned by its pinned snapshot")
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("expected interface source evidence on declared links")
	}
	encoded, _ := json.Marshal(body)
	if bytes.Contains(encoded, []byte("source_lines")) || bytes.Contains(encoded, []byte("set rulebase")) {
		t.Fatal("projection leaked configuration source instead of evidence references")
	}
	after, _ := store.NetworkExploration(saved.ID)
	afterBytes, _ := json.Marshal(after)
	snapshotAfter, _ := store.Snapshot(request.Nodes[0].SnapshotID)
	snapshotAfterBytes, _ := json.Marshal(snapshotAfter)
	if !bytes.Equal(before, afterBytes) || !bytes.Equal(snapshotBytes, snapshotAfterBytes) || len(store.ExplorationSummaries()) != 1 {
		t.Fatal("display projection changed immutable saved work")
	}
}

func TestTopologyViewRejectsScopeInjectionAndUnavailableWorkers(t *testing.T) {
	browser, store := newTestBrowser(t)
	request := importExplorationSample(t, store)
	view := topologyViewRequest{Request: request}
	duplicate := request
	duplicate.Nodes = append([]NetworkNodeRef(nil), request.Nodes...)
	duplicate.Nodes[1].SnapshotID = duplicate.Nodes[0].SnapshotID
	if status, _ := topologyResponse(t, browser, view, false); status != http.StatusForbidden {
		t.Fatalf("unverified projection status=%d", status)
	}
	response, err := browser.request(http.MethodGet, "/api/topology-view", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET projection status=%d", response.StatusCode)
	}
	for name, invalid := range map[string]any{
		"unknown config injection": map[string]any{"request": request, "config": map[string]any{}},
		"client result injection":  map[string]any{"request": request, "result": map[string]any{"disposition": "permit"}},
		"invalid annotation":       topologyViewRequest{Request: request, Roles: map[string]string{"edge": "database"}},
		"foreign annotation":       topologyViewRequest{Request: request, Roles: map[string]string{"foreign": "host"}},
		"negative branch":          topologyViewRequest{Request: request, BranchIndex: -1},
		"duplicate snapshot":       topologyViewRequest{Request: duplicate},
		"two identities":           topologyViewRequest{Request: request, ExplorationID: strings.Repeat("a", 64), InvestigationID: strings.Repeat("b", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			if status, _ := topologyResponse(t, browser, invalid, true); status != http.StatusBadRequest {
				t.Fatalf("invalid projection status=%d", status)
			}
		})
	}
	t.Run("worker unavailable", func(t *testing.T) {
		t.Setenv("GRAPHLAYER_ENGINE", filepath.Join(t.TempDir(), "missing-engine"))
		if status, _ := topologyResponse(t, browser, view, true); status != http.StatusServiceUnavailable {
			t.Fatalf("missing worker projection status=%d", status)
		}
	})
}
