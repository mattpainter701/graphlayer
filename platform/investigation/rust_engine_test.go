package investigation

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

func TestMissingRustWorkerReportsInstallationFailureWithoutPublishing(t *testing.T) {
	b, store := newTestBrowser(t)
	t.Setenv("GRAPHLAYER_ENGINE", filepath.Join(t.TempDir(), "missing-engine"))
	config := string(firewall.SampleASA())
	resp, err := b.request(http.MethodPost, "/api/imports", mustJSON(map[string]any{"name": "edge.cfg", "config": config, "vendor": "cisco-asa"}), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable || errorCode(t, resp) != "analysis_engine_unavailable" {
		t.Fatalf("missing worker was presented as bad user input: %d", resp.StatusCode)
	}
	resp, err = b.request(http.MethodPost, "/api/import-previews", mustJSON(map[string]any{"items": []any{map[string]any{"name": "edge.cfg", "config": config, "vendor": "cisco-asa"}}}), true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("preview: %d %s", resp.StatusCode, readBody(t, resp))
	}
	var result struct {
		Preview importPreviewResponse `json:"preview"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &result); err != nil {
		t.Fatal(err)
	}
	if result.Preview.Valid || len(result.Preview.Items) != 1 || result.Preview.Items[0].ErrorCode != "analysis_engine_unavailable" || len(store.Summaries()) != 0 || store.WorkspaceInfo().Revision != 0 {
		t.Fatal("unavailable worker allowed publication or was shown as invalid config")
	}
}
