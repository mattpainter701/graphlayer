"""
Graph theory algorithms for network topology.

Eccentricity, graph radius, and graph diameter based on BFS.
"""
from collections import deque
from typing import Dict, Set, Union

Graph = Dict[str, Set[str]]


def _bfs_distances(graph: Graph, start: str) -> Dict[str, Union[int, float]]:
    """Return shortest path distances from `start` to every node.
    Unreachable nodes receive distance ``float('inf')``.
    """
    dist = {node: float('inf') for node in graph}
    dist[start] = 0
    queue = deque([start])
    while queue:
        current = queue.popleft()
        cur_dist = dist[current]
        for nbr in graph[current]:
            if dist[nbr] == float('inf'):
                dist[nbr] = cur_dist + 1
                queue.append(nbr)
    return dist


def eccentricity(graph: Graph, node_id: str) -> float:
    """Eccentricity of a node: the longest shortest path to any other node.

    Returns:
        float: the eccentricity, or ``inf`` if the graph is disconnected and
        some nodes are unreachable from ``node_id``.

    Raises:
        ValueError: if ``node_id`` is not present in the graph.
    """
    if node_id not in graph:
        raise ValueError(f"Node {node_id!r} is not in the graph")
    distances = _bfs_distances(graph, node_id)
    return max(distances.values())


def graph_radius(graph: Graph) -> float:
    """Graph radius: minimum eccentricity among all nodes.

    Returns:
        float: the radius, or ``inf`` for a disconnected graph.
    """
    if not graph:
        raise ValueError("Graph is empty")
    return min(eccentricity(graph, n) for n in graph)


def graph_diameter(graph: Graph) -> float:
    """Graph diameter: maximum eccentricity among all nodes.

    Returns:
        float: the diameter, or ``inf`` for a disconnected graph.
    """
    if not graph:
        raise ValueError("Graph is empty")
    return max(eccentricity(graph, n) for n in graph)
