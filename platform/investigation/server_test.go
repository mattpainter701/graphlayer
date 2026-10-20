package investigation

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/firewall"
)

type testBrowser struct {
	ts     *httptest.Server
	client *http.Client
	cookie *http.Cookie
	csrf   string
	host   string
}

func newTestBrowser(t *testing.T) (*testBrowser, *Store) {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewUnstartedServer(http.NotFoundHandler())
	host := ts.Listener.Addr().String()
	app, err := NewHandler(store, host)
	if err != nil {
		ts.Close()
		store.Close()
		t.Fatal(err)
	}
	ts.Config.Handler = app
	ts.Start()
	b := &testBrowser{ts: ts, client: ts.Client(), host: host}
	t.Cleanup(func() { ts.Close(); _ = store.Close() })
	resp, err := b.request(http.MethodGet, "/api/session", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("session status = %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			b.cookie = c
		}
	}
	if b.cookie == nil || !b.cookie.HttpOnly || b.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie not protected: %#v", b.cookie)
	}
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	b.csrf = session.CSRF
	if b.csrf == "" {
		t.Fatal("missing CSRF token")
	}
	return b, store
}

func (b *testBrowser) request(method, path string, body []byte, verified bool) (*http.Response, error) {
	req, err := http.NewRequest(method, b.ts.URL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Host = b.host
	req.Header.Set("Content-Type", "application/json")
	if b.cookie != nil {
		req.AddCookie(b.cookie)
	}
	if verified {
		req.Header.Set("Origin", "http://"+b.host)
		req.Header.Set("X-CSRF-Token", b.csrf)
	}
	return b.client.Do(req)
}

func responseJSON(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer r.Body.Close()
	var value map[string]any
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestHTTPStaticAssetAllowlist(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, path := range []string{"/assets/unknown.js", "/assets/../server.go", "/assets/../../go.mod", "/assetsx/app.js", "/web/index.html"} {
		resp, err := b.request("GET", path, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
	resp, err := b.request("HEAD", "/assets/topology.js", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 || len(data) != 0 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("HEAD topology script: status=%d body=%d err=%v", resp.StatusCode, len(data), err)
	}
}

func TestHTTPImportEvaluateHistoryAndSecurityHeaders(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, path := range []string{"/", "/assets/app.js", "/assets/style.css", "/assets/shell.js", "/assets/shell.css", "/assets/topology.js", "/assets/topology.css", "/assets/d3.min.js"} {
		resp, err := b.request("GET", path, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("GET %s status = %d", path, resp.StatusCode)
		}
		if resp.Header.Get("Content-Security-Policy") == "" || !strings.Contains(resp.Header.Get("Content-Security-Policy"), "default-src 'none'") {
			t.Errorf("missing restrictive CSP on %s", path)
		}
		if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("missing nosniff on %s", path)
		}
		if strings.HasSuffix(path, ".js") && !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/javascript") {
			t.Errorf("incorrect script content type on %s", path)
		}
		if strings.HasSuffix(path, ".css") && !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") {
			t.Errorf("incorrect stylesheet content type on %s", path)
		}
		resp.Body.Close()
	}
	resp, err := b.request("GET", "/api/sample", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("sample status = %d", resp.StatusCode)
	}
	var sample struct {
		Name   string `json:"name"`
		Config string `json:"config"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&sample); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if sample.Name == "" || sample.Config != string(firewall.SampleASA()) {
		t.Fatal("sample endpoint did not serve the parser fixture")
	}
	body, _ := json.Marshal(map[string]string{"name": "sample", "config": sample.Config})
	resp, err = b.request("POST", "/api/imports", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("import status = %d body=%s", resp.StatusCode, readBody(t, resp))
	}
	imported := responseJSON(t, resp)
	snap := imported["snapshot"].(map[string]any)
	id := snap["id"].(string)
	chain := snap["chains"].([]any)[0].(map[string]any)
	chainID := chain["id"].(string)
	var investigationIDs []string
	for _, tc := range []struct {
		port int
		want string
	}{{443, "permit"}, {22, "deny"}, {8443, "indeterminate"}} {
		payload, _ := json.Marshal(map[string]any{"chain_id": chainID, "source": "192.0.2.19", "destination": "10.20.4.15", "protocol": "tcp", "source_port": 51000, "destination_port": tc.port})
		resp, err = b.request("POST", "/api/snapshots/"+id+"/evaluate", payload, true)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("evaluate %d status=%d body=%s", tc.port, resp.StatusCode, readBody(t, resp))
		}
		inv := responseJSON(t, resp)["investigation"].(map[string]any)
		investigationIDs = append(investigationIDs, inv["id"].(string))
		if inv["snapshot_id"] != id || inv["chain_id"] != chainID || inv["result"].(map[string]any)["disposition"] != tc.want {
			t.Errorf("unexpected evaluation %d: %#v", tc.port, inv)
		}
	}
	resp, err = b.request("GET", "/api/snapshots/"+id, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("snapshot detail status = %d", resp.StatusCode)
	}
	resp.Body.Close()
	for _, invID := range investigationIDs {
		resp, err = b.request("GET", "/api/investigations/"+invID, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("investigation detail status = %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
	for _, path := range []string{"/api/snapshots", "/api/investigations"} {
		resp, err = b.request("GET", path, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("history GET %s status=%d", path, resp.StatusCode)
		}
		value := responseJSON(t, resp)
		if path == "/api/snapshots" && len(value["snapshots"].([]any)) != 1 {
			t.Errorf("snapshot list has unexpected rows: %#v", value)
		}
		if path == "/api/investigations" && len(value["investigations"].([]any)) != 3 {
			t.Errorf("history has unexpected rows: %#v", value)
		}
	}
	for _, path := range []string{"/api/snapshots/" + id, "/api/snapshots/not-found"} {
		resp, err = b.request("GET", path, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	resp, err = b.request("DELETE", "/api/snapshots", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 405 {
		t.Errorf("unsupported API method = %d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, err = b.request("GET", "/missing", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("missing asset status = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func TestHTTPRejectsUntrustedSessionAndHostRequests(t *testing.T) {
	b, _ := newTestBrowser(t)
	post := []byte(`{"name":"x","config":"x"}`)
	// Missing session, wrong Origin, and missing CSRF fail before request decoding.
	for _, tc := range []struct {
		name                 string
		cookie, origin, csrf bool
		host                 string
	}{
		{"no session", false, true, true, b.host}, {"foreign host", true, true, true, "127.0.0.1:1"}, {"foreign origin", true, false, true, b.host}, {"no csrf", true, true, false, b.host},
	} {
		req, _ := http.NewRequest("POST", b.ts.URL+"/api/imports", bytes.NewReader(post))
		req.Host = tc.host
		req.Header.Set("Content-Type", "application/json")
		if tc.cookie && b.cookie != nil {
			req.AddCookie(b.cookie)
		}
		if tc.origin {
			req.Header.Set("Origin", "http://"+b.host)
		} else {
			req.Header.Set("Origin", "http://evil.example")
		}
		if tc.csrf {
			req.Header.Set("X-CSRF-Token", b.csrf)
		}
		resp, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Errorf("%s status = %d", tc.name, resp.StatusCode)
		}
		resp.Body.Close()
	}
	// Exercise remote-address and cross-site checks directly because HTTP clients always originate locally here.
	app, err := NewHandler(&Store{dir: t.TempDir(), snapshots: map[string][]byte{}, investigations: map[string][]byte{}}, b.host)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, remote, fetch string }{{"remote", "192.0.2.9:1234", "same-origin"}, {"fetch-site", "127.0.0.1:1234", "cross-site"}} {
		req := httptest.NewRequest("GET", "http://"+b.host+"/api/session", nil)
		req.Host = b.host
		req.RemoteAddr = tc.remote
		req.Header.Set("Sec-Fetch-Site", tc.fetch)
		rw := httptest.NewRecorder()
		app.ServeHTTP(rw, req)
		if rw.Code != 403 {
			t.Errorf("%s status = %d", tc.name, rw.Code)
		}
	}
}

func TestHTTPRejectsInvalidJSONAndBoundsRequests(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"malformed", `{"name":`, 400}, {"duplicate", `{"name":"a","NAME":"b","config":"x"}`, 400}, {"null", "null", 400},
		{"unknown", `{"name":"a","config":"x","unexpected":true}`, 400}, {"trailing", `{"name":"a","config":"x"}{}`, 400},
	} {
		resp, err := b.request("POST", "/api/imports", []byte(tc.body), true)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.status {
			t.Errorf("%s status = %d; want %d", tc.name, resp.StatusCode, tc.status)
		}
		resp.Body.Close()
	}
	tooLarge := []byte(`{"name":"large","config":"` + strings.Repeat("x", (2<<20)+1) + `"}`)
	resp, err := b.request("POST", "/api/imports", tooLarge, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 413 {
		t.Errorf("oversized config status = %d", resp.StatusCode)
	}
	resp.Body.Close()
	requestLarge := []byte(`{"name":"large","config":"` + strings.Repeat("x", (4<<20)+1) + `"}`)
	resp, err = b.request("POST", "/api/imports", requestLarge, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 413 {
		t.Errorf("oversized request status = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHTTPASAEdgeConfigurationsNeverBecomeInternalErrors(t *testing.T) {
	b, store := newTestBrowser(t)
	fixtures := []struct {
		name   string
		config string
	}{
		{"truncated binding", "hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-group IN in interface\n"},
		{"truncated ACL", "hostname edge\naccess-list IN\naccess-group IN in interface outside\n"},
		{"global binding", "hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-group IN global\n"},
		{"control-plane binding", "hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-group IN in interface outside control-plane\n"},
		{"unsupported object member before permit", "hostname edge\nobject network UNKNOWN\n fqdn v4 example.invalid\naccess-list IN line 10 extended deny tcp any object UNKNOWN eq 443\naccess-list IN line 20 extended permit tcp any any eq 443\naccess-group IN in interface outside\n"},
		{"explicit line order", "hostname edge\naccess-list IN line 20 extended permit tcp any any eq 443\naccess-list IN line 10 extended deny tcp any any eq 443\naccess-group IN in interface outside\n"},
		{"missing hostname", "access-list IN extended permit tcp any any eq 443\naccess-group IN in interface outside\n"},
		{"duplicate binding", "hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-group IN in interface outside\naccess-group IN in interface outside\n"},
		{"conflicting bindings", "hostname edge\naccess-list IN extended permit tcp any any eq 443\naccess-list OTHER extended permit tcp any any eq 443\naccess-group IN in interface outside\naccess-group OTHER in interface outside\n"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{"name": fixture.name, "config": fixture.config})
			resp, err := b.request("POST", "/api/imports", payload, true)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.StatusCode == 500 {
				t.Fatalf("user config became an internal error: %s", readBody(t, resp))
			}
			if resp.StatusCode == 422 {
				resp.Body.Close()
				return
			}
			if resp.StatusCode != 201 {
				t.Fatalf("unexpected import status %d: %s", resp.StatusCode, readBody(t, resp))
			}
			var envelope struct {
				Snapshot Snapshot `json:"snapshot"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
				resp.Body.Close()
				t.Fatalf("decode imported snapshot: %v", err)
			}
			resp.Body.Close()
			if len(envelope.Snapshot.Chains) == 0 {
				t.Fatal("accepted configuration produced no ACL chains")
			}
			for _, chain := range envelope.Snapshot.Chains {
				inv, err := store.Evaluate(envelope.Snapshot.ID, chain.ID, sampleFlow(443))
				if err != nil {
					t.Fatalf("evaluate accepted chain %q: %v", chain.ID, err)
				}
				switch fixture.name {
				case "truncated binding", "truncated ACL", "global binding", "control-plane binding", "conflicting bindings", "explicit line order":
					if inv.Result.Disposition == "permit" {
						t.Fatalf("unsupported or ambiguous attachment produced a permit: %#v", inv.Result)
					}
				}
				if fixture.name == "unsupported object member before permit" && inv.Result.Disposition == "permit" {
					t.Fatalf("early unresolved deny was bypassed by a later permit: %#v", inv.Result)
				}
			}
		})
	}
}

