"""Python compatibility facade for the Rust NetBox connector."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict, Optional, Sequence

from ..rust_worker import RustWorkerError, call_rust
from .base_connector import BaseConnector

DEFAULT_TIMEOUT_SECONDS = 30.0
DEFAULT_PAGE_SIZE = 1000


class NetBoxAPIError(RuntimeError):
    """A sanitized error returned by the Rust NetBox transport."""


@dataclass(frozen=True)
class NetBoxQueryResult:
    endpoint: str
    records: list[Dict[str, Any]]
    query: Optional[str] = None
    fields: Optional[Sequence[str]] = None
    page_size: int = DEFAULT_PAGE_SIZE


class NetBoxAPIClient:
    """Compatibility API whose requests and pagination execute in Rust."""

    def __init__(self, base_url: str, token: str, *, verify_ssl: bool = True,
                 timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
                 default_page_size: int = DEFAULT_PAGE_SIZE) -> None:
        self.base_url = base_url
        self.token = token
        self.verify_ssl = verify_ssl
        self.timeout_seconds = timeout_seconds
        self.default_page_size = default_page_size

    def _request_data(self) -> Dict[str, Any]:
        return {
            "connector_data": {
                "connector_type": "netbox",
                "host": self.base_url,
                "credentials": {"api_token": self.token},
                "payload": {
                    "verify_ssl": self.verify_ssl,
                    "timeout_seconds": self.timeout_seconds,
                    "page_size": self.default_page_size,
                },
            }
        }

    def _call(self, operation: str, request: Dict[str, Any]) -> Any:
        try:
            return call_rust(operation, request, timeout=int(self.timeout_seconds) + 5)
        except RustWorkerError as exc:
            raise NetBoxAPIError(str(exc)) from exc

    def fetch_endpoint_page(self, endpoint: str, *, query: Optional[str] = None,
                            fields: Optional[Sequence[str]] = None,
                            page_size: Optional[int] = None, offset: int = 0) -> Dict[str, Any]:
        request = self._request_data()
        request.update({"endpoint": endpoint, "query": query, "fields": fields,
                        "page_size": page_size or self.default_page_size, "offset": offset})
        return self._call("netbox.fetch_endpoint_page", request)

    def fetch_endpoint(self, endpoint: str, *, query: Optional[str] = None,
                       fields: Optional[Sequence[str]] = None,
                       page_size: Optional[int] = None) -> NetBoxQueryResult:
        request = self._request_data()
        request.update({"endpoint": endpoint, "query": query, "fields": fields,
                        "page_size": page_size or self.default_page_size})
        raw = self._call("netbox.fetch_endpoint", request)
        return NetBoxQueryResult(endpoint=raw["endpoint"], records=raw["records"],
                                 query=raw.get("query"), fields=raw.get("fields"),
                                 page_size=raw["page_size"])

    def ping(self) -> bool:
        self._call("netbox.ping", self._request_data())
        return True


class NetBoxConnector(BaseConnector):
    """NetBox lifecycle and compatibility methods backed by Rust operations."""

    SUPPORTS_DEVICES = True

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None,
                 task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)
        payload = dict(connector_data.get("payload", {}) or {})
        credentials = dict(connector_data.get("credentials", {}) or {})
        self.instance_url = (payload.get("instance_url") or connector_data.get("instance_url")
                             or self.host)
        self.api_token = str(payload.get("api_token") or credentials.get("api_token")
                             or credentials.get("token") or connector_data.get("api_token") or "")
        self.verify_ssl = bool(payload.get("verify_ssl", True))
        self.page_size = int(payload.get("page_size", DEFAULT_PAGE_SIZE) or DEFAULT_PAGE_SIZE)
        self.timeout_seconds = float(payload.get("timeout_seconds", DEFAULT_TIMEOUT_SECONDS))
        self.device_endpoint = payload.get("device_endpoint", "dcim/devices/")
        self.interface_endpoint = payload.get("interface_endpoint", "dcim/interfaces/")
        self.cable_endpoint = payload.get("cable_endpoint", "dcim/cables/")
        self.prefix_endpoint = payload.get("prefix_endpoint", "ipam/prefixes/")
        self.ip_endpoint = payload.get("ip_endpoint", "ipam/ip-addresses/")
        self.site_filter = payload.get("site")
        self.role_filter = payload.get("role")
        self._client = NetBoxAPIClient(
            self.instance_url, self.api_token, verify_ssl=self.verify_ssl,
            timeout_seconds=self.timeout_seconds, default_page_size=self.page_size,
        )

    def connect(self):
        self._client.ping()
        self.mark_connected()
        return True

    def disconnect(self):
        self.mark_disconnected()
        return True

    def do_download(self, **kwargs):
        if not self.connected:
            self.connect()
        request = {"connector_data": self.connector_data, "kwargs": kwargs}
        try:
            return call_rust("netbox.collect", request, timeout=int(self.timeout_seconds) + 5)
        except RustWorkerError as exc:
            raise NetBoxAPIError(str(exc)) from exc

    def transform(self, raw_data: Any, **kwargs: Any) -> Any:
        try:
            return call_rust("netbox.normalize", {
                "connector_data": self.connector_data,
                "raw_data": raw_data,
                "kwargs": kwargs,
            })
        except RustWorkerError as exc:
            raise NetBoxAPIError(str(exc)) from exc

    def _normalize_bundle(self, raw_bundle: Any) -> Dict[str, Any]:
        return self.transform(raw_bundle)


def test_connection(instance_url: str, api_token: str, **kwargs: Any) -> Dict[str, Any]:
    connector = NetBoxConnector("netbox-smoke", {
        "name": "NetBox Smoke Test", "host": instance_url, "connector_type": "netbox",
        "credentials": {"api_token": api_token}, "payload": kwargs,
    })
    try:
        connector.connect()
        return {"status": "success", "connected": connector.connected}
    except Exception as exc:  # noqa: BLE001 - public compatibility helper reports sanitized failures
        return {"status": "error", "error": str(exc)}


test_connection.__test__ = False
