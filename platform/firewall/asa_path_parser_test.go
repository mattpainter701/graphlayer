package firewall

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	asa "github.com/mattpainter701/graphlayer/platform/vendors/cisco/asa"
)

//go:embed testdata/asa_path_sample.txt
var asaPathSample []byte

func parsePathInventoryForTest(t *testing.T, input []byte) *Config {
	t.Helper()
	parsed, err := asa.ParseRunningConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	c := &Config{ArtifactSHA256: parsed.ArtifactSHA256, SourceLines: make([]SourceLine, len(parsed.Lines)), Evidence: []Evidence{}}
	for i, line := range parsed.Lines {
		c.SourceLines[i] = SourceLine{Number: line.Number, Text: "[OMITTED NON-POLICY CONFIGURATION]"}
	}
	if err := addASAPathInventory(c, parsed); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestASAPathInventoryParsesConnectedStaticRoutesAndObjectNAT(t *testing.T) {
	c := parsePathInventoryForTest(t, asaPathSample)
	p := c.Path
	if p == nil || p.Version != asaPathVersion || !p.RoutingComplete || !p.NATComplete || !p.InterInterface {
		t.Fatalf("unexpected path summary: %#v", p)
	}
	if len(p.Interfaces) != 3 || len(p.Routes) != 5 || len(p.NATRules) != 2 {
		t.Fatalf("unexpected path inventory sizes: interfaces=%d routes=%d NAT=%d", len(p.Interfaces), len(p.Routes), len(p.NATRules))
	}
	byName := map[string]PathInterface{}
	for _, in := range p.Interfaces {
		byName[in.Name] = in
	}
	if byName["inside"].Address != "10.0.0.1/24" || !byName["inside"].Resolved || byName["inside"].SecurityLevel == nil || *byName["inside"].SecurityLevel != 100 || *byName["outside"].SecurityLevel != 0 || *byName["dmz"].SecurityLevel != 50 {
		t.Fatalf("inside interface was not normalized: %#v", byName["inside"])
	}
	routes := map[string]PathRoute{}
	for _, route := range p.Routes {
		routes[route.Kind+"/"+route.Prefix] = route
	}
	if routes["connected/10.20.4.0/24"].Interface != "dmz" || routes["static/0.0.0.0/0"].NextHop != "192.0.2.254" || routes["static/203.0.113.0/24"].Distance != 5 {
		t.Fatalf("connected/static routes were not normalized: %#v", p.Routes)
	}
	nats := map[string]PathNATRule{}
	for _, rule := range p.NATRules {
		nats[rule.ObjectName] = rule
	}
	if nats["WEB"].RealPrefix != "10.20.4.15/32" || nats["WEB"].MappedPrefix != "198.51.100.15/32" || nats["WEB"].RealInterface != "dmz" || nats["IDENTITY-CLIENT"].RealPrefix != "10.0.0.20/32" || nats["IDENTITY-CLIENT"].MappedPrefix != "10.0.0.20/32" {
		t.Fatalf("static object NAT mappings were not normalized: %#v", p.NATRules)
	}
	if got := string(SamplePathASA()); !strings.Contains(got, "access-group OUTSIDE-IN") {
		t.Fatal("SamplePathASA did not return the embedded sample")
	}
	assertPathEvidence(t, c)
}

func TestParseASAConfigIntegratesThePathSample(t *testing.T) {
	c, err := ParseASAConfig(SamplePathASA())
	if err != nil {
		t.Fatal(err)
	}
	if c.ParserVersion != "asa-config-v3" || c.Path == nil || len(c.Chains) != 3 {
		t.Fatalf("path sample was not integrated into config v3: version=%s path=%#v chains=%d", c.ParserVersion, c.Path, len(c.Chains))
	}
	if !c.Path.RoutingComplete || !c.Path.NATComplete {
		t.Fatalf("supported sample unexpectedly incomplete: %#v", c.Path)
	}
	assertPathEvidence(t, c)
}

func TestASAPathInventoryMarksUnsupportedInputsWithoutLeakingRawValues(t *testing.T) {
	input := []byte("interface GigabitEthernet0/0\n nameif outside\n ip address 192.0.2.1 255.255.255.0\n!\nroute outside 203.0.113.0 255.255.255.0 192.0.2.2 track 9\nrouter ospf 10\n nat (inside,outside) source dynamic PRIVATE_POOL\nobject network WEB\n host 10.0.0.5\n nat (inside,outside) static 203.0.113.5 service tcp 443 8443\n")
	c := parsePathInventoryForTest(t, input)
	if c.Path.RoutingComplete || c.Path.NATComplete || len(c.Path.Routes) != 1 || c.Path.Routes[0].Kind != "connected" || len(c.Path.NATRules) != 0 {
		t.Fatalf("unsupported path input was treated as complete: %#v", c.Path)
	}
	serialized, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_POOL", "track 9", "8443"} {
		if strings.Contains(string(serialized), secret) {
			t.Fatalf("unsupported raw configuration value %q leaked: %s", secret, serialized)
		}
	}
	assertPathEvidence(t, c)
}

