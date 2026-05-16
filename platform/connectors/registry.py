"""Runtime connector registration and factory helpers.

The registry keeps connector discovery intentionally lightweight: built-in
connectors register at import time, external connectors can register classes
programmatically, and optional module paths can be imported on demand before a
connector is created.
"""

from __future__ import annotations

import importlib
from typing import Any, Dict, Iterable, Mapping, Type

from .base_connector import BaseConnector


class ConnectorRegistry:
    """In-memory registry for connector implementations."""

    def __init__(self) -> None:
        self._connectors: Dict[str, Type[BaseConnector]] = {}

    def register(self, connector_type: str, connector_class: Type[BaseConnector]) -> Type[BaseConnector]:
        normalized_type = self._normalize_type(connector_type)
        if not normalized_type:
            raise ValueError("connector_type is required")
        if not issubclass(connector_class, BaseConnector):
            raise TypeError("connector_class must inherit from BaseConnector")
        self._connectors[normalized_type] = connector_class
        return connector_class

    def unregister(self, connector_type: str) -> None:
        self._connectors.pop(self._normalize_type(connector_type), None)

    def get(self, connector_type: str) -> Type[BaseConnector]:
        normalized_type = self._normalize_type(connector_type)
        try:
            return self._connectors[normalized_type]
        except KeyError as exc:
            available = ", ".join(self.list_types()) or "none"
            raise KeyError(f"unknown connector_type '{connector_type}' (available: {available})") from exc

    def list_types(self) -> list[str]:
        return sorted(self._connectors)

    def create(
        self,
        connector_type: str,
        guid: str,
        connector_data: Mapping[str, Any] | None = None,
        **kwargs: Any,
    ) -> BaseConnector:
        data = dict(connector_data or {})
        data.setdefault("connector_type", self._normalize_type(connector_type))
        return self.get(connector_type)(guid, data, **kwargs)

    @staticmethod
    def _normalize_type(connector_type: str) -> str:
        return str(connector_type or "").strip().lower().replace("-", "_")


_default_registry = ConnectorRegistry()


def register_connector(connector_type: str, connector_class: Type[BaseConnector]) -> Type[BaseConnector]:
    """Register a connector class in the process-wide registry."""

    return _default_registry.register(connector_type, connector_class)


def connector(connector_type: str):
    """Decorator for connector classes.

    Example:
        @connector("custom_rest")
        class CustomRestConnector(BaseConnector):
            ...
    """

    def decorator(connector_class: Type[BaseConnector]) -> Type[BaseConnector]:
        return register_connector(connector_type, connector_class)

    return decorator


def discover_connectors(module_paths: Iterable[str]) -> list[str]:
    """Import modules that may register connector classes.

    Returns the imported module paths so callers can log what was discovered.
    """

    imported: list[str] = []
    for module_path in module_paths:
        module_path = str(module_path or "").strip()
        if not module_path:
            continue
        importlib.import_module(module_path)
        imported.append(module_path)
    return imported


def get_connector(connector_type: str) -> Type[BaseConnector]:
    return _default_registry.get(connector_type)


def create_connector(
    connector_type: str,
    guid: str,
    connector_data: Mapping[str, Any] | None = None,
    **kwargs: Any,
) -> BaseConnector:
    return _default_registry.create(connector_type, guid, connector_data, **kwargs)


def list_connectors() -> list[str]:
    return _default_registry.list_types()


def register_builtin_connectors() -> list[str]:
    """Register built-in connector classes and return available connector types."""

    from .fileshare_connector import FileShareConnector
    from .netbox_connector import NetBoxConnector
    from .servicenow_cmdb_connector import ServiceNowCMDBConnector
    from .solarwinds_ncm_connector import SolarWindsNCMConnector

    register_connector("fileshare", FileShareConnector)
    register_connector("netbox", NetBoxConnector)
    register_connector("servicenow_cmdb", ServiceNowCMDBConnector)
    register_connector("solarwinds_ncm", SolarWindsNCMConnector)
    return list_connectors()


register_builtin_connectors()
