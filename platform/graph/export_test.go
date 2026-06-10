package graph

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mattpainter701/graphlayer/platform/models"
)

func TestGraphBuilder_Devices(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{
		Format: FormatGraphML,
		Title:  "Test Topology",
	})

	b.AddDevice(models.Device{
		ID:       "dev-1",
		Hostname: "core-sw01",
		Vendor:   "cisco",
		Model:    "Catalyst 9300",
		Version:  "17.9.4",
		Role:     "core",
		Site:     "DC1",
	})
	b.AddDevice(models.Device{
		ID:       "dev-2",
		Hostname: "fw-pa01",
		Vendor:   "paloalto",
		Model:    "PA-5250",
		Version:  "10.2.0",
		Role:     "firewall",
		Site:     "DC1",
	})

	graph := b.Build()

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	// Check vendor coloring
	n1 := graph.Nodes[0]
	n2 := graph.Nodes[1]

	if n1.Meta.Color != VendorColor("cisco") {
		t.Errorf("expected cisco color, got %s", n1.Meta.Color)
	}
	if n2.Meta.Color != VendorColor("paloalto") {
		t.Errorf("expected paloalto color, got %s", n2.Meta.Color)
	}

	// Cisco should be blue-ish, Palo Alto red-ish
	if !strings.HasPrefix(n1.Meta.Color, "#") {
		t.Error("color should be hex")
	}
}

func TestGraphBuilder_Assets(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML})

	b.AddAsset(models.AssetNode{
		Id:       "asset-1",
		IP:       "10.1.1.100",
		Hostname: "web-server-01",
		Vendor:   "vmware",
		OSName:   "Linux",
		Subnet:   "10.1.1.0/24",
	})

	graph := b.Build()

	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}

	n := graph.Nodes[0]
	if n.Meta.NodeType != "asset" {
		t.Errorf("expected asset type, got %s", n.Meta.NodeType)
	}
}

func TestGraphBuilder_Networks(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML})

	vlanID := 100
	b.AddNetwork(models.NetworkNode{
		Id:          "net-1",
		NetworkName: "VLAN100_Servers",
		Subnet:      "10.1.1.0/24",
		VLANID:      &vlanID,
		VLANName:    "Servers",
	})

	graph := b.Build()

	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}

	n := graph.Nodes[0]
	if n.Meta.VLANID != 100 {
		t.Errorf("expected VLANID 100, got %d", n.Meta.VLANID)
	}
	if n.Meta.Segment == "" {
		t.Error("expected a segment identifier")
	}
}

func TestGraphBuilder_Links(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{
		Format: FormatGraphML,
		Overlays: GraphOverlay{
			ConfidenceEdge: true,
		},
	})

	b.AddDevice(models.Device{ID: "dev-1", Hostname: "rtr01", Vendor: "cisco", Role: "router"})
	b.AddDevice(models.Device{ID: "dev-2", Hostname: "rtr02", Vendor: "juniper", Role: "router"})

	b.AddLink(models.Link{
		Id:      "link-1",
		FromID:  "dev-1",
		ToID:    "dev-2",
		EdgeName: "OSPF adjacency",
		Protocol: "OSPF",
	})

	graph := b.Build()

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}

	e := graph.Edges[0]
	if e.Meta.Thickness <= 1.0 {
		t.Errorf("expected thickness > 1.0, got %.2f", e.Meta.Thickness)
	}
}

func TestGraphBuilder_L2Links(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML})

	b.AddL2Link(models.L2Link{
		Key:          "l2-1",
		FromDevice:   "sw01",
		FromPort:     "Gi1/0/1",
		ToDevice:     "sw02",
		ToPort:       "Gi1/0/1",
		LinkType:     "trunk",
		AllowedVLANs: []int{10, 20, 30},
	})

	graph := b.Build()

	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}

	e := graph.Edges[0]
	if e.Meta.EdgeType != "l2" {
		t.Errorf("expected l2 edge type, got %s", e.Meta.EdgeType)
	}
	if e.Meta.Thickness == 0 {
		t.Error("L2 links should have thickness > 0")
	}
}