func TestASAPathInventoryMarksLegacyNATOnlyCommandsUnsupported(t *testing.T) {
	for name, line := range map[string]string{
		"classic static": "static (inside,outside) 203.0.113.7 10.0.0.7 netmask 255.255.255.255",
		"classic global": "global (outside) 1 203.0.113.8",
		"nat control":    "nat-control",
	} {
		t.Run(name, func(t *testing.T) {
			c := parsePathInventoryForTest(t, []byte(line+"\n"))
			if c.Path.NATComplete || len(c.Path.NATRules) != 0 || len(c.Path.Warnings) == 0 || c.SourceLines[0].Text != "[OMITTED UNSUPPORTED PATH CONFIGURATION]" {
				t.Fatalf("legacy NAT command was silently ignored: path=%#v source=%q", c.Path, c.SourceLines[0].Text)
			}
			serialized, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			for _, raw := range []string{"203.0.113.7", "203.0.113.8", "10.0.0.7"} {
				if strings.Contains(string(serialized), raw) {
					t.Fatalf("legacy NAT value %q leaked: %s", raw, serialized)
				}
			}
			assertPathEvidence(t, c)
		})
	}
}

func TestASAPathInventoryDeduplicatesAmbiguousInterfaceNames(t *testing.T) {
	input := []byte("interface GigabitEthernet0/0\n nameif shared\n ip address 10.0.0.1 255.255.255.0\ninterface GigabitEthernet0/1\n nameif shared\n ip address 192.0.2.1 255.255.255.0\nroute shared 0 0 10.0.0.254\n")
	c := parsePathInventoryForTest(t, input)
	if c.Path.RoutingComplete || len(c.Path.Interfaces) != 1 || c.Path.Interfaces[0].Name != "shared" || c.Path.Interfaces[0].Resolved || len(c.Path.Routes) != 0 {
		t.Fatalf("ambiguous interface name was published as resolved: %#v", c.Path)
	}
	assertPathEvidence(t, c)
}

func TestASAPathInventoryMarksStandbyAddressUnresolvedAndSanitizesEvidence(t *testing.T) {
	input := []byte("interface GigabitEthernet0/0\n nameif outside\n security-level 0\n ip address 192.0.2.1 255.255.255.0 standby 192.0.2.2\n")
	c := parsePathInventoryForTest(t, input)
	if c.Path.RoutingComplete || len(c.Path.Interfaces) != 1 || c.Path.Interfaces[0].Resolved || c.Path.Interfaces[0].Address != "192.0.2.1/24" {
		t.Fatalf("standby address form was treated as resolved transit state: %#v", c.Path)
	}
	found := false
	for _, warning := range c.Path.Warnings {
		if warning.Code == "unsupported_interface" && warning.Line == 4 {
			found = true
		}
	}
	if !found || c.SourceLines[3].Text != "[OMITTED UNSUPPORTED PATH CONFIGURATION]" {
		t.Fatalf("standby address lacks a sanitized unsupported marker: warning=%t line=%q", found, c.SourceLines[3].Text)
	}
	serialized, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "192.0.2.2") {
		t.Fatalf("standby peer address leaked into normalized evidence: %s", serialized)
	}
	assertPathEvidence(t, c)
}

