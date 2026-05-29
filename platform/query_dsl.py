"""Small graph query DSL used by analyst-facing topology workflows.

The parser intentionally accepts a compact, SQL-like subset which can be
validated deterministically in tests and embedded by CLIs/services later:

    MATCH PATH FROM core TO edge MAX_HOPS 3
    WHERE relationship.protocol = "LLDP" AND evidence.source IN ("netbox")
    SELECT path,evidence

Supported constraints include path endpoints/max hops, relationship filters,
evidence selectors, metadata selectors, and confidence thresholds.
"""

from __future__ import annotations

from dataclasses import dataclass, field
import re
import shlex
from typing import Any, Iterable, Mapping, Sequence


_IDENTIFIER = r"[A-Za-z_][A-Za-z0-9_.-]*"


@dataclass(frozen=True)
class Comparison:
    """A field comparison parsed from a WHERE clause."""

    field: str
    operator: str
    value: Any

    def matches(self, candidate: Mapping[str, Any]) -> bool:
        actual = _lookup(candidate, self.field)
        if self.operator == "=":
            return str(actual).lower() == str(self.value).lower()
        if self.operator == "!=":
            return str(actual).lower() != str(self.value).lower()
        if self.operator == "in":
            return str(actual).lower() in {str(v).lower() for v in self.value}
        if self.operator in {">=", ">", "<=", "<"}:
            try:
                left = float(actual)
                right = float(self.value)
            except (TypeError, ValueError):
                return False
            return {
                ">=": left >= right,
                ">": left > right,
                "<=": left <= right,
                "<": left < right,
            }[self.operator]
        raise ValueError(f"unsupported comparison operator: {self.operator}")


@dataclass(frozen=True)
class GraphQuery:
    """Parsed graph query with path, relationship and evidence constraints."""

    source: str | None = None
    target: str | None = None
    max_hops: int | None = None
    selectors: tuple[str, ...] = ("path",)
    relationship_filters: tuple[Comparison, ...] = ()
    evidence_filters: tuple[Comparison, ...] = ()
    metadata_filters: tuple[Comparison, ...] = ()
    confidence: float | None = None
    saved_name: str | None = None

    def matches_edge(self, edge: Mapping[str, Any]) -> bool:
        if self.confidence is not None and float(edge.get("confidence", 0.0)) < self.confidence:
            return False
        return all(f.matches(edge) for f in self.relationship_filters)

    def matching_evidence(self, evidence: Iterable[Mapping[str, Any]]) -> list[Mapping[str, Any]]:
        matches: list[Mapping[str, Any]] = []
        for item in evidence:
            if all(f.matches(item) for f in self.evidence_filters) and all(
                f.matches(item.get("metadata", {})) for f in self.metadata_filters
            ):
                matches.append(item)
        return matches


@dataclass
class SavedQueryStore:
    """In-memory saved query registry for deterministic fixtures and CLIs."""

    queries: dict[str, str] = field(default_factory=dict)

    def add(self, name: str, query: str) -> None:
        if not re.fullmatch(_IDENTIFIER, name):
            raise ValueError(f"invalid saved query name: {name!r}")
        self.queries[name] = query

    def parse(self, text: str) -> GraphQuery:
        query = parse_query(text)
        if query.saved_name:
            if query.saved_name not in self.queries:
                raise KeyError(f"saved query not found: {query.saved_name}")
            return parse_query(self.queries[query.saved_name])
        return query


def parse_query(text: str) -> GraphQuery:
    """Parse a compact graph query DSL string into a :class:`GraphQuery`."""

    text = " ".join(text.strip().split())
    if not text:
        raise ValueError("query is empty")

    saved = re.fullmatch(rf"SAVED\s+({_IDENTIFIER})", text, flags=re.IGNORECASE)
    if saved:
        return GraphQuery(saved_name=saved.group(1))

    match = re.match(r"MATCH\s+PATH\b", text, flags=re.IGNORECASE)
    if not match:
        raise ValueError("query must start with MATCH PATH or SAVED <name>")

    remainder = text[match.end() :].strip()
    where_clause = ""
    select_clause = "path"

    select_split = re.split(r"\bSELECT\b", remainder, maxsplit=1, flags=re.IGNORECASE)
    if len(select_split) == 2:
        remainder, select_clause = select_split[0].strip(), select_split[1].strip()

    where_split = re.split(r"\bWHERE\b", remainder, maxsplit=1, flags=re.IGNORECASE)
    if len(where_split) == 2:
        path_clause, where_clause = where_split[0].strip(), where_split[1].strip()
    else:
        path_clause = remainder

    source = _optional_keyword_value(path_clause, "FROM")
    target = _optional_keyword_value(path_clause, "TO")
    max_hops_raw = _optional_keyword_value(path_clause, "MAX_HOPS")
    max_hops = int(max_hops_raw) if max_hops_raw is not None else None
    if max_hops is not None and max_hops < 1:
        raise ValueError("MAX_HOPS must be >= 1")

    selectors = tuple(part.strip().lower() for part in select_clause.split(",") if part.strip()) or ("path",)
    allowed_selectors = {"path", "relationships", "evidence", "metadata", "confidence"}
    unknown = sorted(set(selectors) - allowed_selectors)
    if unknown:
        raise ValueError(f"unknown selector(s): {', '.join(unknown)}")

    relationship_filters: list[Comparison] = []
    evidence_filters: list[Comparison] = []
    metadata_filters: list[Comparison] = []
    confidence: float | None = None

    for comparison in _parse_where(where_clause):
        if comparison.field == "confidence":
            confidence = float(comparison.value)
        elif comparison.field.startswith("relationship."):
            relationship_filters.append(_trim_prefix(comparison, "relationship."))
        elif comparison.field.startswith("evidence."):
            evidence_filters.append(_trim_prefix(comparison, "evidence."))
        elif comparison.field.startswith("metadata."):
            metadata_filters.append(_trim_prefix(comparison, "metadata."))
        else:
            raise ValueError(f"unsupported filter namespace: {comparison.field}")

    return GraphQuery(
        source=source,
        target=target,
        max_hops=max_hops,
        selectors=selectors,
        relationship_filters=tuple(relationship_filters),
        evidence_filters=tuple(evidence_filters),
        metadata_filters=tuple(metadata_filters),
        confidence=confidence,
    )