func TestGraphBuilder_FirewallPolicies(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{
		Format: FormatGraphML,
		Overlays: GraphOverlay{
			SecurityPolicy: true,
		},
	})

	b.AddFirewallPolicy(models.FirewallPolicy{
		ID:               "pol-1",
		Name:             "Allow_Web_to_App",
		Firewall:         "fw-pa01",
		SourceZones:      []string{"untrust"},
		DestinationZones: []string{"trust"},
		Action:           "allow",
		RuleOrder:        10,
	})

	graph := b.Build()

	// Should have 2 zone nodes + policy edge(s)
	if len(graph.Nodes) < 2 {
		t.Fatalf("expected at least 2 zone nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) < 1 {
		t.Fatalf("expected at least 1 policy edge, got %d", len(graph.Edges))
	}

	// Verify policy edge
	hasPolicy := false
	for _, e := range graph.Edges {
		if e.Meta.EdgeType == "policy" {
			hasPolicy = true
			if e.Meta.Action != "allow" {
				t.Errorf("expected action allow, got %s", e.Meta.Action)
			}
		}
	}
	if !hasPolicy {
		t.Error("expected a policy edge")
	}
}

func TestViewFilters(t *testing.T) {
	devices := []models.Device{
		{ID: "d1", Hostname: "rtr01", Vendor: "cisco", Role: "router"},
		{ID: "d2", Hostname: "sw01", Vendor: "cisco", Role: "access"},
		{ID: "d3", Hostname: "fw01", Vendor: "paloalto", Role: "firewall"},
		{ID: "d4", Hostname: "core01", Vendor: "juniper", Role: "core"},
	}

	tests := []struct {
		name     string
		filter   ViewFilter
		expected int
	}{
		{"all", ViewAll, 4},
		{"routing", ViewRoutingOnly, 2}, // router + core both match routing
		{"access", ViewAccessOnly, 1},
		{"security", ViewSecurityOnly, 1},
		{"core", ViewCoreOnly, 1},
		{"firewalls", ViewFirewallsOnly, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewGraphBuilder(ExportOptions{
				Format:     FormatGraphML,
				ViewFilter: tt.filter,
			})
			for _, d := range devices {
				b.AddDevice(d)
			}
			graph := b.Build()
			// Count non-zone, non-network nodes
			count := 0
			for _, n := range graph.Nodes {
				if n.Meta.NodeType != "zone" && n.Meta.NodeType != "network" {
					count++
				}
			}
			if count != tt.expected {
				t.Errorf("filter %s: expected %d nodes, got %d", tt.filter, tt.expected, count)
			}
		})
	}
}

func TestVLANGrouping(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{
		Format: FormatGraphML,
		Overlays: GraphOverlay{
			VLANGrouping: true,
		},
	})

	vlan10 := 10
	vlan20 := 20

	b.AddNetwork(models.NetworkNode{
		Id: "n1", NetworkName: "VLAN10_Web", Subnet: "10.1.1.0/24",
		VLANID: &vlan10, VLANName: "Web",
	})
	b.AddNetwork(models.NetworkNode{
		Id: "n2", NetworkName: "VLAN10_App", Subnet: "10.1.2.0/24",
		VLANID: &vlan10, VLANName: "Web",
	})
	b.AddNetwork(models.NetworkNode{
		Id: "n3", NetworkName: "VLAN20_DB", Subnet: "10.1.3.0/24",
		VLANID: &vlan20, VLANName: "Database",
	})

	graph := b.Build()

	// Should have grouping nodes for each segment
	hasSegment := false
	for _, n := range graph.Nodes {
		if n.Meta.NodeType == "segment_group" {
			hasSegment = true
		}
	}
	if !hasSegment {
		t.Error("expected segment_group nodes with VLAN grouping enabled")
	}
}

func TestGraphMLEmpty(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML, Title: "Empty"})
	graph := b.Build()
	data, err := ExportGraphML(graph)
	if err != nil {
		t.Fatalf("GraphML export failed: %v", err)
	}
	if !strings.HasPrefix(string(data), xml.Header) {
		t.Error("GraphML should start with XML header")
	}
	if !strings.Contains(string(data), "<graphml") {
		t.Error("GraphML should contain <graphml> element")
	}
}

func TestGraphMLRoundTrip(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML, Title: "Test Network"})

	b.AddDevice(models.Device{
		ID: "dev-1", Hostname: "router01", Vendor: "cisco", Model: "ISR4451", Role: "router", Site: "NYC",
	})
	b.AddDevice(models.Device{
		ID: "dev-2", Hostname: "switch01", Vendor: "arista", Model: "7050SX", Role: "core", Site: "NYC",
	})
	b.AddLink(models.Link{
		Id: "link-1", FromID: "dev-1", ToID: "dev-2", EdgeName: "10G SFP+", Protocol: "Ethernet",
	})

	graph := b.Build()
	data, err := ExportGraphML(graph)
	if err != nil {
		t.Fatalf("GraphML export failed: %v", err)
	}

	// Parse back to verify structure
	var doc graphmlDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("GraphML parse failed: %v", err)
	}
	if len(doc.Graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes in XML, got %d", len(doc.Graph.Nodes))
	}
	if len(doc.Graph.Edges) != 1 {
		t.Errorf("expected 1 edge in XML, got %d", len(doc.Graph.Edges))
	}
}

