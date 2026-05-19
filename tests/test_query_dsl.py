import pytest

from platform.query_dsl import SavedQueryStore, filter_graph, parse_query


GRAPH = {
    "nodes": [
        {"id": "core", "name": "core-router"},
        {"id": "dist", "name": "distribution-switch"},
        {"id": "edge", "name": "edge-firewall"},
    ],
    "edges": [
        {
            "source_id": "core",
            "source": "core",
            "source_name": "core-router",
            "target_id": "dist",
            "target": "dist",
            "target_name": "distribution-switch",
            "protocol": "LLDP",
            "relationship": "uplink",
            "confidence": 0.94,
            "evidence": [
                {"source": "netbox", "collector": "sync", "metadata": {"site": "dc1", "role": "authoritative"}},
                {"source": "snmp", "collector": "poller", "metadata": {"site": "dc1"}},
            ],
        },
        {
            "source_id": "dist",
            "source": "dist",
            "source_name": "distribution-switch",
            "target_id": "edge",
            "target": "edge",
            "target_name": "edge-firewall",
            "protocol": "CDP",
            "relationship": "transit",
            "confidence": 0.72,
            "evidence": [
                {"source": "cli", "collector": "show-cdp", "metadata": {"site": "dc2"}},
            ],
        },
    ],
}


def test_parse_path_relationship_and_evidence_selectors():
    query = parse_query(
        'MATCH PATH FROM core TO dist MAX_HOPS 2 '
        'WHERE relationship.protocol = "LLDP" AND evidence.source IN ("netbox", "servicenow") '
        'AND metadata.site = "dc1" AND confidence >= 0.9 SELECT path,evidence,confidence'
    )

    assert query.source == "core"
    assert query.target == "dist"
    assert query.max_hops == 2
    assert query.selectors == ("path", "evidence", "confidence")
    assert query.confidence == 0.9
    assert query.relationship_filters[0].field == "protocol"
    assert query.evidence_filters[0].value == ["netbox", "servicenow"]
    assert query.metadata_filters[0].field == "site"


def test_filter_graph_applies_path_evidence_and_confidence_constraints():
    query = parse_query(
        'MATCH PATH FROM core TO dist WHERE relationship.protocol = "LLDP" '
        'AND evidence.source = "netbox" AND metadata.role = "authoritative" '
        'AND confidence >= 0.9 SELECT path,evidence'
    )

    result = filter_graph(query, GRAPH)

    assert [node["id"] for node in result["nodes"]] == ["core", "dist"]
    assert len(result["edges"]) == 1
    assert result["edges"][0]["protocol"] == "LLDP"
    assert result["edges"][0]["evidence"] == [
        {"source": "netbox", "collector": "sync", "metadata": {"site": "dc1", "role": "authoritative"}}
    ]


def test_filter_graph_rejects_low_confidence_and_wrong_relationships():
    query = parse_query(
        'MATCH PATH WHERE relationship.protocol != "LLDP" AND confidence >= 0.9 SELECT path'
    )

    result = filter_graph(query, GRAPH)

    assert result["edges"] == []
    assert result["nodes"] == []


def test_saved_query_store_expands_named_fixtures():
    store = SavedQueryStore()
    store.add("dc1_authoritative_lldp", 'MATCH PATH WHERE relationship.protocol = "LLDP" AND metadata.site = "dc1" SELECT path,evidence')

    query = store.parse("SAVED dc1_authoritative_lldp")
    result = filter_graph(query, GRAPH)

    assert len(result["edges"]) == 1
    assert result["edges"][0]["source_id"] == "core"


def test_invalid_selectors_and_filters_are_rejected():
    with pytest.raises(ValueError, match="unknown selector"):
        parse_query("MATCH PATH SELECT packets")

    with pytest.raises(ValueError, match="unsupported filter namespace"):
        parse_query('MATCH PATH WHERE node.name = "core"')

    with pytest.raises(ValueError, match="MAX_HOPS"):
        parse_query("MATCH PATH MAX_HOPS 0")
