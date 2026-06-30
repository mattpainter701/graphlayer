package graph

import (
	"fmt"
	"sort"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// AddDevice adds a Device to the graph as a node.
func (b *GraphBuilder) AddDevice(d models.Device) {
	id := deviceNodeID(d)
	if _, exists := b.nodes[id]; exists {
		return
	}
	b.networkDevs[id] = d
	node := GraphNode{
		ID:    id,
		Label: d.Hostname,
		Meta: NodeWithMeta{
			Label:    d.Hostname,
			NodeType: classifyDeviceRole(d),
			Vendor:   d.Vendor,
			Role:     d.Role,
		},
		Attributes: map[string]string{
			"vendor":  d.Vendor,
			"model":   d.Model,
			"version": d.Version,
			"role":    d.Role,
			"site":    d.Site,
		},
	}
	node.Meta.Color = VendorColor(d.Vendor)
	b.nodes[id] = node
}

// AddAsset adds an AssetNode to the graph.
func (b *GraphBuilder) AddAsset(a models.AssetNode) {
	id := assetNodeID(a)
	if _, exists := b.nodes[id]; exists {
		return
	}
	b.assets[id] = a
	label := a.Hostname
	if label == "" {
		label = a.IP
	}
	node := GraphNode{
		ID:    id,
		Label: label,
		Meta: NodeWithMeta{
			Label:    label,
			NodeType: "asset",
			Vendor:   a.Vendor,
			Subnet:   a.Subnet,
		},
		Attributes: map[string]string{
			"ip":       a.IP,
			"subnet":   a.Subnet,
			"vendor":   a.Vendor,
			"hostname": a.Hostname,
			"os":       a.OSName,
			"role":     a.Role,
		},
	}
	node.Meta.Color = VendorColor(a.Vendor)
	b.nodes[id] = node
}

// AddNetwork adds a NetworkNode to the graph.
func (b *GraphBuilder) AddNetwork(n models.NetworkNode) {
	id := networkNodeID(n)
	if _, exists := b.nodes[id]; exists {
		return
	}
	b.networks[id] = n
	node := GraphNode{
		ID:    id,
		Label: n.NetworkName,
		Meta: NodeWithMeta{
			Label:    n.NetworkName,
			NodeType: "network",
			Subnet:   n.Subnet,
		},
		Attributes: map[string]string{
			"subnet":       n.Subnet,
			"network_name": n.NetworkName,
			"criticality":  n.Criticality,
		},
	}
	if n.VLANID != nil {
		node.Meta.VLANID = *n.VLANID
		node.Attributes["vlan_id"] = fmt.Sprintf("%d", *n.VLANID)
	}
	if n.VLANName != "" {
		node.Meta.VLANName = n.VLANName
		node.Attributes["vlan_name"] = n.VLANName
	}
	if n.VLANID != nil {
		node.Meta.Segment = segmentFromVLAN(*n.VLANID, n.VLANName, n.Subnet)
	} else {
		node.Meta.Segment = segmentFromVLAN(0, n.VLANName, n.Subnet)
	}
	node.Meta.Color = "#4CAF50" // green for networks
	b.nodes[id] = node
}

// AddLink adds a Link to the graph as an edge.
func (b *GraphBuilder) AddLink(l models.Link) {
	srcID := l.FromID
	if srcID == "" {
		srcID = l.Source
	}
	dstID := l.ToID
	if dstID == "" {
		dstID = l.Destination
	}

	edge := GraphEdge{
		ID:     l.Id,
		Source: srcID,
		Target: dstID,
		Label:  l.EdgeName,
		Meta: EdgeWithMeta{
			Label:      l.EdgeName,
			EdgeType:   classifyLinkType(l),
			Protocol:   l.Protocol,
			Confidence: confidenceFromLifecycle(l.Lifecycle),
		},
		Attributes: map[string]string{
			"protocol": l.Protocol,
			"port":     l.Port,
		},
	}
	edge.Meta.Thickness = confidenceToThickness(edge.Meta.Confidence)
	b.edges = append(b.edges, edge)
}

// AddL2Link adds an L2 adjacency edge.
func (b *GraphBuilder) AddL2Link(l models.L2Link) {
	edge := GraphEdge{
		ID:     l.Key,
		Source: l.FromDevice,
		Target: l.ToDevice,
		Label:  fmt.Sprintf("%s ↔ %s", l.FromPort, l.ToPort),
		Meta: EdgeWithMeta{
			Label:    fmt.Sprintf("%s ↔ %s", l.FromPort, l.ToPort),
			EdgeType: "l2",
			VLANIDs:  l.AllowedVLANs,
		},
		Attributes: map[string]string{
			"link_type": l.LinkType,
			"from_port": l.FromPort,
			"to_port":   l.ToPort,
		},
	}
	if l.NativeVLAN != nil {
		edge.Attributes["native_vlan"] = fmt.Sprintf("%d", *l.NativeVLAN)
	}
	// default mid-confidence for L2 adjacencies
	edge.Meta.Confidence = 0.75
	edge.Meta.Thickness = confidenceToThickness(edge.Meta.Confidence)
	b.edges = append(b.edges, edge)
}

// AddFirewallPolicy adds a FirewallPolicy as a policy overlay edge.
func (b *GraphBuilder) AddFirewallPolicy(p models.FirewallPolicy) {
	if !b.options.Overlays.SecurityPolicy {
		return
	}
	// Create policy edges between source and destination zones
	// For each source/dest zone pair, create an annotated edge
	for _, srcZone := range p.SourceZones {
		for _, dstZone := range p.DestinationZones {
			srcID := zoneNodeID(p.Firewall, srcZone)
			dstID := zoneNodeID(p.Firewall, dstZone)

			// Ensure zone nodes exist
			b.ensureZoneNode(p.Firewall, srcZone)
			b.ensureZoneNode(p.Firewall, dstZone)

			edge := GraphEdge{
				ID:     fmt.Sprintf("policy-%s-%s-%s", p.ID, srcZone, dstZone),
				Source: srcID,
				Target: dstID,
				Label:  p.Name,
				Meta: EdgeWithMeta{
					Label:      p.Name,
					EdgeType:   "policy",
					Action:     p.Action,
					RuleName:   p.Name,
					Confidence: 0.9,
				},
				Attributes: map[string]string{
					"action":      p.Action,
					"rule_name":   p.Name,
					"firewall":    p.Firewall,
					"rule_order":  fmt.Sprintf("%d", p.RuleOrder),
					"description": p.Description,
				},
			}
			edge.Meta.Thickness = 2.0
			b.edges = append(b.edges, edge)
		}
	}
}

// ensureZoneNode creates a zone node if it doesn't exist.
func (b *GraphBuilder) ensureZoneNode(firewall, zone string) {
	id := zoneNodeID(firewall, zone)
	if _, exists := b.nodes[id]; exists {
		return
	}
	b.nodes[id] = GraphNode{
		ID:    id,
		Label: fmt.Sprintf("%s:%s", firewall, zone),
		Meta: NodeWithMeta{
			Label:    zone,
			NodeType: "zone",
		},
		Attributes: map[string]string{
			"zone":     zone,
			"firewall": firewall,
		},
	}
}

// Build applies all filters and overlays, returning the final TopologyGraph.
func (b *GraphBuilder) Build() TopologyGraph {
	// Apply view filter
	filtered := b.applyViewFilter()

	// Apply VLAN/segment grouping
	if b.options.Overlays.VLANGrouping {
		filtered = b.applyVLANGrouping(filtered)
	}

	graph := TopologyGraph{
		Title:       b.options.Title,
		Description: b.options.Description,
		Nodes:       filtered,
		Edges:       b.edges,
		Options:     b.options,
	}

	// Sort nodes and edges for deterministic output
	sort.Slice(graph.Nodes, func(i, j int) bool {
		return graph.Nodes[i].ID < graph.Nodes[j].ID
	})
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].Source == graph.Edges[j].Source {
			return graph.Edges[i].Target < graph.Edges[j].Target
		}
		return graph.Edges[i].Source < graph.Edges[j].Source
	})

	return graph
}