func TestASAPathInventoryRejectsMappedSubnetRewritesAndUnsupportedObjectMembers(t *testing.T) {
	base := "interface Gi0/0\n nameif inside\n ip address 10.0.0.1 255.255.255.0\ninterface Gi0/1\n nameif outside\n ip address 192.0.2.1 255.255.255.0\n"
	for name, objectBody := range map[string]string{
		"non-aligned mapped subnet":  "object network WEB\n subnet 10.0.0.0 255.255.255.0\nobject network WEB\n nat (inside,outside) static 198.51.100.7\n",
		"unknown fqdn member":        "object network WEB\n host 10.0.0.7\n fqdn v4 web.example.invalid\nobject network WEB\n nat (inside,outside) static 198.51.100.7\n",
		"incompatible extra address": "object network WEB\n host 10.0.0.7\n subnet 10.0.0.0 255.255.255.0\nobject network WEB\n nat (inside,outside) static 198.51.100.7\n",
	} {
		t.Run(name, func(t *testing.T) {
			c := parsePathInventoryForTest(t, []byte(base+objectBody))
			if c.Path.NATComplete || len(c.Path.NATRules) != 0 {
				t.Fatalf("unsafe NAT mapping was accepted: %#v", c.Path)
			}
			assertPathEvidence(t, c)
		})
	}
}

func assertPathEvidence(t *testing.T, c *Config) {
	t.Helper()
	evidence := map[string]bool{}
	for _, item := range c.Evidence {
		if evidence[item.ID] {
			t.Fatalf("duplicate evidence ID %q", item.ID)
		}
		evidence[item.ID] = true
	}
	checkRefs := func(owner string, refs []string) {
		t.Helper()
		if len(refs) == 0 {
			t.Fatalf("%s has no evidence references", owner)
		}
		for _, ref := range refs {
			if !evidence[ref] {
				t.Fatalf("%s references missing evidence %q", owner, ref)
			}
		}
	}
	checkRefs("path config", c.Path.EvidenceRefs)
	for _, in := range c.Path.Interfaces {
		checkRefs("interface "+in.Name, in.EvidenceRefs)
	}
	for _, route := range c.Path.Routes {
		checkRefs("route "+route.ID, route.EvidenceRefs)
	}
	for _, nat := range c.Path.NATRules {
		checkRefs("NAT "+nat.ID, nat.EvidenceRefs)
	}
}

const fakeASAEngineResponse = `{"protocol_version":"graphlayer-engine-v1","operation":"parse_asa_path","result":{"path":{"version":"asa-path-v1","interfaces":[],"routes":[],"nat_rules":[],"routing_complete":true,"nat_complete":true,"inter_interface":false,"warnings":[],"evidence_refs":[]},"evidence":[],"source_line_updates":[]}}`

func TestASAEngineFakeWorker(t *testing.T) {
	mode := os.Getenv("GRAPH_LAYER_ASA_FAKE_MODE")
	if mode == "" {
		return
	}
	switch mode {
	case "valid":
		var request asaPathEngineRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil || request.ProtocolVersion != asaEngineProtocolVersion || request.Operation != asaEngineOperation {
			fmt.Fprintln(os.Stderr, "invalid fake worker request")
			os.Exit(31)
		}
		_, _ = io.WriteString(os.Stdout, fakeASAEngineResponse)
	case "timeout":
		time.Sleep(5 * time.Second)
	case "overflow":
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < 9; i++ {
			_, _ = io.WriteString(os.Stdout, chunk)
		}
	case "malformed":
		_, _ = io.WriteString(os.Stdout, `{"protocol_version":`)
	case "trailing":
		_, _ = io.WriteString(os.Stdout, fakeASAEngineResponse+` {}`)
	case "duplicate_protocol":
		response := strings.Replace(fakeASAEngineResponse, `"protocol_version":"graphlayer-engine-v1"`, `"protocol_version":"wrong","protocol_version":"graphlayer-engine-v1"`, 1)
		_, _ = io.WriteString(os.Stdout, response)
	case "duplicate_nested":
		response := strings.Replace(fakeASAEngineResponse, `"version":"asa-path-v1"`, `"version":"wrong","version":"asa-path-v1"`, 1)
		_, _ = io.WriteString(os.Stdout, response)
	case "wrong_protocol":
		_, _ = io.WriteString(os.Stdout, strings.Replace(fakeASAEngineResponse, asaEngineProtocolVersion, "wrong-version", 1))
	case "nonzero":
		_, _ = io.WriteString(os.Stderr, "sensitive-worker-canary")
		os.Exit(42)
	default:
		fmt.Fprintln(os.Stderr, "unknown fake mode")
		os.Exit(43)
	}
	os.Exit(0)
}