func TestHTTPParseErrorAndStorageFailureAreDistinctAndRedacted(t *testing.T) {
	b, store := newTestBrowser(t)
	canary := "HTTP_SECRET_CANARY_VALUE"
	badConfig := "hostname broken\nusername x password " + canary + strings.Repeat("x", 8200)
	bad, _ := json.Marshal(map[string]string{"name": "secret", "config": badConfig})
	resp, err := b.request("POST", "/api/imports", bad, true)
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != 422 {
		t.Fatalf("parse error status = %d body=%s", resp.StatusCode, body)
	}
	if strings.Contains(body, canary) {
		t.Fatal("parse error response leaked upload secret")
	}
	valid := firewall.SampleASA()
	parsed, err := firewall.ParseASAConfig(valid)
	if err != nil {
		t.Fatal(err)
	}
	if err := osMkdir(filepath.Join(store.dir, "snapshot-"+snapshotID(*parsed)+".json")); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"name": "write failure", "config": string(valid)})
	resp, err = b.request("POST", "/api/imports", payload, true)
	if err != nil {
		t.Fatal(err)
	}
	body = readBody(t, resp)
	if resp.StatusCode != 500 {
		t.Fatalf("storage error status = %d body=%s", resp.StatusCode, body)
	}
	if strings.Contains(body, filepath.Dir(store.dir)) || strings.Contains(strings.ToLower(body), "already exists") {
		t.Fatalf("storage response exposed internal detail: %s", body)
	}
}

