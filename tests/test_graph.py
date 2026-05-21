import math
import pytest
from platform.graph import eccentricity, graph_radius, graph_diameter


def _make_graph(adj: dict) -> dict:
    """Convert adjacency lists (with lists as values) to sets."""
    return {k: set(v) for k, v in adj.items()}


class TestEccentricity:
    def test_connected_triangle(self):
        g = _make_graph({'A': ['B', 'C'], 'B': ['A', 'C'], 'C': ['A', 'B']})
        assert eccentricity(g, 'A') == 1
        assert eccentricity(g, 'B') == 1
        assert eccentricity(g, 'C') == 1

    def test_chain(self):
        g = _make_graph({'A': ['B'], 'B': ['A', 'C'], 'C': ['B', 'D'], 'D': ['C']})
        assert eccentricity(g, 'A') == 3
        assert eccentricity(g, 'B') == 2
        assert eccentricity(g, 'C') == 2
        assert eccentricity(g, 'D') == 3

    def test_disconnected_two_components(self):
        g = _make_graph({'A': ['B'], 'B': ['A'], 'C': ['D'], 'D': ['C']})
        for node in g:
            assert math.isinf(eccentricity(g, node))

    def test_single_node(self):
        g = _make_graph({'A': []})
        assert eccentricity(g, 'A') == 0

    def test_unbalanced_star(self):
        g = _make_graph({'center': ['a', 'b', 'c'], 'a': ['center'],
                         'b': ['center'], 'c': ['center']})
        assert eccentricity(g, 'center') == 1
        for leaf in ['a', 'b', 'c']:
            assert eccentricity(g, leaf) == 2

    def test_missing_node_raises(self):
        g = _make_graph({'A': ['B'], 'B': ['A']})
        with pytest.raises(ValueError):
            eccentricity(g, 'C')


class TestGraphRadiusDiameter:
    def test_connected_kite(self):
        # Kite graph: center node connected to three leaves
        g = _make_graph({'center': ['a', 'b', 'c'], 'a': ['center'],
                         'b': ['center'], 'c': ['center']})
        # eccentricities: center=1, leaves=2 → min=1, max=2
        assert graph_radius(g) == 1
        assert graph_diameter(g) == 2

    def test_chain(self):
        g = _make_graph({'A': ['B'], 'B': ['A', 'C'], 'C': ['B', 'D'], 'D': ['C']})
        assert graph_radius(g) == 2
        assert graph_diameter(g) == 3

    def test_disconnected(self):
        g = _make_graph({'A': ['B'], 'B': ['A'], 'C': ['D'], 'D': ['C']})
        assert math.isinf(graph_radius(g))
        assert math.isinf(graph_diameter(g))

    def test_single_node(self):
        g = _make_graph({'A': []})
        assert graph_radius(g) == 0
        assert graph_diameter(g) == 0

    def test_empty_graph_raises(self):
        with pytest.raises(ValueError, match="Graph is empty"):
            graph_radius({})
        with pytest.raises(ValueError, match="Graph is empty"):
            graph_diameter({})