// applyViewFilter filters nodes based on the view filter option.
func (b *GraphBuilder) applyViewFilter() []GraphNode {
	filter := b.options.ViewFilter
	if filter == "" || filter == ViewAll {
		nodes := make([]GraphNode, 0, len(b.nodes))
		for _, n := range b.nodes {
			nodes = append(nodes, n)
		}
		return nodes
	}

	allowedRoles := viewFilterRoles(filter)
	nodes := make([]GraphNode, 0)
	for _, n := range b.nodes {
		if n.Meta.NodeType == "zone" || n.Meta.NodeType == "network" {
			nodes = append(nodes, n)
			continue
		}
		for _, role := range allowedRoles {
			if n.Meta.Role == role || n.Meta.NodeType == role {
				nodes = append(nodes, n)
				break
			}
		}
	}
	return nodes
}

// applyVLANGrouping groups nodes by VLAN segment.
func (b *GraphBuilder) applyVLANGrouping(nodes []GraphNode) []GraphNode {
	segments := make(map[string][]GraphNode)
	var ungrouped []GraphNode

	for _, n := range nodes {
		seg := n.Meta.Segment
		if seg == "" {
			ungrouped = append(ungrouped, n)
			continue
		}
		segments[seg] = append(segments[seg], n)
	}

	result := make([]GraphNode, 0)
	for seg, groupNodes := range segments {
		if len(groupNodes) == 1 {
			result = append(result, groupNodes...)
			continue
		}
		// Create a grouping node
		groupID := "segment-" + seg
		childIDs := make([]string, len(groupNodes))
		for i, n := range groupNodes {
			childIDs[i] = n.ID
		}
		groupNode := GraphNode{
			ID:    groupID,
			Label: fmt.Sprintf("Segment: %s (%d devices)", seg, len(groupNodes)),
			Meta: NodeWithMeta{
				Label:    seg,
				NodeType: "segment_group",
				Segment:  seg,
			},
			Attributes: map[string]string{
				"segment":    seg,
				"node_count": fmt.Sprintf("%d", len(groupNodes)),
				"children":   fmt.Sprintf("%v", childIDs),
			},
		}
		result = append(result, groupNode)
		result = append(result, groupNodes...)
	}
	result = append(result, ungrouped...)
	return result
}