func TestGEXFRoundTrip(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGEXF, Title: "Test GEXF"})

	b.AddDevice(models.Device{ID: "d1", Hostname: "fw01", Vendor: "fortinet", Role: "firewall"})
	b.AddDevice(models.Device{ID: "d2", Hostname: "rtr01", Vendor: "cisco", Role: "router"})
	b.AddLink(models.Link{Id: "l1", FromID: "d1", ToID: "d2", EdgeName: "Trunk", Protocol: "TCP"})

	graph := b.Build()
	data, err := ExportGEXF(graph)
	if err != nil {
		t.Fatalf("GEXF export failed: %v", err)
	}
	if !strings.HasPrefix(string(data), xml.Header) {
		t.Error("GEXF should start with XML header")
	}
	if !strings.Contains(string(data), "<gexf") {
		t.Error("GEXF should contain <gexf> element")
	}
}

func TestCytoscapeJSON(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatCytoscape, Title: "Cyto Test"})

	b.AddDevice(models.Device{ID: "d1", Hostname: "core01", Vendor: "juniper", Role: "core"})
	b.AddDevice(models.Device{ID: "d2", Hostname: "edge01", Vendor: "cisco", Role: "edge"})
	b.AddLink(models.Link{Id: "l1", FromID: "d1", ToID: "d2", EdgeName: "IS-IS"})

	graph := b.Build()
	data, err := ExportCytoscape(graph, true)
	if err != nil {
		t.Fatalf("Cytoscape export failed: %v", err)
	}

	var cy CytoscapeGraph
	if err := json.Unmarshal(data, &cy); err != nil {
		t.Fatalf("Cytoscape JSON parse failed: %v", err)
	}
	if len(cy.Elements.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(cy.Elements.Nodes))
	}
	if len(cy.Elements.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(cy.Elements.Edges))
	}
	if cy.Layout == nil || cy.Layout.Name == "" {
		t.Error("expected layout configuration")
	}
	if len(cy.Style) == 0 {
		t.Error("expected style rules for vendor-colored output")
	}
}

func TestD3JSON(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatD3, Title: "D3 Test"})

	b.AddDevice(models.Device{ID: "d1", Hostname: "sw01", Vendor: "arista", Role: "switch"})
	b.AddDevice(models.Device{ID: "d2", Hostname: "sw02", Vendor: "arista", Role: "switch"})
	b.AddL2Link(models.L2Link{Key: "l2-1", FromDevice: "d1", ToDevice: "d2", FromPort: "Et1", ToPort: "Et1", LinkType: "trunk"})

	graph := b.Build()
	data, err := ExportD3(graph)
	if err != nil {
		t.Fatalf("D3 export failed: %v", err)
	}

	var d3g D3Graph
	if err := json.Unmarshal(data, &d3g); err != nil {
		t.Fatalf("D3 JSON parse failed: %v", err)
	}
	if len(d3g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(d3g.Nodes))
	}
	if len(d3g.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(d3g.Links))
	}
	if d3g.Meta == nil || d3g.Meta.Title != "D3 Test" {
		t.Error("expected D3 metadata with title")
	}
}

func TestD3HTMLTemplate(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatD3, Title: "HTML Test"})
	b.AddDevice(models.Device{ID: "d1", Hostname: "test01", Vendor: "cisco", Role: "router"})

	graph := b.Build()
	jsonData, err := ExportD3(graph)
	if err != nil {
		t.Fatalf("D3 export failed: %v", err)
	}

	html := D3HTMLTemplate(jsonData)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML template should start with DOCTYPE")
	}
	if !strings.Contains(html, "d3js.org") {
		t.Error("HTML template should include D3 CDN")
	}
	if !strings.Contains(html, "data.nodes") {
		t.Error("HTML template should contain data")
	}
}

func TestExportDispatcher(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML, Title: "Test"})
	b.AddDevice(models.Device{ID: "d1", Hostname: "test", Vendor: "cisco", Role: "router"})
	graph := b.Build()

	formats := SupportedFormats()
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			g := graph
			g.Options.Format = f
			data, err := Export(g)
			if err != nil {
				t.Errorf("export %s failed: %v", f, err)
			}
			if len(data) == 0 {
				t.Errorf("export %s returned empty data", f)
			}
		})
	}
}

