"""Python lifecycle facade for the Rust ServiceNow CMDB connector."""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Any, Dict, Optional, Sequence

from ..rust_worker import RustWorkerError, call_rust
from .base_connector import BaseConnector

STATE_FILENAME = "servicenow_cmdb_state.json"
DEFAULT_PAGE_SIZE = 1000
DEFAULT_TIMEOUT_SECONDS = 30.0


class ServiceNowAPIError(RuntimeError):
    """A sanitized error returned by the Rust ServiceNow transport."""


@dataclass(frozen=True)
class ServiceNowQueryResult:
    table: str
    records: list[Dict[str, Any]]
    query: Optional[str] = None
    fields: Optional[Sequence[str]] = None
    page_size: int = DEFAULT_PAGE_SIZE
    updated_since: Optional[str] = None


class ServiceNowTableClient:
    """Compatibility API whose HTTP and pagination operations execute in Rust."""

    def __init__(self, base_url: str, username: str, password: str, *,
                 verify_ssl: bool = True, timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
                 default_page_size: int = DEFAULT_PAGE_SIZE,
                 table_prefix: str = "/api/now/table") -> None:
        self.base_url = base_url
        self.username = username
        self.password = password
        self.verify_ssl = verify_ssl
        self.timeout_seconds = timeout_seconds
        self.default_page_size = default_page_size
        self.table_prefix = table_prefix

    def _request_data(self) -> Dict[str, Any]:
        return {"connector_data": {
            "connector_type": "servicenow_cmdb", "host": self.base_url,
            "username": self.username, "password": self.password,
            "payload": {"verify_ssl": self.verify_ssl,
                        "timeout_seconds": self.timeout_seconds,
                        "page_size": self.default_page_size,
                        "table_prefix": self.table_prefix},
        }}

    def _call(self, operation: str, request: Dict[str, Any]) -> Any:
        try:
            return call_rust(operation, request, timeout=int(self.timeout_seconds) + 5)
        except RustWorkerError as exc:
            raise ServiceNowAPIError(str(exc)) from exc

    def fetch_table_page(self, table: str, *, query: Optional[str] = None,
                         fields: Optional[Sequence[str]] = None,
                         page_size: Optional[int] = None, offset: int = 0) -> Dict[str, Any]:
        request = self._request_data()
        request.update({"table": table, "query": query, "fields": fields,
                        "page_size": page_size or self.default_page_size, "offset": offset})
        return self._call("servicenow.fetch_table_page", request)

    def fetch_table(self, table: str, *, query: Optional[str] = None,
                    fields: Optional[Sequence[str]] = None,
                    page_size: Optional[int] = None) -> ServiceNowQueryResult:
        request = self._request_data()
        request.update({"table": table, "query": query, "fields": fields,
                        "page_size": page_size or self.default_page_size})
        result = self._call("servicenow.fetch_table", request)
        return ServiceNowQueryResult(table=result["table"], records=result["records"],
                                     query=result.get("query"), fields=result.get("fields"),
                                     page_size=result["page_size"],
                                     updated_since=result.get("updated_since"))

    def ping(self) -> bool:
        self._call("servicenow.ping", self._request_data())
        return True


class ServiceNowCMDBConnector(BaseConnector):
    """ServiceNow lifecycle facade backed by Rust normalization and acquisition."""

    SUPPORTS_DEVICES = True

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None,
                 task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)
        payload = dict(connector_data.get("payload", {}) or {})
        credentials = dict(connector_data.get("credentials", {}) or {})
        self.instance_url = (payload.get("instance_url") or connector_data.get("instance_url")
                             or self.host)
        self.table_prefix = payload.get("table_prefix", "/api/now/table")
        self.page_size = int(payload.get("page_size", DEFAULT_PAGE_SIZE) or DEFAULT_PAGE_SIZE)
        self.verify_ssl = bool(payload.get("verify_ssl", True))
        self.updated_since = payload.get("updated_since")
        self.device_tables = list(payload.get("device_tables", ["cmdb_ci_server", "cmdb_ci_netgear"]))
        self.relationship_table = payload.get("relationship_table", "cmdb_rel_ci")
        username = str(connector_data.get("username") or credentials.get("username") or "")
        password = str(connector_data.get("password") or credentials.get("password") or "")
        self._client = ServiceNowTableClient(
            self.instance_url, username, password, verify_ssl=self.verify_ssl,
            timeout_seconds=float(payload.get("timeout_seconds", DEFAULT_TIMEOUT_SECONDS)),
            default_page_size=self.page_size, table_prefix=self.table_prefix,
        )

    def _state_file_path(self) -> str:
        return os.path.join(self.working_folder, STATE_FILENAME)

    def _load_state(self) -> Dict[str, Any]:
        return call_rust("servicenow.load_state", {"state_file_path": self._state_file_path()})

    def _save_state(self, state: Dict[str, Any]) -> None:
        call_rust("servicenow.save_state", {
            "state_file_path": self._state_file_path(), "state": state,
        })

    def _resolve_updated_since(self) -> Optional[str]:
        return call_rust("servicenow.resolve_updated_since", {
            "connector_data": self.connector_data,
            "state_file_path": self._state_file_path(),
        }) or None

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
        request = {"connector_data": self.connector_data, "kwargs": kwargs,
                   "state_file_path": self._state_file_path()}
        try:
            bundle = call_rust("servicenow.collect", request,
                               timeout=int(float(self.payload.get("timeout_seconds", DEFAULT_TIMEOUT_SECONDS))) + 5)
        except RustWorkerError as exc:
            raise ServiceNowAPIError(str(exc)) from exc
        return bundle

    def transform(self, raw_data: Any, **kwargs: Any) -> Any:
        try:
            return call_rust("servicenow.normalize", {
                "connector_data": self.connector_data,
                "raw_data": raw_data,
                "kwargs": kwargs,
            })
        except RustWorkerError as exc:
            raise ServiceNowAPIError(str(exc)) from exc

    def _normalize_bundle(self, raw_bundle: Any) -> Dict[str, Any]:
        return self.transform(raw_bundle)


def test_connection(instance_url: str, username: str, password: str,
                    **kwargs: Any) -> Dict[str, Any]:
    connector = ServiceNowCMDBConnector("servicenow-smoke", {
        "name": "ServiceNow CMDB Smoke Test", "host": instance_url,
        "connector_type": "servicenow_cmdb", "username": username,
        "password": password, "payload": kwargs,
    })
    try:
        connector.connect()
        return {"status": "success", "connected": connector.connected}
    except Exception as exc:  # noqa: BLE001 - public helper reports sanitized errors
        return {"status": "error", "error": str(exc)}


test_connection.__test__ = False
