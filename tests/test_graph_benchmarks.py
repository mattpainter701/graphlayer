"""
Benchmark tests for graph algorithm performance regression detection.

These benchmarks exercise the core graph algorithms (BFS, eccentricity,
radius, diameter) at various scales and graph shapes to detect
performance regressions.
"""
import pytest
from platform.graph import eccentricity, graph_radius, graph_diameter

pytestmark = pytest.mark.benchmark


def _make_graph(adj: dict) -> dict:
    """Convert adjacency lists (with lists as values) to sets."""
    return {k: set(v) for k, v in adj.items()}


# ---------------------------------------------------------------------------
# Graph generators at various scales
# ---------------------------------------------------------------------------

def chain_graph(n: int) -> dict:
    """Linear chain of n nodes: 0-1-2-...-(n-1)"""
    g = {}
    for i in range(n):
        neighbors = []
        if i > 0:
            neighbors.append(str(i - 1))
        if i < n - 1:
            neighbors.append(str(i + 1))
        g[str(i)] = set(neighbors)
    return g


def star_graph(n: int) -> dict:
    """Star graph: node '0' connected to n-1 leaves"""
    g = {'0': set(str(i) for i in range(1, n))}
    for i in range(1, n):
        g[str(i)] = {'0'}
    return g


def complete_graph(n: int) -> dict:
    """Complete graph (clique) of n nodes"""
    g = {}
    for i in range(n):
        g[str(i)] = set(str(j) for j in range(n) if j != i)
    return g


def grid_graph(rows: int, cols: int) -> dict:
    """2D grid graph: nodes labeled 'r,c'"""
    g = {}
    for r in range(rows):
        for c in range(cols):
            neighbors = []
            if r > 0:
                neighbors.append(f'{r-1},{c}')
            if r < rows - 1:
                neighbors.append(f'{r+1},{c}')
            if c > 0:
                neighbors.append(f'{r},{c-1}')
            if c < cols - 1:
                neighbors.append(f'{r},{c+1}')
            g[f'{r},{c}'] = set(neighbors)
    return g


# ---------------------------------------------------------------------------
# Benchmarks: BFS distances internal (via eccentricity)
# ---------------------------------------------------------------------------

class TestBFSBenchmarks:
    """Benchmarks for BFS-based distance computation."""

    @pytest.mark.benchmark(min_rounds=50)
    def test_eccentricity_small_chain(self, benchmark):
        g = chain_graph(10)
        benchmark(eccentricity, g, '0')

    @pytest.mark.benchmark(min_rounds=30)
    def test_eccentricity_medium_chain(self, benchmark):
        g = chain_graph(100)
        benchmark(eccentricity, g, '0')

    @pytest.mark.benchmark(min_rounds=10)
    def test_eccentricity_large_chain(self, benchmark):
        g = chain_graph(1000)
        benchmark(eccentricity, g, '0')

    @pytest.mark.benchmark(min_rounds=50)
    def test_eccentricity_star_graph(self, benchmark):
        g = star_graph(100)
        benchmark(eccentricity, g, '0')

    @pytest.mark.benchmark(min_rounds=20)
    def test_eccentricity_complete_small(self, benchmark):
        g = complete_graph(50)
        benchmark(eccentricity, g, '0')

    @pytest.mark.benchmark(min_rounds=10)
    def test_eccentricity_grid_medium(self, benchmark):
        g = grid_graph(10, 10)
        benchmark(eccentricity, g, '0,0')

    @pytest.mark.benchmark(min_rounds=5)
    def test_eccentricity_grid_large(self, benchmark):
        g = grid_graph(32, 32)
        benchmark(eccentricity, g, '0,0')


class TestRadiusDiameterBenchmarks:
    """Benchmarks for graph_radius and graph_diameter computation."""

    @pytest.mark.benchmark(min_rounds=20)
    def test_radius_chain_medium(self, benchmark):
        g = chain_graph(100)
        benchmark(graph_radius, g)

    @pytest.mark.benchmark(min_rounds=10)
    def test_radius_chain_large(self, benchmark):
        g = chain_graph(500)
        benchmark(graph_radius, g)

    @pytest.mark.benchmark(min_rounds=20)
    def test_radius_star(self, benchmark):
        g = star_graph(100)
        benchmark(graph_radius, g)

    @pytest.mark.benchmark(min_rounds=10)
    def test_diameter_chain_medium(self, benchmark):
        g = chain_graph(100)
        benchmark(graph_diameter, g)

    @pytest.mark.benchmark(min_rounds=10)
    def test_diameter_chain_large(self, benchmark):
        g = chain_graph(500)
        benchmark(graph_diameter, g)

    @pytest.mark.benchmark(min_rounds=20)
    def test_diameter_star(self, benchmark):
        g = star_graph(100)
        benchmark(graph_diameter, g)

    @pytest.mark.benchmark(min_rounds=5)
    def test_radius_and_diameter_grid(self, benchmark):
        g = grid_graph(32, 32)
        # Measuring both radius and diameter in sequence
        def run_both():
            _ = graph_radius(g)
            _ = graph_diameter(g)
        benchmark(run_both)


class TestEdgeCasesBenchmarks:
    """Benchmarks for edge case graph sizes."""

    def test_eccentricity_single_node(self, benchmark):
        g = {'A': set()}
        benchmark(eccentricity, g, 'A')

    def test_eccentricity_two_nodes(self, benchmark):
        g = {'A': {'B'}, 'B': {'A'}}
        benchmark(eccentricity, g, 'A')

    @pytest.mark.benchmark(min_rounds=100)
    def test_eccentricity_tiny_star(self, benchmark):
        g = star_graph(5)
        benchmark(eccentricity, g, '0')