func TestQuickExport(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{})
	b.AddDevice(models.Device{ID: "d1", Hostname: "quick", Vendor: "cisco", Role: "router"})

	data, err := QuickExport(b, FormatGraphML, "Quick Test")
	if err != nil {
		t.Fatalf("QuickExport failed: %v", err)
	}
	if !strings.Contains(string(data), "<graphml") {
		t.Error("QuickExport should produce GraphML")
	}
}

func TestVendorColors(t *testing.T) {
	tests := []struct {
		vendor string
		color  string
	}{
		{"cisco", "#049FD9"},
		{"paloalto", "#FA582D"},
		{"fortinet", "#EE3124"},
		{"juniper", "#84B135"},
		{"arista", "#0072CE"},
		{"aruba", "#FF8300"},
		{"dell", "#007DB8"},
	}

	for _, tt := range tests {
		c := VendorColor(tt.vendor)
		if c != tt.color {
			t.Errorf("VendorColor(%s) = %s, want %s", tt.vendor, c, tt.color)
		}
	}

	// Unknown vendor
	if c := VendorColor("nonexistent"); c != VendorColors["unknown"] {
		t.Errorf("unknown vendor should get default color, got %s", c)
	}

	// Empty vendor
	if c := VendorColor(""); c != VendorColors["unknown"] {
		t.Errorf("empty vendor should get default color, got %s", c)
	}
}

func TestConfidenceToThickness(t *testing.T) {
	if v := confidenceToThickness(0.0); v != 1.0 {
		t.Errorf("0 confidence should be thickness 1.0, got %.2f", v)
	}
	if v := confidenceToThickness(0.5); v != 3.0 {
		t.Errorf("0.5 confidence should be thickness 3.0, got %.2f", v)
	}
	if v := confidenceToThickness(1.0); v != 5.0 {
		t.Errorf("1.0 confidence should be thickness 5.0, got %.2f", v)
	}
}

func TestSecurityPolicyDisabled(t *testing.T) {
	// Without policy overlay enabled, policies should not produce edges
	b := NewGraphBuilder(ExportOptions{
		Format: FormatGraphML,
		Overlays: GraphOverlay{
			SecurityPolicy: false,
		},
	})

	b.AddFirewallPolicy(models.FirewallPolicy{
		ID: "pol-1", Name: "Test", Firewall: "fw01",
		SourceZones: []string{"untrust"}, DestinationZones: []string{"trust"},
		Action: "allow",
	})

	graph := b.Build()
	for _, e := range graph.Edges {
		if e.Meta.EdgeType == "policy" {
			t.Error("policy edges should not appear when SecurityPolicy overlay is disabled")
		}
	}
}

func TestDuplicateDevicePrevention(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML})
	b.AddDevice(models.Device{ID: "d1", Hostname: "sw01", Vendor: "cisco", Role: "switch"})
	b.AddDevice(models.Device{ID: "d1", Hostname: "sw01", Vendor: "cisco", Role: "switch"})

	graph := b.Build()
	if len(graph.Nodes) != 1 {
		t.Errorf("duplicate device should be prevented, got %d nodes", len(graph.Nodes))
	}
}

func TestExportToHTML(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatD3, Title: "HTML Export"})
	b.AddDevice(models.Device{ID: "d1", Hostname: "test01", Vendor: "cisco", Role: "router"})
	graph := b.Build()

	html, err := ExportToHTML(graph)
	if err != nil {
		t.Fatalf("ExportToHTML failed: %v", err)
	}
	if !strings.Contains(string(html), "<script>") {
		t.Error("HTML output should contain script tags")
	}
}

func TestExportToHTML_WrongFormat(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatGraphML, Title: "Wrong"})
	b.AddDevice(models.Device{ID: "d1", Hostname: "test", Vendor: "cisco", Role: "router"})
	graph := b.Build()

	_, err := ExportToHTML(graph)
	if err == nil {
		t.Error("ExportToHTML with GraphML format should fail")
	}
}

func TestSupportedFormats(t *testing.T) {
	fmts := SupportedFormats()
	if len(fmts) != 5 {
		t.Errorf("expected 5 supported formats, got %d", len(fmts))
	}
	seen := make(map[ExportFormat]bool)
	for _, f := range fmts {
		seen[f] = true
	}
	for _, want := range []ExportFormat{FormatGraphML, FormatGEXF, FormatCytoscape, FormatD3, FormatDOT} {
		if !seen[want] {
			t.Errorf("missing format %s", want)
		}
	}
}

