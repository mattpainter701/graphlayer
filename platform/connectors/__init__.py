"""Connector implementations, shared contracts, and runtime registry."""

from .base_connector import BaseConnector, ConnectorContract, NormalizedRecord
from .fileshare_connector import FileShareConnector
from .netbox_connector import NetBoxConnector
from .registry import (
    ConnectorRegistry,
    connector,
    create_connector,
    discover_connectors,
    get_connector,
    list_connectors,
    register_connector,
)
from .servicenow_cmdb_connector import ServiceNowCMDBConnector
from .solarwinds_ncm_connector import SolarWindsNCMConnector

__all__ = [
    "BaseConnector",
    "ConnectorContract",
    "ConnectorRegistry",
    "FileShareConnector",
    "NetBoxConnector",
    "NormalizedRecord",
    "ServiceNowCMDBConnector",
    "SolarWindsNCMConnector",
    "connector",
    "create_connector",
    "discover_connectors",
    "get_connector",
    "list_connectors",
    "register_connector",
]