func useFakeASAEngine(t *testing.T, mode string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal("could not locate test binary")
	}
	t.Setenv("GRAPHLAYER_ENGINE", executable)
	t.Setenv("GRAPH_LAYER_ASA_FAKE_MODE", mode)
	asaEngineTestArgs = []string{"-test.run=^TestASAEngineFakeWorker$"}
	t.Cleanup(func() { asaEngineTestArgs = nil })
}

func TestASAPathEngineMissingWorkerFailsClosedWithoutPathLookup(t *testing.T) {
	t.Setenv("GRAPHLAYER_ENGINE", "graphlayer-engine")
	if _, err := asaPathEnginePath(); err == nil {
		t.Fatal("relative worker path should fail instead of searching PATH or falling back")
	}
	t.Setenv("GRAPHLAYER_ENGINE", filepath.Join(t.TempDir(), "missing-worker.exe"))
	if _, err := asaPathEnginePath(); err == nil {
		t.Fatal("missing explicitly configured worker should fail closed")
	}
	config, err := ParseASAConfig([]byte("hostname edge\n"))
	if err == nil || config != nil || !errors.Is(err, ErrAnalysisEngine) {
		t.Fatalf("ASA parse silently fell back without the Rust worker: config=%#v err=%v", config, err)
	}
}

func TestASAPathEngineRejectsNonRegularAndNonExecutableFiles(t *testing.T) {
	t.Setenv("GRAPHLAYER_ENGINE", t.TempDir())
	if _, err := asaPathEnginePath(); err == nil {
		t.Fatal("worker directory was accepted as an executable")
	}
	if runtime.GOOS == "windows" {
		return
	}
	worker := filepath.Join(t.TempDir(), "worker")
	if err := os.WriteFile(worker, []byte("not executable"), 0600); err != nil {
		t.Fatal("could not create worker fixture")
	}
	t.Setenv("GRAPHLAYER_ENGINE", worker)
	if _, err := asaPathEnginePath(); err == nil {
		t.Fatal("non-executable regular file was accepted as a worker")
	}
}

func TestASAPathEngineAcceptsOneStrictResponse(t *testing.T) {
	parsed, err := asa.ParseRunningConfig([]byte("hostname edge\n"))
	if err != nil {
		t.Fatal("could not parse ASA input")
	}
	useFakeASAEngine(t, "valid")
	response, err := callASAPathEngine(asaPathEngineRequest{
		ProtocolVersion: asaEngineProtocolVersion,
		Operation:       asaEngineOperation,
		Input: asaPathEngineInput{
			ArtifactSHA256: parsed.ArtifactSHA256,
			Lines:          []asaPathEngineLine{{Number: 1, Text: "hostname edge"}},
			NetworkObjects: map[string]asaPathEngineNetworkObject{},
		},
	})
	if err != nil {
		t.Fatalf("valid worker response rejected: %v", err)
	}
	if response.ProtocolVersion != asaEngineProtocolVersion || response.Operation != asaEngineOperation {
		t.Fatalf("worker protocol was not preserved: %#v", response)
	}
}

func TestASAPathInventoryWrapsWorkerFailuresWithSentinel(t *testing.T) {
	parsed, err := asa.ParseRunningConfig([]byte("hostname edge\n"))
	if err != nil {
		t.Fatal("could not parse ASA input")
	}
	useFakeASAEngine(t, "malformed")
	c := &Config{
		ArtifactSHA256: parsed.ArtifactSHA256,
		SourceLines:    []SourceLine{{Number: 1, Text: "[OMITTED NON-POLICY CONFIGURATION]"}},
		Evidence:       []Evidence{},
	}
	err = addASAPathInventory(c, parsed)
	if !errors.Is(err, ErrAnalysisEngine) {
		t.Fatalf("worker failure was not identifiable: %v", err)
	}
}

