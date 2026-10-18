package investigation

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
	"github.com/mattpainter701/graphlayer/platform/policy"
)

type delayedImportResponse struct {
	*httptest.ResponseRecorder
	started chan struct{}
	release chan struct{}
}

func (w *delayedImportResponse) Write(b []byte) (int, error) {
	close(w.started)
	<-w.release
	return w.ResponseRecorder.Write(b)
}

func TestSequentialImportAdmissionDoesNotWaitForPreviousResponseDelivery(t *testing.T) {
	s, _ := openTestStore(t)
	app, err := NewHandler(s, "127.0.0.1:8088")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := json.Marshal(map[string]string{"name": "PAN", "vendor": "paloalto-panos", "config": string(firewall.SamplePANOS())})
	writer := &delayedImportResponse{httptest.NewRecorder(), make(chan struct{}), make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.importConfig(writer, httptest.NewRequest(http.MethodPost, "/api/imports", bytes.NewReader(first)))
	}()
	defer func() { close(writer.release); <-done }()
	<-writer.started
	second, _ := json.Marshal(map[string]string{"name": "Forti", "vendor": "fortinet-fortigate", "config": string(firewall.SampleFortiGate())})
	response := httptest.NewRecorder()
	app.importConfig(response, httptest.NewRequest(http.MethodPost, "/api/imports", bytes.NewReader(second)))
	if response.Code != http.StatusCreated {
		t.Fatalf("completed prior import retained parsing admission: %d %s", response.Code, response.Body.String())
	}
}

func TestHTTPVendorSamplesUseStrictSelectorsAndRedactFailures(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, tc := range []struct {
		kind, vendor string
	}{
		{"panos", "paloalto-panos"},
		{"fortigate", "fortinet-fortigate"},
	} {
		resp, err := b.request(http.MethodGet, "/api/sample/"+tc.kind, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("sample %s status = %d", tc.kind, resp.StatusCode)
		}
		var sample struct {
			Name   string `json:"name"`
			Config string `json:"config"`
			Vendor string `json:"vendor"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&sample); err != nil {
			resp.Body.Close()
			t.Fatal(err)
		}
		resp.Body.Close()
		if sample.Name == "" || sample.Config == "" || sample.Vendor != tc.vendor {
			t.Fatalf("sample %s metadata = %#v", tc.kind, sample)
		}
		payload, _ := json.Marshal(map[string]string{"name": sample.Name, "config": sample.Config, "vendor": tc.vendor})
		resp, err = b.request(http.MethodPost, "/api/imports", payload, true)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("import %s status=%d body=%s", tc.kind, resp.StatusCode, readBody(t, resp))
		}
		var imported struct {
			Snapshot Snapshot `json:"snapshot"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&imported); err != nil {
			resp.Body.Close()
			t.Fatal(err)
		}
		resp.Body.Close()
		if imported.Snapshot.Vendor != tc.vendor || len(imported.Snapshot.SourceLines) == 0 || len(imported.Snapshot.Evidence) == 0 {
			t.Fatalf("import %s lost canonical vendor or source evidence: %#v", tc.kind, imported.Snapshot)
		}
	}

	for _, tc := range []struct {
		name, vendor, config string
	}{
		{"unknown selector", "paloalto", "set deviceconfig system hostname edge-fw\n"},
		{"PAN-OS XML is outside the supported format", "paloalto-panos", "<config><devices></config>"},
		{"FortiOS malformed block", "fortinet-fortigate", "config firewall policy\n edit 1\n set action accept\n"},
		{"ASA does not match PAN selector", "paloalto-panos", "hostname wrong-vendor\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			canary := "MULTIVENDOR_IMPORT_SECRET_CANARY"
			payload, _ := json.Marshal(map[string]string{"name": tc.name, "config": tc.config + "\npassword " + canary, "vendor": tc.vendor})
			resp, err := b.request(http.MethodPost, "/api/imports", payload, true)
			if err != nil {
				t.Fatal(err)
			}
			body := readBody(t, resp)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("invalid %s status=%d body=%s", tc.name, resp.StatusCode, body)
			}
			if strings.Contains(body, canary) || strings.Contains(body, tc.config) {
				t.Fatalf("failed import leaked uploaded content: %s", body)
			}
		})
	}
}

