"""ServiceNow CMDB connector for GraphLayer.

This connector implements a small, shippable subset of the Sprint 3 scope:

- ServiceNow Table API client with pagination
- CMDB CI normalization into GraphLayer-friendly records
- cmdb_rel_ci relationship extraction into topology links
- incremental sync watermark persistence
- test-friendly HTTP client with no third-party dependencies
"""

from __future__ import annotations

import base64
import json
import os
import ssl
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, Iterable, List, Mapping, Optional, Sequence, Tuple
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urljoin
from urllib.request import Request, urlopen

from .base_connector import BaseConnector, NormalizedRecord


STATE_FILENAME = "servicenow_cmdb_state.json"
DEFAULT_PAGE_SIZE = 1000
DEFAULT_TIMEOUT_SECONDS = 30.0


class ServiceNowAPIError(RuntimeError):
    """Raised when the ServiceNow Table API returns an error response."""


@dataclass(frozen=True)
class ServiceNowQueryResult:
    """Raw records returned from ServiceNow plus the query metadata used."""

    table: str
    records: List[Dict[str, Any]]
    query: Optional[str] = None
    fields: Optional[Sequence[str]] = None
    page_size: int = DEFAULT_PAGE_SIZE
    updated_since: Optional[str] = None


class ServiceNowTableClient:
    """Small Table API client with pagination support.

    The implementation intentionally stays dependency-free so the connector can
    be used in tests with only the Python standard library.
    """

    def __init__(
        self,
        base_url: str,
        username: str,
        password: str,
        *,
        verify_ssl: bool = True,
        timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
        default_page_size: int = DEFAULT_PAGE_SIZE,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.username = username
        self.password = password
        self.verify_ssl = verify_ssl
        self.timeout_seconds = timeout_seconds
        self.default_page_size = default_page_size

    def _ssl_context(self) -> Optional[ssl.SSLContext]:
        if not self.base_url.lower().startswith("https://"):
            return None
        if self.verify_ssl:
            return ssl.create_default_context()
        return ssl._create_unverified_context()  # noqa: SLF001 - intentional for test/dev support

    def _request_json(self, path: str, params: Optional[Mapping[str, Any]] = None) -> Dict[str, Any]:
        query = urlencode([(key, value) for key, value in (params or {}).items() if value is not None])
        url = urljoin(self.base_url + "/", path.lstrip("/"))
        if query:
            url = f"{url}?{query}"

        request = Request(url, method="GET")
        if self.username or self.password:
            credentials = f"{self.username}:{self.password}".encode("utf-8")
            request.add_header("Authorization", f"Basic {base64.b64encode(credentials).decode('ascii')}")
        request.add_header("Accept", "application/json")

        try:
            with urlopen(request, timeout=self.timeout_seconds, context=self._ssl_context()) as response:
                raw_body = response.read().decode("utf-8")
                if not raw_body.strip():
                    return {}
                return json.loads(raw_body)
        except HTTPError as exc:  # pragma: no cover - exercised through URLError/HTTPError handling in tests
            body = exc.read().decode("utf-8", errors="replace") if exc.fp else ""
            raise ServiceNowAPIError(f"ServiceNow API returned HTTP {exc.code} for {url}: {body}") from exc
        except URLError as exc:  # pragma: no cover - network failures are environment specific
            raise ServiceNowAPIError(f"ServiceNow API request failed for {url}: {exc.reason}") from exc
        except json.JSONDecodeError as exc:
            raise ServiceNowAPIError(f"ServiceNow API returned invalid JSON for {url}") from exc

    def fetch_table_page(
        self,
        table: str,
        *,
        query: Optional[str] = None,
        fields: Optional[Sequence[str]] = None,
        page_size: Optional[int] = None,
        offset: int = 0,
    ) -> Dict[str, Any]:
        params: Dict[str, Any] = {
            "sysparm_limit": page_size or self.default_page_size,
            "sysparm_offset": offset,
            "sysparm_display_value": "all",
            "sysparm_exclude_reference_link": "true",
        }
        if query:
            params["sysparm_query"] = query
        if fields:
            params["sysparm_fields"] = ",".join(fields)
        return self._request_json(f"/api/now/table/{table}", params)

    def fetch_table(
        self,
        table: str,
        *,
        query: Optional[str] = None,
        fields: Optional[Sequence[str]] = None,
        page_size: Optional[int] = None,
    ) -> ServiceNowQueryResult:
        limit = page_size or self.default_page_size
        offset = 0
        records: List[Dict[str, Any]] = []

        while True:
            page = self.fetch_table_page(table, query=query, fields=fields, page_size=limit, offset=offset)
            page_records = list(page.get("result", []) or [])
            if not isinstance(page_records, list):
                raise ServiceNowAPIError(f"ServiceNow API returned malformed records for {table}")
            records.extend(page_records)
            if len(page_records) < limit:
                break
            offset += limit

        return ServiceNowQueryResult(
            table=table,
            records=records,
            query=query,
            fields=fields,
            page_size=limit,
        )

    def ping(self) -> bool:
        self.fetch_table_page(
            "cmdb_ci",
            fields=("sys_id",),
            page_size=1,
        )
        return True


class ServiceNowCMDBConnector(BaseConnector):
    """ServiceNow CMDB connector with record normalization."""

    SUPPORTS_DEVICES = True

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None, task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)

        payload = dict(connector_data.get("payload", {}) or {})
        self.instance_url = self._normalize_instance_url(
            payload.get("instance_url")
            or connector_data.get("instance_url")
            or self.host
        )
        self.table_prefix = payload.get("table_prefix", "/api/now/table")
        self.page_size = int(payload.get("page_size", DEFAULT_PAGE_SIZE) or DEFAULT_PAGE_SIZE)
        self.verify_ssl = bool(payload.get("verify_ssl", True))
        self.updated_since = payload.get("updated_since")
        self.device_tables = list(payload.get("device_tables", ["cmdb_ci_server", "cmdb_ci_netgear"]))
        self.relationship_table = payload.get("relationship_table", "cmdb_rel_ci")
        self._client = ServiceNowTableClient(
            self.instance_url,
            self.username,
            self.password,
            verify_ssl=self.verify_ssl,
            timeout_seconds=float(self.payload.get("timeout_seconds", DEFAULT_TIMEOUT_SECONDS)),
            default_page_size=self.page_size,
        )

    def _normalize_instance_url(self, url: str) -> str:
        cleaned = str(url or "").strip().rstrip("/")
        if not cleaned:
            raise ValueError("instance_url or host is required for ServiceNow CMDB")
        if not cleaned.startswith(("http://", "https://")):
            cleaned = f"https://{cleaned}"
        return cleaned

    def _state_file_path(self) -> str:
        return os.path.join(self.working_folder, STATE_FILENAME)

    def _load_state(self) -> Dict[str, Any]:
        try:
            with open(self._state_file_path(), "r", encoding="utf-8") as handle:
                data = json.load(handle)
                if isinstance(data, dict):
                    return data
        except FileNotFoundError:
            return {}
        except json.JSONDecodeError:
            return {}
        return {}

    def _save_state(self, state: Mapping[str, Any]) -> None:
        with open(self._state_file_path(), "w", encoding="utf-8") as handle:
            json.dump(dict(state), handle, indent=2, sort_keys=True)

    def _resolve_updated_since(self) -> Optional[str]:
        if self.updated_since:
            return str(self.updated_since)
        state = self._load_state()
        value = state.get("last_sync")
        if value:
            return str(value)
        return None

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

        bundle = self._collect_and_normalize(**kwargs)
        self._save_state({"last_sync": bundle["sync"]["last_sync"]})
        return bundle

    def transform(self, raw_data: Any, **kwargs: Any) -> Any:
        return self._normalize_bundle(raw_data)

    def _collect_and_normalize(self, **kwargs: Any) -> Dict[str, Any]:
        updated_since = kwargs.get("updated_since") or self._resolve_updated_since()
        device_records: List[Dict[str, Any]] = []
        relationship_records: List[Dict[str, Any]] = []

        for table in self.device_tables:
            query = self._updated_since_query(updated_since)
            result = self._client.fetch_table(
                table,
                query=query,
                page_size=self.page_size,
            )
            device_records.extend(result.records)

        relation_query = self._updated_since_query(updated_since)
        rel_result = self._client.fetch_table(
            self.relationship_table,
            query=relation_query,
            page_size=self.page_size,
        )
        relationship_records.extend(rel_result.records)

        raw_bundle = {
            "updated_since": updated_since,
            "devices": device_records,
            "relationships": relationship_records,
            "source": "servicenow_cmdb",
        }
        return self._normalize_bundle(raw_bundle)

    def _normalize_bundle(self, raw_bundle: Mapping[str, Any]) -> Dict[str, Any]:
        devices = [self._normalize_device(record) for record in raw_bundle.get("devices", []) or []]
        relationships = [self._normalize_relationship(record) for record in raw_bundle.get("relationships", []) or []]
        records = devices + relationships

        last_sync = self._latest_timestamp(
            [
                self._extract_timestamp(record.payload.get("last_updated"))
                for record in records
                if isinstance(record, NormalizedRecord)
            ]
        )
        if last_sync is None:
            last_sync = self._latest_timestamp([self._extract_timestamp(raw_bundle.get("updated_since"))])
        if last_sync is None:
            last_sync = datetime.now(timezone.utc)

        return {
            "status": "completed",
            "connector": self.connector_type or self.__class__.__name__,
            "record_type": "topology",
            "count": len(records),
            "devices": [record.__dict__ for record in devices],
            "relationships": [record.__dict__ for record in relationships],
            "records": [record.__dict__ for record in records],
            "sync": {
                "last_sync": self._format_timestamp(last_sync),
                "source": "servicenow_cmdb",
            },
        }

    def _updated_since_query(self, updated_since: Optional[str]) -> Optional[str]:
        if not updated_since:
            return None
        return f"sys_updated_on>={updated_since}"

    def _normalize_device(self, record: Mapping[str, Any]) -> NormalizedRecord:
        ci_class = self._value(record, "sys_class_name") or self._value(record, "class") or "cmdb_ci"
        model = self._value(record, "model_id") or self._value(record, "model") or ""
        hostname = self._value(record, "name") or self._value(record, "hostname") or self._value(record, "fqdn") or self._value(record, "ip_address") or self._value(record, "asset_tag") or self._value(record, "sys_id")
        normalized = {
            "id": self._value(record, "sys_id"),
            "hostname": hostname,
            "ip_address": self._value(record, "ip_address") or self._value(record, "ip"),
            "device_type": self._map_device_type(ci_class, model, hostname),
            "ci_class": ci_class,
            "vendor": self._value(record, "manufacturer") or self._value(record, "vendor") or self._value(record, "manufacturer.display_value"),
            "model": model,
            "os_version": self._value(record, "os_version") or self._value(record, "version"),
            "serial_number": self._value(record, "serial_number"),
            "last_updated": self._value(record, "sys_updated_on") or self._value(record, "updated_on"),
            "source_table": ci_class,
            "evidence": self._evidence(record, ci_class),
            "correlation_keys": self.build_correlation_keys(
                ("hostname", hostname),
                ("ip_address", self._value(record, "ip_address") or self._value(record, "ip")),
                ("serial", self._value(record, "serial_number")),
                ("ci_class", ci_class),
                ("vendor", self._value(record, "manufacturer") or self._value(record, "vendor") or self._value(record, "manufacturer.display_value")),
                ("model", model),
            ),
        }
        metadata = {
            "sys_id": self._value(record, "sys_id"),
            "sys_updated_on": self._value(record, "sys_updated_on"),
            "source_table": ci_class,
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="device",
            source_id=self._value(record, "sys_id") or hostname,
            payload=normalized,
            confidence=self._device_confidence(ci_class),
            metadata=metadata,
        )

    def _normalize_relationship(self, record: Mapping[str, Any]) -> NormalizedRecord:
        parent_id, parent_name = self._reference_value(record, "parent")
        child_id, child_name = self._reference_value(record, "child")
        rel_type = self._value(record, "type") or self._value(record, "relationship_type") or "related-to"
        normalized = {
            "id": self._value(record, "sys_id"),
            "source_id": parent_id,
            "source_name": parent_name,
            "target_id": child_id,
            "target_name": child_name,
            "relationship_type": rel_type,
            "last_updated": self._value(record, "sys_updated_on") or self._value(record, "updated_on"),
            "source_table": self.relationship_table,
            "evidence": self._evidence(record, self.relationship_table),
            "correlation_keys": self.build_correlation_keys(
                ("source_id", parent_id),
                ("source_name", parent_name),
                ("target_id", child_id),
                ("target_name", child_name),
                ("relationship_type", rel_type),
            ),
        }
        metadata = {
            "sys_id": self._value(record, "sys_id"),
            "sys_updated_on": self._value(record, "sys_updated_on"),
            "source_table": self.relationship_table,
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="link",
            source_id=self._value(record, "sys_id") or f"{parent_id}->{child_id}",
            payload=normalized,
            confidence=0.84,
            metadata=metadata,
        )

    def _evidence(self, record: Mapping[str, Any], table: str) -> Dict[str, str]:
        evidence = {
            "source": "servicenow_cmdb",
            "table": table,
            "sys_id": self._value(record, "sys_id"),
            "sys_updated_on": self._value(record, "sys_updated_on"),
        }
        return {key: value for key, value in evidence.items() if value}

    def _map_device_type(self, ci_class: str, model: str, name: str) -> str:
        haystack = f"{ci_class} {model} {name}".lower()
        if "server" in ci_class.lower():
            return "server"
        if any(token in haystack for token in ("firewall", "asa", "palo", "forti", "checkpoint", "srx", "firepower")):
            return "firewall"
        if any(token in haystack for token in ("router", "wan", "edge", "gateway")):
            return "router"
        if any(token in haystack for token in ("switch", "nexus", "catalyst", "aruba", "access")):
            return "switch"
        if "netgear" in ci_class.lower():
            return "network_device"
        return "network_device"

    def _device_confidence(self, ci_class: str) -> float:
        if ci_class.lower() == "cmdb_ci_server":
            return 0.92
        if ci_class.lower() == "cmdb_ci_netgear":
            return 0.88
        return 0.80

    def _reference_value(self, record: Mapping[str, Any], field: str) -> Tuple[str, str]:
        value = record.get(field)
        if isinstance(value, Mapping):
            return (
                str(value.get("value") or value.get("sys_id") or value.get("id") or ""),
                str(value.get("display_value") or value.get("displayValue") or value.get("name") or value.get("value") or ""),
            )
        display = record.get(f"{field}.display_value") or record.get(f"{field}_display_value")
        return str(value or ""), str(display or value or "")

    def _value(self, record: Mapping[str, Any], field: str) -> str:
        value = record.get(field)
        if isinstance(value, Mapping):
            nested = value.get("display_value") or value.get("displayValue") or value.get("value") or value.get("sys_id")
            return str(nested or "")
        display = record.get(f"{field}.display_value") or record.get(f"{field}_display_value")
        return str(display or value or "")

    def _extract_timestamp(self, value: Optional[str]) -> Optional[datetime]:
        if not value:
            return None
        text = str(value).strip()
        for candidate in (
            text,
            text.replace("Z", "+00:00"),
        ):
            try:
                return datetime.fromisoformat(candidate)
            except ValueError:
                pass
        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S"):
            try:
                return datetime.strptime(text, fmt)
            except ValueError:
                continue
        return None

    def _latest_timestamp(self, values: Iterable[Optional[datetime]]) -> Optional[datetime]:
        filtered = [value for value in values if value is not None]
        if not filtered:
            return None
        return max(filtered)

    def _format_timestamp(self, value: datetime) -> str:
        if value.tzinfo is None:
            value = value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc).isoformat()


def test_connection(instance_url: str, username: str, password: str, **kwargs: Any) -> Dict[str, Any]:
    """Quick connection helper used by tests and smoke checks."""

    connector = ServiceNowCMDBConnector(
        "servicenow-smoke",
        {
            "name": "ServiceNow CMDB Smoke Test",
            "host": instance_url,
            "connector_type": "servicenow_cmdb",
            "username": username,
            "password": password,
            "payload": kwargs,
        },
    )

    try:
        connector.connect()
        return {"status": "success", "connected": connector.connected}
    except Exception as exc:  # pragma: no cover - smoke helper only
        return {"status": "error", "error": str(exc)}


test_connection.__test__ = False
