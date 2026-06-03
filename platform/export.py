"""
Topology graph export utilities.

Supports GraphML, Cytoscape-compatible JSON, and compact analyst bundle formats
with round-trip consistency.
"""
import json
from xml.etree.ElementTree import Element, SubElement, tostring
from xml.dom import minidom
from typing import Any, Dict, List, Optional

# ---------------------------------------------------------------------------
# GraphML export
# ---------------------------------------------------------------------------

def export_to_graphml(graph: Dict[str, Any]) -> str:
    """Export topology graph to GraphML XML format.
    
    Args:
        graph: Dict with "nodes" (list of dicts with id/name) and 
               "edges" (list of dicts with source_id, target_id, etc.)
    Returns:
        Formatted GraphML XML string.
    """
    root = Element("graphml")
    root.set("xmlns", "http://graphml.graphdrawing.org/xmlns")
    
    # Define graph element
    graph_el = SubElement(root, "graph")
    graph_el.set("id", "G")
    graph_el.set("edgedefault", "undirected")
    
    # Key definitions
    key_name = SubElement(root, "key")
    key_name.set("id", "name")
    key_name.set("for", "node")
    key_name.set("attr.name", "name")
    key_name.set("attr.type", "string")
    
    key_role = SubElement(root, "key")
    key_role.set("id", "role")
    key_role.set("for", "node")
    key_role.set("attr.name", "role")
    key_role.set("attr.type", "string")
    
    key_protocol = SubElement(root, "key")
    key_protocol.set("id", "protocol")
    key_protocol.set("for", "edge")
    key_protocol.set("attr.name", "protocol")
    key_protocol.set("attr.type", "string")
    
    key_confidence = SubElement(root, "key")
    key_confidence.set("id", "confidence")
    key_confidence.set("for", "edge")
    key_confidence.set("attr.name", "confidence")
    key_confidence.set("attr.type", "double")
    
    key_relationship = SubElement(root, "key")
    key_relationship.set("id", "relationship")
    key_relationship.set("for", "edge")
    key_relationship.set("attr.name", "relationship")
    key_relationship.set("attr.type", "string")
    
    # Nodes
    for node in graph.get("nodes", []):
        node_el = SubElement(graph_el, "node")
        node_el.set("id", node.get("id", ""))
        if "name" in node:
            data = SubElement(node_el, "data")
            data.set("key", "name")
            data.text = node["name"]
        if "role" in node:
            data = SubElement(node_el, "data")
            data.set("key", "role")
            data.text = node["role"]
    
    # Edges
    for i, edge in enumerate(graph.get("edges", [])):
        edge_el = SubElement(graph_el, "edge")
        edge_el.set("id", f"e{i}")
        edge_el.set("source", edge.get("source_id", ""))
        edge_el.set("target", edge.get("target_id", ""))
        if "protocol" in edge:
            data = SubElement(edge_el, "data")
            data.set("key", "protocol")
            data.text = edge["protocol"]
        if "confidence" in edge:
            data = SubElement(edge_el, "data")
            data.set("key", "confidence")
            data.text = str(edge["confidence"])
        if "relationship" in edge:
            data = SubElement(edge_el, "data")
            data.set("key", "relationship")
            data.text = edge["relationship"]
    
    raw = tostring(root, encoding="unicode")
    return minidom.parseString(raw).toprettyxml(indent="  ")