func TestHTTPEscapesSecretCanariesAndEvidenceLines(t *testing.T) {
	b, _ := newTestBrowser(t)
	canary := "XSS_SECRET_CANARY"
	config := strings.ReplaceAll(string(firewall.SampleASA()), "hostname edge-fw", "hostname "+canary+" <script>alert(1)</script>")
	body, _ := json.Marshal(map[string]string{"name": "<img src=x onerror=alert(1)>", "config": config})
	resp, err := b.request("POST", "/api/imports", body, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("import status %d: %s", resp.StatusCode, readBody(t, resp))
	}
	encoded, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if bytes.Contains(encoded, []byte("PASSWORD_CANARY_VALUE")) || bytes.Contains(encoded, []byte("COMMUNITY_CANARY_VALUE")) || bytes.Contains(encoded, []byte("TUNNEL_KEY_CANARY_VALUE")) {
		t.Fatal("API response leaked fixture secrets")
	}
	var data map[string]any
	if err := json.Unmarshal(encoded, &data); err != nil {
		t.Fatal(err)
	}
	snap := data["snapshot"].(map[string]any)
	if snap["name"] != "<img src=x onerror=alert(1)>" {
		t.Fatal("display name changed unexpectedly")
	}
	lines := snap["source_lines"].([]any)
	for _, line := range lines {
		l := line.(map[string]any)
		txt := l["text"].(string)
		if strings.Contains(txt, canary) || strings.Contains(txt, "<script>") {
			t.Fatalf("non-policy hostname leaked as source: %s", txt)
		}
	}
	for _, ev := range snap["evidence"].([]any) {
		e := ev.(map[string]any)
		if e["start_line"].(float64) < 1 || e["end_line"].(float64) < e["start_line"].(float64) {
			t.Fatalf("invalid exact line evidence: %#v", e)
		}
	}
	// The Go JSON encoder must escape markup in responses to prevent an HTML embedding from becoming executable.
	if bytes.Contains(encoded, []byte("<script>")) || bytes.Contains(encoded, []byte("<img")) {
		t.Fatal("response JSON contains unescaped HTML markup")
	}
}

func osMkdir(path string) error { return os.Mkdir(path, 0700) }

func TestHTTPTrustedRequestRejectsOriginURLDecorations(t *testing.T) {
	b, _ := newTestBrowser(t)
	for _, origin := range []string{"https://" + b.host, "http://user@" + b.host, "http://" + b.host + "/path", "http://" + b.host + "?x=1", "http://evil.example"} {
		req, _ := http.NewRequest("GET", b.ts.URL+"/api/session", nil)
		req.Host = b.host
		req.Header.Set("Origin", origin)
		resp, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Errorf("Origin %q status = %d", origin, resp.StatusCode)
		}
		resp.Body.Close()
	}
	if _, _, err := net.SplitHostPort(b.host); err != nil {
		t.Fatalf("test listener host must be numeric host:port: %q", b.host)
	}
	if _, err := url.Parse("http://" + b.host); err != nil {
		t.Fatal(err)
	}
}