func TestVendorImportSanitizesSourceAndUnsupportedFeaturesStayIndeterminate(t *testing.T) {
	b, store := newTestBrowser(t)
	canary := "MULTIVENDOR_SOURCE_SECRET_CANARY"
	asa := strings.Replace(string(firewall.SampleASA()), "hostname edge-fw", "hostname edge-fw\nusername sanitized-user password 5 "+canary, 1)
	if !strings.Contains(asa, canary) {
		t.Fatal("ASA sample hostname not found for secret canary injection")
	}
	payload, _ := json.Marshal(map[string]string{"name": "sanitized ASA", "config": asa, "vendor": "cisco-asa"})
	resp, err := b.request(http.MethodPost, "/api/imports", payload, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("ASA import status=%d body=%s", resp.StatusCode, readBody(t, resp))
	}
	encoded, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), canary) {
		t.Fatal("successful import response exposed the uploaded credential")
	}
	var imported struct {
		Snapshot Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(encoded, &imported); err != nil {
		t.Fatal(err)
	}
	for _, line := range imported.Snapshot.SourceLines {
		if strings.Contains(line.Text, canary) {
			t.Fatal("persisted source evidence exposed the uploaded credential")
		}
	}

	for _, tc := range []struct {
		name, kind, config string
	}{
		{"PAN App-ID", "paloalto-panos", "set rulebase security rules unsupported-app application [ web-browsing ]\n"},
		{"Forti central NAT", "fortinet-fortigate", "config firewall central-snat-map\n edit 1\n set srcintf \"port2\"\n set dstintf \"port1\"\n next\nend\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := b.request(http.MethodGet, "/api/sample/"+map[string]string{"paloalto-panos": "panos", "fortinet-fortigate": "fortigate"}[tc.kind], nil, false)
			if err != nil {
				t.Fatal(err)
			}
			var sample struct {
				Name, Config string
			}
			if err := json.NewDecoder(resp.Body).Decode(&sample); err != nil {
				resp.Body.Close()
				t.Fatal(err)
			}
			resp.Body.Close()
			config := sample.Config + tc.config
			payload, _ := json.Marshal(map[string]string{"name": sample.Name + " unsupported feature", "config": config, "vendor": tc.kind})
			resp, err = b.request(http.MethodPost, "/api/imports", payload, true)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusCreated {
				t.Fatalf("recognized %s config should import with an unsupported warning, status=%d body=%s", tc.name, resp.StatusCode, readBody(t, resp))
			}
			var result struct {
				Snapshot Snapshot `json:"snapshot"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				t.Fatal(err)
			}
			resp.Body.Close()
			if len(result.Snapshot.Path.Warnings) == 0 {
				t.Fatalf("%s import did not retain unsupported feature evidence", tc.name)
			}
			ingress := "trust"
			if tc.kind == "fortinet-fortigate" {
				ingress = "port1"
			}
			inv, err := store.EvaluatePath(result.Snapshot.ID, ingress, policy.Flow{Source: "10.1.0.42", Destination: "203.0.113.15", Protocol: "tcp", SourcePort: 52000, DestinationPort: 443})
			if err != nil {
				t.Fatal(err)
			}
			if inv.Result.Disposition != "indeterminate" {
				t.Fatalf("unsupported %s resolved to %s", tc.name, inv.Result.Disposition)
			}
		})
	}
}

func TestReleasedV4WorkspaceBytesSurviveMixedV5Reopen(t *testing.T) {
	dir, fixture := copyActualWorkspace(t, "workspace-v4")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("open released v4 workspace: %v", err)
	}
	if len(s.Summaries()) != countPrefixRecords(fixture, "snapshot-") || len(s.NetworkSummaries()) != countPrefixRecords(fixture, "network-investigation-") {
		t.Fatal("released v4 workspace did not contain its expected snapshot and network history")
	}
	req := importVendorNetworkSample(t, s)
	inv, err := s.EvaluateNetwork(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertWorkspaceBytes(t, dir, fixture)

	reopened, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reopen mixed released-v4/current-v5 workspace: %v", err)
	}
	defer reopened.Close()
	if len(reopened.Summaries()) != countPrefixRecords(fixture, "snapshot-")+3 || len(reopened.NetworkSummaries()) != countPrefixRecords(fixture, "network-investigation-")+1 {
		t.Fatalf("mixed workspace lost history: %d snapshots, %d network investigations", len(reopened.Summaries()), len(reopened.NetworkSummaries()))
	}
	replayed, err := reopened.NetworkInvestigation(inv.ID)
	if err != nil || replayed.Result.Disposition != inv.Result.Disposition || len(replayed.Result.Forward.Hops) != 3 || len(replayed.Result.Return.Hops) != 3 {
		t.Fatalf("mixed v5 investigation replay = (%#v, %v)", replayed, err)
	}
	assertWorkspaceBytes(t, dir, fixture)
}

func countPrefixRecords(records map[string][]byte, prefix string) int {
	count := 0
	for name := range records {
		if strings.HasPrefix(name, prefix) {
			count++
		}
	}
	return count
}