def parse_graphml(xml_str: str) -> Dict[str, Any]:
    """Round-trip: parse a GraphML string back into a graph dict.
    
    Args:
        xml_str: GraphML XML string.
    Returns:
        Graph dict with "nodes" and "edges" lists.
    """
    import xml.etree.ElementTree as ET
    
    root = ET.fromstring(xml_str)
    # Handle namespace
    ns = "http://graphml.graphdrawing.org/xmlns"
    
    graph_el = root.find(f"{{{ns}}}graph") or root.find("graph")
    if graph_el is None:
        raise ValueError("No <graph> element found in GraphML")
    
    # Collect key definitions
    key_map = {}
    for key_el in root.findall(f"{{{ns}}}key") or root.findall("key"):
        kid = key_el.get("id", "")
        kfor = key_el.get("for", "")
        kname = key_el.get("attr.name", kid)
        key_map[kid] = {"for": kfor, "name": kname}
    
    nodes = []
    edges = []
    
    # Process nodes and edges (children of graph)
    for child in graph_el:
        tag = child.tag
        if "}" in tag:
            tag = tag.split("}", 1)[1]
        
        if tag == "node":
            node = {"id": child.get("id", "")}
            for data in child:
                dt = data.tag
                if "}" in dt:
                    dt = dt.split("}", 1)[1]
                if dt == "data":
                    k = data.get("key", "")
                    if k in key_map:
                        node[key_map[k]["name"]] = data.text or ""
                    else:
                        node[k] = data.text or ""
            nodes.append(node)
        
        elif tag == "edge":
            edge = {
                "source_id": child.get("source", ""),
                "target_id": child.get("target", ""),
            }
            edge["source"] = edge["source_id"]
            edge["target"] = edge["target_id"]
            for data in child:
                dt = data.tag
                if "}" in dt:
                    dt = dt.split("}", 1)[1]
                if dt == "data":
                    k = data.get("key", "")
                    val = data.text or ""
                    if k in key_map:
                        kname = key_map[k]["name"]
                        if kname == "confidence":
                            val = float(val)
                        edge[kname] = val
                    else:
                        edge[k] = val
            edges.append(edge)
    
    return {"nodes": nodes, "edges": edges}


# ---------------------------------------------------------------------------
# Cytoscape-compatible JSON export
# ---------------------------------------------------------------------------

def export_to_cytoscape(graph: Dict[str, Any]) -> str:
    """Export topology graph to Cytoscape.js-compatible JSON format.
    
    Produces the standard elements array format:
      elements: [
        {data: {id: ..., name: ...}},                    # node
        {data: {id: ..., source: ..., target: ..., ...}}  # edge
      ]
    
    Args:
        graph: Dict with "nodes" and "edges" lists.
    Returns:
        Pretty-printed JSON string.
    """
    elements = []
    
    for node in graph.get("nodes", []):
        node_data = {"id": node.get("id", "")}
        if "name" in node:
            node_data["name"] = node["name"]
        if "role" in node:
            node_data["role"] = node["role"]
        elements.append({"data": node_data})
    
    for i, edge in enumerate(graph.get("edges", [])):
        edge_data = {
            "id": edge.get("id", f"e{i}"),
            "source": edge.get("source_id", edge.get("source", "")),
            "target": edge.get("target_id", edge.get("target", "")),
        }
        if "protocol" in edge:
            edge_data["protocol"] = edge["protocol"]
        if "confidence" in edge:
            edge_data["confidence"] = edge["confidence"]
        if "relationship" in edge:
            edge_data["relationship"] = edge["relationship"]
        elements.append({"data": edge_data})
    
    return json.dumps({"elements": elements}, indent=2)


def parse_cytoscape(json_str: str) -> Dict[str, Any]:
    """Round-trip: parse a Cytoscape JSON string back into a graph dict.
    
    Args:
        json_str: Cytoscape JSON string.
    Returns:
        Graph dict with "nodes" and "edges" lists.
    """
    data = json.loads(json_str)
    elements = data.get("elements", [])
    
    nodes = []
    edges = []
    
    for el in elements:
        el_data = el.get("data", {})
        if "source" in el_data or "target" in el_data:
            # It's an edge
            edge = {
                "source_id": el_data.get("source", ""),
                "target_id": el_data.get("target", ""),
            }
            edge["source"] = edge["source_id"]
            edge["target"] = edge["target_id"]
            for k in ("protocol", "confidence", "relationship", "id"):
                if k in el_data:
                    edge[k] = el_data[k]
            edges.append(edge)
        else:
            # It's a node
            node = {"id": el_data.get("id", "")}
            for k in ("name", "role"):
                if k in el_data:
                    node[k] = el_data[k]
            nodes.append(node)
    
    return {"nodes": nodes, "edges": edges}


# ---------------------------------------------------------------------------
# Compact analyst bundle export
# ---------------------------------------------------------------------------