func TestASAPathEngineRejectsInvalidAndUnboundedWorkerResponses(t *testing.T) {
	cases := []struct {
		name string
		mode string
		want string
	}{
		{"timeout", "timeout", "timed out"},
		{"output overflow", "overflow", "size limit"},
		{"malformed JSON", "malformed", "malformed output"},
		{"trailing JSON", "trailing", "malformed output"},
		{"duplicate protocol field", "duplicate_protocol", "malformed output"},
		{"duplicate nested field", "duplicate_nested", "malformed output"},
		{"protocol mismatch", "wrong_protocol", "protocol mismatch"},
		{"nonzero exit hides stderr", "nonzero", "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useFakeASAEngine(t, tc.mode)
			if tc.mode == "timeout" {
				asaEngineTimeout = 40 * time.Millisecond
				t.Cleanup(func() { asaEngineTimeout = 10 * time.Second })
			}
			_, err := callASAPathEngine(asaPathEngineRequest{
				ProtocolVersion: asaEngineProtocolVersion,
				Operation:       asaEngineOperation,
				Input:           asaPathEngineInput{Lines: []asaPathEngineLine{}, NetworkObjects: map[string]asaPathEngineNetworkObject{}},
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("worker failure was not rejected safely: err=%v", err)
			}
			if strings.Contains(err.Error(), "sensitive-worker-canary") {
				t.Fatalf("worker stderr leaked into public error: %v", err)
			}
		})
	}
}

func TestASAEngineOutputLimitCannotBeBypassedByIOCopy(t *testing.T) {
	buffer := &asaEngineOutputBuffer{limit: 32}
	_, err := io.Copy(buffer, io.LimitReader(strings.NewReader(strings.Repeat("x", 64)), 64))
	if err == nil || !buffer.overflow || len(buffer.Bytes()) != 32 {
		t.Fatalf("stdout copy bypassed the size limit: error=%v overflow=%v bytes=%d", err, buffer.overflow, len(buffer.Bytes()))
	}
}

func TestASAPathOutputMergeDeduplicatesEvidenceAndAppliesLineUpdates(t *testing.T) {
	parsed, err := asa.ParseRunningConfig([]byte("hostname edge\nroute outside 0 0 192.0.2.1\n"))
	if err != nil {
		t.Fatal("could not parse ASA input")
	}
	firstID := evidenceID(parsed.ArtifactSHA256, []int{1})
	secondID := evidenceID(parsed.ArtifactSHA256, []int{2})
	c := &Config{
		ArtifactSHA256: parsed.ArtifactSHA256,
		SourceLines:    []SourceLine{{Number: 1, Text: "[OMITTED NON-POLICY CONFIGURATION]"}, {Number: 2, Text: "[OMITTED NON-POLICY CONFIGURATION]"}},
		Evidence:       []Evidence{{ID: firstID, StartLine: 1, EndLine: 1, Kind: "legacy", Summary: "retained"}},
	}
	path := PathConfig{Version: asaPathVersion, Interfaces: []PathInterface{}, Routes: []PathRoute{}, NATRules: []PathNATRule{}, Warnings: []Warning{}, EvidenceRefs: []string{secondID}}
	result := asaPathEngineResult{
		Evidence: []Evidence{
			{ID: firstID, StartLine: 1, EndLine: 1, Kind: "path_interface", Summary: "duplicate"},
			{ID: secondID, StartLine: 2, EndLine: 2, Kind: "static_route", Summary: "new evidence"},
		},
		SourceLineUpdates: []SourceLine{{Number: 2, Text: "[OMITTED UNSUPPORTED PATH CONFIGURATION]"}},
	}
	if err := mergeASAPathEngineOutput(c, parsed, path, result); err != nil {
		t.Fatalf("valid Rust output rejected: %v", err)
	}
	if len(c.Evidence) != 2 || c.Evidence[0].Summary != "retained" || c.Evidence[1].ID != secondID {
		t.Fatalf("evidence merge changed existing order or failed to deduplicate: %#v", c.Evidence)
	}
	if c.SourceLines[1].Text != "[OMITTED UNSUPPORTED PATH CONFIGURATION]" || c.Path == nil || c.Path.Version != asaPathVersion {
		t.Fatalf("path output was not merged: source=%#v path=%#v", c.SourceLines, c.Path)
	}
}

func TestASAPathOutputMergeRejectsInvalidLineUpdates(t *testing.T) {
	parsed, err := asa.ParseRunningConfig([]byte("hostname edge\n"))
	if err != nil {
		t.Fatal("could not parse ASA input")
	}
	c := &Config{SourceLines: []SourceLine{{Number: 1, Text: "redacted"}}}
	path := PathConfig{Version: asaPathVersion}
	err = mergeASAPathEngineOutput(c, parsed, path, asaPathEngineResult{Evidence: []Evidence{}, SourceLineUpdates: []SourceLine{{Number: 1, Text: "one"}, {Number: 1, Text: "duplicate"}}})
	if err == nil {
		t.Fatal("duplicate source line updates were accepted")
	}
}
