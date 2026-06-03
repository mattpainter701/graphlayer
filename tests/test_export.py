"""
Round-trip tests for topology graph export formats.
"""
import json
import pytest
from platform.export import (
    export_to_graphml, parse_graphml,
    export_to_cytoscape, parse_cytoscape,
    export_to_analyst_bundle, parse_analyst_bundle,
    export_graph, parse_graph,
)

# Sample graph matching the query DSL test fixture
SAMPLE_GRAPH = {
    "nodes": [
        {"id": "core", "name": "core-router", "role": "core"},
        {"id": "dist", "name": "distribution-switch", "role": "distribution"},
        {"id": "edge", "name": "edge-firewall", "role": "security"},
    ],
    "edges": [
        {
            "source_id": "core",
            "target_id": "dist",
            "protocol": "LLDP",
            "relationship": "uplink",
            "confidence": 0.94,
        },
        {
            "source_id": "dist",
            "target_id": "edge",
            "protocol": "CDP",
            "relationship": "transit",
            "confidence": 0.72,
        },
    ],
}

MINIMAL_GRAPH = {
    "nodes": [
        {"id": "a", "name": "Node A"},
        {"id": "b", "name": "Node B"},
    ],
    "edges": [
        {
            "source_id": "a",
            "target_id": "b",
            "protocol": "OSPF",
            "confidence": 1.0,
        },
    ],
}

SINGLE_NODE_GRAPH = {
    "nodes": [{"id": "only", "name": "Lonely Node", "role": "leaf"}],
    "edges": [],
}


def _core_edges(graph: dict) -> list:
    """Extract core edge topology: (source, target, protocol, confidence)."""
    result = []
    for e in graph.get("edges", []):
        result.append((
            e.get("source_id", ""),
            e.get("target_id", ""),
            e.get("protocol", None),
            e.get("relationship", None),
            e.get("confidence", None),
        ))
    return sorted(result)


def _assert_same_topology(got: dict, expected: dict):
    """Assert two graphs have the same core topology (nodes and edge connectivity)."""
    # Compare node sets
    got_nodes = {(n["id"], n.get("name"), n.get("role")) for n in got.get("nodes", [])}
    exp_nodes = {(n["id"], n.get("name"), n.get("role")) for n in expected.get("nodes", [])}
    assert got_nodes == exp_nodes, f"Node mismatch: got {got_nodes}, expected {exp_nodes}"
    
    # Compare core edge properties
    assert _core_edges(got) == _core_edges(expected), (
        f"Edge mismatch: got {_core_edges(got)}, expected {_core_edges(expected)}"
    )


class TestGraphMLRoundTrip:
    def test_round_trip_sample(self):
        exported = export_to_graphml(SAMPLE_GRAPH)
        parsed = parse_graphml(exported)
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_round_trip_minimal(self):
        exported = export_to_graphml(MINIMAL_GRAPH)
        parsed = parse_graphml(exported)
        _assert_same_topology(parsed, MINIMAL_GRAPH)

    def test_round_trip_single_node(self):
        exported = export_to_graphml(SINGLE_NODE_GRAPH)
        parsed = parse_graphml(exported)
        _assert_same_topology(parsed, SINGLE_NODE_GRAPH)

    def test_output_is_valid_xml(self):
        xml_str = export_to_graphml(SAMPLE_GRAPH)
        import xml.etree.ElementTree as ET
        root = ET.fromstring(xml_str)
        assert root.tag.endswith("graphml")

    def test_preserves_confidence_type(self):
        exported = export_to_graphml(SAMPLE_GRAPH)
        parsed = parse_graphml(exported)
        for edge in parsed["edges"]:
            if "confidence" in edge:
                assert isinstance(edge["confidence"], float)

    def test_node_count_and_edge_count(self):
        exported = export_to_graphml(SAMPLE_GRAPH)
        parsed = parse_graphml(exported)
        assert len(parsed["nodes"]) == len(SAMPLE_GRAPH["nodes"])
        assert len(parsed["edges"]) == len(SAMPLE_GRAPH["edges"])


class TestCytoscapeRoundTrip:
    def test_round_trip_sample(self):
        exported = export_to_cytoscape(SAMPLE_GRAPH)
        parsed = parse_cytoscape(exported)
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_round_trip_minimal(self):
        exported = export_to_cytoscape(MINIMAL_GRAPH)
        parsed = parse_cytoscape(exported)
        _assert_same_topology(parsed, MINIMAL_GRAPH)

    def test_round_trip_single_node(self):
        exported = export_to_cytoscape(SINGLE_NODE_GRAPH)
        parsed = parse_cytoscape(exported)
        _assert_same_topology(parsed, SINGLE_NODE_GRAPH)

    def test_output_is_valid_json(self):
        json_str = export_to_cytoscape(SAMPLE_GRAPH)
        data = json.loads(json_str)
        assert "elements" in data
        assert isinstance(data["elements"], list)

    def test_element_structure(self):
        json_str = export_to_cytoscape(SAMPLE_GRAPH)
        data = json.loads(json_str)
        for el in data["elements"]:
            assert "data" in el
            if "source" in el["data"]:
                assert "target" in el["data"]


class TestAnalystBundleRoundTrip:
    def test_round_trip_sample(self):
        exported = export_to_analyst_bundle(SAMPLE_GRAPH)
        parsed = parse_analyst_bundle(exported)
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_round_trip_minimal(self):
        exported = export_to_analyst_bundle(MINIMAL_GRAPH)
        parsed = parse_analyst_bundle(exported)
        _assert_same_topology(parsed, MINIMAL_GRAPH)

    def test_round_trip_pretty(self):
        exported = export_to_analyst_bundle(SAMPLE_GRAPH, pretty=True)
        parsed = parse_analyst_bundle(exported)
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_compact_output(self):
        json_str = export_to_analyst_bundle(SAMPLE_GRAPH, pretty=False)
        assert "\n" not in json_str, "Non-pretty output should be single line"
        data = json.loads(json_str)
        assert data["v"] == 1
        assert data["t"] == "topology"

    def test_short_keys(self):
        json_str = export_to_analyst_bundle(SAMPLE_GRAPH)
        data = json.loads(json_str)
        for node in data["n"]:
            assert "i" in node  # id compacted to i
        for edge in data["e"]:
            assert "s" in edge  # source_id compacted to s
            assert "t" in edge  # target_id compacted to t


class TestExportDispatcher:
    def test_graphml_via_dispatcher(self):
        result = export_graph(SAMPLE_GRAPH, "graphml")
        parsed = parse_graph(result, "graphml")
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_cytoscape_via_dispatcher(self):
        result = export_graph(SAMPLE_GRAPH, "cytoscape")
        parsed = parse_graph(result, "cytoscape")
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_analyst_bundle_via_dispatcher(self):
        result = export_graph(SAMPLE_GRAPH, "analyst-bundle")
        parsed = parse_graph(result, "analyst-bundle")
        _assert_same_topology(parsed, SAMPLE_GRAPH)

    def test_unsupported_format_raises(self):
        with pytest.raises(ValueError, match="Unsupported export format"):
            export_graph(SAMPLE_GRAPH, "unsupported")
        with pytest.raises(ValueError, match="Unsupported format"):
            parse_graph("{}", "unsupported")