// viewFilterRoles maps a view filter to device roles.
func viewFilterRoles(f ViewFilter) []string {
	switch f {
	case ViewRoutingOnly:
		return []string{"router", "edge", "core", "routing"}
	case ViewAccessOnly:
		return []string{"access", "switch", "distribution"}
	case ViewSecurityOnly:
		return []string{"firewall", "perimeter", "security"}
	case ViewCoreOnly:
		return []string{"core"}
	case ViewFirewallsOnly:
		return []string{"firewall", "perimeter"}
	case ViewSwitchesOnly:
		return []string{"switch", "access", "distribution"}
	default:
		return nil
	}
}

// classifyDeviceRole determines the role/type for a device.
func classifyDeviceRole(d models.Device) string {
	role := d.Role
	switch role {
	case "edge", "core", "distribution", "access":
		return role
	case "firewall", "perimeter":
		return "firewall"
	default:
		if role == "" {
			return "device"
		}
		return role
	}
}

// classifyLinkType determines the edge type from a Link.
func classifyLinkType(l models.Link) string {
	if l.IsOTProtocol {
		return "ot"
	}
	if l.IsRelationship {
		return "relationship"
	}
	if l.IsDirected {
		return "directed"
	}
	if l.Protocol != "" {
		return "l3"
	}
	return "link"
}

// confidenceFromLifecycle extracts a confidence score from a RecordLifecycle.
func confidenceFromLifecycle(lc models.RecordLifecycle) float64 {
	// Default moderate confidence
	return 0.5
}

// confidenceToThickness maps confidence (0-1) to edge thickness (1-5).
func confidenceToThickness(c float64) float64 {
	if c <= 0 {
		return 1.0
	}
	return 1.0 + c*4.0 // 1.0 to 5.0
}

// segmentFromVLAN creates a segment identifier from VLAN info.
func segmentFromVLAN(vlanID int, vlanName, subnet string) string {
	if vlanID > 0 && vlanName != "" {
		return fmt.Sprintf("VLAN%d_%s", vlanID, vlanName)
	}
	if vlanID > 0 {
		return fmt.Sprintf("VLAN%d", vlanID)
	}
	if vlanName != "" {
		return vlanName
	}
	if subnet != "" {
		return "subnet_" + subnet
	}
	return ""
}

// ID generators
func deviceNodeID(d models.Device) string {
	if d.ID != "" {
		return d.ID
	}
	return "device:" + d.Hostname
}

func assetNodeID(a models.AssetNode) string {
	if a.Id != "" {
		return a.Id
	}
	if a.Hostname != "" {
		return "asset:" + a.Hostname
	}
	return "asset:" + a.IP
}

func networkNodeID(n models.NetworkNode) string {
	if n.Id != "" {
		return n.Id
	}
	return "network:" + n.NetworkName
}

func zoneNodeID(firewall, zone string) string {
	return fmt.Sprintf("zone:%s:%s", firewall, zone)
}