func TestDOTExport(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatDOT, Title: "DOT Test"})

	b.AddDevice(models.Device{ID: "d1", Hostname: "router01", Vendor: "cisco", Role: "router"})
	b.AddDevice(models.Device{ID: "d2", Hostname: "switch01", Vendor: "arista", Role: "switch"})
	b.AddLink(models.Link{Id: "l1", FromID: "d1", ToID: "d2", EdgeName: "GE1/0/1", Protocol: "OSPF"})

	graph := b.Build()
	data, err := ExportDOT(graph)
	if err != nil {
		t.Fatalf("DOT export failed: %v", err)
	}

	dot := string(data)
	if !strings.Contains(dot, "digraph G {") {
		t.Error("DOT output should contain 'digraph G {'")
	}
	if !strings.Contains(dot, "router01") {
		t.Error("DOT output should contain router01")
	}
	if !strings.Contains(dot, "switch01") {
		t.Error("DOT output should contain switch01")
	}
	if !strings.Contains(dot, "->") {
		t.Error("DOT output should contain edge arrows")
	}
	if !strings.Contains(dot, "}") {
		t.Error("DOT output should close with }")
	}
}

func TestDOTExportEmpty(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatDOT, Title: "Empty DOT"})
	graph := b.Build()
	data, err := ExportDOT(graph)
	if err != nil {
		t.Fatalf("DOT export of empty graph failed: %v", err)
	}
	dot := string(data)
	if !strings.Contains(dot, "digraph G {") {
		t.Error("empty DOT should still have digraph header")
	}
}

func TestDOTExportNilGraph(t *testing.T) {
	_, err := ExportToDOTCustom(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil graph")
	}
}

func TestDOTCustomAttrs(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatDOT, Title: "Custom"})
	b.AddDevice(models.Device{ID: "r1", Hostname: "core", Vendor: "cisco", Role: "router"})
	b.AddDevice(models.Device{ID: "s1", Hostname: "edge", Vendor: "juniper", Role: "switch"})
	b.AddLink(models.Link{Id: "e1", FromID: "r1", ToID: "s1", EdgeName: "uplink"})

	graph := b.Build()

	nodeAttrs := map[string]map[string]string{
		"r1": {"color": "blue", "shape": "circle"},
	}
	edgeAttrs := map[string]map[string]string{
		"r1->s1": {"style": "dashed", "color": "green"},
	}

	data, err := ExportToDOTCustom(&graph, nodeAttrs, edgeAttrs)
	if err != nil {
		t.Fatalf("DOT custom export failed: %v", err)
	}

	dot := string(data)
	if !strings.Contains(dot, `color="blue"`) {
		t.Error("custom node color should be applied")
	}
	if !strings.Contains(dot, `style="dashed"`) {
		t.Error("custom edge style should be applied")
	}
}

func TestDOTVendorColors(t *testing.T) {
	b := NewGraphBuilder(ExportOptions{Format: FormatDOT})
	b.AddDevice(models.Device{ID: "d1", Hostname: "fw01", Vendor: "paloalto", Role: "firewall"})
	b.AddDevice(models.Device{ID: "d2", Hostname: "rtr01", Vendor: "juniper", Role: "router"})

	graph := b.Build()
	data, err := ExportDOT(graph)
	if err != nil {
		t.Fatalf("DOT export failed: %v", err)
	}

	dot := string(data)
	// Should contain vendor colors as fillcolor
	if !strings.Contains(dot, "fillcolor") {
		t.Error("DOT output should contain vendor-based fillcolor")
	}
	// Firewall should be doubleoctagon shape
	if !strings.Contains(dot, "doubleoctagon") {
		t.Error("firewall should have doubleoctagon shape")
	}
	// Router should be diamond shape
	if !strings.Contains(dot, "diamond") {
		t.Error("router should have diamond shape")
	}
}

func TestDOTIDQuoting(t *testing.T) {
	ids := []struct {
		input    string
		shouldQ  bool // should be quoted
	}{
		{"simple", false},
		{"node-1", true},  // hyphen
		{"node.1", true},  // dot
		{"digraph", true}, // keyword
		{"123", false},
	}

	for _, tt := range ids {
		result := dotID(tt.input)
		quoted := strings.HasPrefix(result, "\"")
		if quoted != tt.shouldQ {
			t.Errorf("dotID(%q): expected quoted=%v, got %q", tt.input, tt.shouldQ, result)
		}
	}
}