def filter_graph(query: GraphQuery, graph: Mapping[str, Any]) -> dict[str, Any]:
    """Apply a parsed query to a simple graph fixture.

    ``graph`` should contain ``nodes`` and ``edges``. Edges may include an
    ``evidence`` list. The result keeps only edges matching path endpoints,
    confidence, relationship filters and evidence selectors.
    """

    nodes = {node["id"]: node for node in graph.get("nodes", [])}
    selected_edges = []
    for edge in graph.get("edges", []):
        if query.source and query.source not in {edge.get("source"), edge.get("source_name"), edge.get("source_id")}:
            continue
        if query.target and query.target not in {edge.get("target"), edge.get("target_name"), edge.get("target_id")}:
            continue
        if not query.matches_edge(edge):
            continue
        evidence = query.matching_evidence(edge.get("evidence", []))
        if query.evidence_filters and not evidence:
            continue
        selected = dict(edge)
        if "evidence" in query.selectors:
            selected["evidence"] = evidence
        elif "evidence" in selected:
            selected.pop("evidence")
        selected_edges.append(selected)

    node_ids = []
    seen_node_ids = set()
    for edge in selected_edges:
        for node_id in (edge.get("source_id"), edge.get("target_id")):
            if node_id in nodes and node_id not in seen_node_ids:
                node_ids.append(node_id)
                seen_node_ids.add(node_id)
    return {
        "nodes": [nodes[node_id] for node_id in node_ids],
        "edges": selected_edges[: query.max_hops] if query.max_hops else selected_edges,
        "selectors": list(query.selectors),
    }


def _optional_keyword_value(clause: str, keyword: str) -> str | None:
    match = re.search(rf"\b{keyword}\s+(\"[^\"]+\"|'[^']+'|\S+)", clause, flags=re.IGNORECASE)
    if not match:
        return None
    return _clean_scalar(match.group(1))


def _parse_where(where_clause: str) -> list[Comparison]:
    if not where_clause:
        return []
    parts = re.split(r"\s+AND\s+", where_clause, flags=re.IGNORECASE)
    return [_parse_comparison(part.strip()) for part in parts if part.strip()]


def _parse_comparison(part: str) -> Comparison:
    in_match = re.fullmatch(rf"({_IDENTIFIER})\s+IN\s+\((.+)\)", part, flags=re.IGNORECASE)
    if in_match:
        return Comparison(in_match.group(1).lower(), "in", [_clean_scalar(v) for v in _split_csv(in_match.group(2))])
    match = re.fullmatch(rf"({_IDENTIFIER})\s*(>=|<=|!=|=|>|<)\s*(.+)", part)
    if not match:
        raise ValueError(f"invalid WHERE comparison: {part}")
    return Comparison(match.group(1).lower(), match.group(2), _clean_scalar(match.group(3)))


def _split_csv(value: str) -> list[str]:
    lexer = shlex.shlex(value, posix=True)
    lexer.whitespace = ","
    lexer.whitespace_split = True
    return list(lexer)


def _clean_scalar(value: str) -> str:
    return value.strip().strip('"').strip("'")


def _trim_prefix(comparison: Comparison, prefix: str) -> Comparison:
    return Comparison(comparison.field.removeprefix(prefix), comparison.operator, comparison.value)


def _lookup(candidate: Mapping[str, Any], dotted: str) -> Any:
    current: Any = candidate
    for part in dotted.split("."):
        if not isinstance(current, Mapping):
            return None
        current = current.get(part)
    return current