def _compact_node(node: Dict[str, Any]) -> Dict[str, Any]:
    """Compact a single node for analyst bundle."""
    out = {"i": node.get("id", "")}
    if "name" in node:
        out["n"] = node["name"]
    if "role" in node:
        out["r"] = node["role"]
    return out


def _compact_edge(edge: Dict[str, Any]) -> Dict[str, Any]:
    """Compact a single edge for analyst bundle."""
    out = {
        "s": edge.get("source_id", edge.get("source", "")),
        "t": edge.get("target_id", edge.get("target", "")),
    }
    if "protocol" in edge:
        out["p"] = edge["protocol"]
    if "confidence" in edge:
        out["c"] = edge["confidence"]
    if "relationship" in edge:
        out["r"] = edge["relationship"]
    return out


def export_to_analyst_bundle(graph: Dict[str, Any], pretty: bool = False) -> str:
    """Export topology graph to compact analyst bundle JSON format.
    
    Uses short key aliases (i=id, n=name, s=source, t=target, p=protocol, etc.)
    for bandwidth/size efficiency.
    
    Args:
        graph: Dict with "nodes" and "edges" lists.
        pretty: If True, pretty-print the JSON (default: compact).
    Returns:
        JSON string.
    """
    bundle = {
        "v": 1,  # version
        "t": "topology",  # type
        "n": [ _compact_node(n) for n in graph.get("nodes", []) ],
        "e": [ _compact_edge(e) for e in graph.get("edges", []) ],
    }
    indent = 2 if pretty else None
    return json.dumps(bundle, indent=indent, separators=(",", ":") if not pretty else None)


def parse_analyst_bundle(json_str: str) -> Dict[str, Any]:
    """Round-trip: parse a compact analyst bundle back into a graph dict.
    
    Args:
        json_str: Analyst bundle JSON string.
    Returns:
        Graph dict with "nodes" and "edges" lists.
    """
    data = json.loads(json_str)
    
    nodes = []
    for n in data.get("n", []):
        node = {"id": n.get("i", "")}
        if "n" in n:
            node["name"] = n["n"]
        if "r" in n:
            node["role"] = n["r"]
        nodes.append(node)
    
    edges = []
    for e in data.get("e", []):
        edge = {
            "source_id": e.get("s", ""),
            "target_id": e.get("t", ""),
        }
        edge["source"] = edge["source_id"]
        edge["target"] = edge["target_id"]
        if "p" in e:
            edge["protocol"] = e["p"]
        if "c" in e:
            edge["confidence"] = e["c"]
        if "r" in e:
            edge["relationship"] = e["r"]
        edges.append(edge)
    
    return {"nodes": nodes, "edges": edges}


# ---------------------------------------------------------------------------
# Unified dispatcher
# ---------------------------------------------------------------------------

FORMATS = {"graphml", "cytoscape", "analyst-bundle"}


def export_graph(graph: Dict[str, Any], fmt: str, **kwargs) -> str:
    """Export a topology graph to the specified format.
    
    Args:
        graph: Dict with "nodes" and "edges" lists.
        fmt: One of "graphml", "cytoscape", "analyst-bundle".
        **kwargs: Passed through to the format-specific exporter.
    Returns:
        Formatted string.
    
    Raises:
        ValueError: If format is unsupported.
    """
    if fmt == "graphml":
        return export_to_graphml(graph)
    elif fmt == "cytoscape":
        return export_to_cytoscape(graph)
    elif fmt == "analyst-bundle":
        return export_to_analyst_bundle(graph, **kwargs)
    else:
        raise ValueError(f"Unsupported export format: {fmt!r}. Supported: {sorted(FORMATS)}")


def parse_graph(data: str, fmt: str) -> Dict[str, Any]:
    """Parse exported data back into a graph dict (round-trip).
    
    Args:
        data: String in one of the supported formats.
        fmt: One of "graphml", "cytoscape", "analyst-bundle".
    Returns:
        Graph dict with "nodes" and "edges" lists.
    """
    if fmt == "graphml":
        return parse_graphml(data)
    elif fmt == "cytoscape":
        return parse_cytoscape(data)
    elif fmt == "analyst-bundle":
        return parse_analyst_bundle(data)
    else:
        raise ValueError(f"Unsupported format: {fmt!r}")
