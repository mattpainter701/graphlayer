"""NetBox REST API connector for GraphLayer.

Focused Sprint 4 subset:
- NetBox REST API client with pagination
- device / interface / cable / IPAM prefix normalization
- recorded-response friendly transform path
- dependency-free implementation for tests and local runs
"""

from __future__ import annotations

import json
import os
import ssl
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, Iterable, List, Mapping, Optional, Sequence
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urljoin
from urllib.request import Request, urlopen

from .base_connector import BaseConnector, NormalizedRecord


DEFAULT_TIMEOUT_SECONDS = 30.0
DEFAULT_PAGE_SIZE = 1000


class NetBoxAPIError(RuntimeError):
    """Raised when the NetBox API returns an error response."""


@dataclass(frozen=True)
class NetBoxQueryResult:
    """Raw records returned from NetBox plus query metadata."""

    endpoint: str
    records: List[Dict[str, Any]]
    query: Optional[str] = None
    fields: Optional[Sequence[str]] = None
    page_size: int = DEFAULT_PAGE_SIZE


class NetBoxAPIClient:
    """Small NetBox REST client with pagination support."""

    def __init__(
        self,
        base_url: str,
        token: str,
        *,
        verify_ssl: bool = True,
        timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
        default_page_size: int = DEFAULT_PAGE_SIZE,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
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
        if self.token:
            request.add_header("Authorization", f"Token {self.token}")
        request.add_header("Accept", "application/json")

        try:
            with urlopen(request, timeout=self.timeout_seconds, context=self._ssl_context()) as response:
                raw_body = response.read().decode("utf-8")
                if not raw_body.strip():
                    return {}
                return json.loads(raw_body)
        except HTTPError as exc:  # pragma: no cover - surfaced via recorded-response tests
            body = exc.read().decode("utf-8", errors="replace") if exc.fp else ""
            raise NetBoxAPIError(f"NetBox API returned HTTP {exc.code} for {url}: {body}") from exc
        except URLError as exc:  # pragma: no cover - environment specific
            raise NetBoxAPIError(f"NetBox API request failed for {url}: {exc.reason}") from exc
        except json.JSONDecodeError as exc:
            raise NetBoxAPIError(f"NetBox API returned invalid JSON for {url}") from exc

    def fetch_endpoint_page(
        self,
        endpoint: str,
        *,
        query: Optional[str] = None,
        fields: Optional[Sequence[str]] = None,
        page_size: Optional[int] = None,
        offset: int = 0,
    ) -> Dict[str, Any]:
        params: Dict[str, Any] = {
            "limit": page_size or self.default_page_size,
            "offset": offset,
        }
        if query:
            params["q"] = query
        if fields:
            params["fields"] = ",".join(fields)
        return self._request_json(f"/api/{endpoint.lstrip('/')}", params)

    def fetch_endpoint(
        self,
        endpoint: str,
        *,
        query: Optional[str] = None,
        fields: Optional[Sequence[str]] = None,
        page_size: Optional[int] = None,
    ) -> NetBoxQueryResult:
        limit = page_size or self.default_page_size
        offset = 0
        records: List[Dict[str, Any]] = []

        while True:
            page = self.fetch_endpoint_page(endpoint, query=query, fields=fields, page_size=limit, offset=offset)
            page_records = list(page.get("results", []) or [])
            if not isinstance(page_records, list):
                raise NetBoxAPIError(f"NetBox API returned malformed records for {endpoint}")
            records.extend(page_records)

            next_page = page.get("next")
            if next_page:
                offset += limit
                continue
            if len(page_records) < limit:
                break
            offset += limit

        return NetBoxQueryResult(
            endpoint=endpoint,
            records=records,
            query=query,
            fields=fields,
            page_size=limit,
        )

    def ping(self) -> bool:
        self._request_json("/api/", {})
        return True


class NetBoxConnector(BaseConnector):
    """NetBox connector covering devices, interfaces, cables, and IPAM prefixes."""

    SUPPORTS_DEVICES = True

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None, task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)

        payload = dict(connector_data.get("payload", {}) or {})
        credentials = dict(connector_data.get("credentials", {}) or {})

        self.instance_url = self._normalize_instance_url(
            payload.get("instance_url")
            or connector_data.get("instance_url")
            or self.host
        )
        self.api_token = str(
            payload.get("api_token")
            or credentials.get("api_token")
            or credentials.get("token")
            or connector_data.get("api_token")
            or ""
        )
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
            self.instance_url,
            self.api_token,
            verify_ssl=self.verify_ssl,
            timeout_seconds=self.timeout_seconds,
            default_page_size=self.page_size,
        )

    def _normalize_instance_url(self, url: str) -> str:
        cleaned = str(url or "").strip().rstrip("/")
        if not cleaned:
            raise ValueError("instance_url or host is required for NetBox")
        if not cleaned.startswith(("http://", "https://")):
            cleaned = f"https://{cleaned}"
        return cleaned

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
        return bundle

    def transform(self, raw_data: Any, **kwargs: Any) -> Any:
        return self._normalize_bundle(raw_data)

    def _collect_and_normalize(self, **kwargs: Any) -> Dict[str, Any]:
        device_filter = kwargs.get("site") or self.site_filter
        role_filter = kwargs.get("role") or self.role_filter

        raw_bundle = {
            "source": "netbox",
            "devices": self._client.fetch_endpoint(
                self.device_endpoint,
                query=self._build_filter_query(device_filter, role_filter),
                page_size=self.page_size,
            ).records,
            "interfaces": self._client.fetch_endpoint(
                self.interface_endpoint,
                page_size=self.page_size,
            ).records,
            "cables": self._client.fetch_endpoint(
                self.cable_endpoint,
                page_size=self.page_size,
            ).records,
            "prefixes": self._client.fetch_endpoint(
                self.prefix_endpoint,
                page_size=self.page_size,
            ).records,
            "ip_addresses": self._client.fetch_endpoint(
                self.ip_endpoint,
                page_size=self.page_size,
            ).records,
        }
        return self._normalize_bundle(raw_bundle)

    def _normalize_bundle(self, raw_bundle: Mapping[str, Any]) -> Dict[str, Any]:
        devices_raw = list(raw_bundle.get("devices", []) or [])
        interfaces_raw = list(raw_bundle.get("interfaces", []) or [])
        cables_raw = list(raw_bundle.get("cables", []) or [])
        prefixes_raw = list(raw_bundle.get("prefixes", []) or [])
        ip_addresses_raw = list(raw_bundle.get("ip_addresses", []) or [])

        ip_assignments = self._build_ip_assignments(ip_addresses_raw)
        devices = [self._normalize_device(record) for record in devices_raw]
        interfaces = [self._normalize_interface(record, ip_assignments) for record in interfaces_raw]
        links = [self._normalize_cable(record) for record in cables_raw]
        subnets = [self._normalize_prefix(record) for record in prefixes_raw]
        addresses = [self._normalize_ip_address(record) for record in ip_addresses_raw]

        records = devices + interfaces + links + subnets + addresses
        last_sync = self._latest_timestamp(
            [self._extract_timestamp(record.payload.get("last_updated")) for record in records]
        )
        if last_sync is None:
            last_sync = datetime.now(timezone.utc)

        return {
            "status": "completed",
            "connector": self.connector_type or self.__class__.__name__,
            "record_type": "topology",
            "count": len(records),
            "devices": [record.__dict__ for record in devices],
            "interfaces": [record.__dict__ for record in interfaces],
            "links": [record.__dict__ for record in links],
            "subnets": [record.__dict__ for record in subnets],
            "addresses": [record.__dict__ for record in addresses],
            "records": [record.__dict__ for record in records],
            "sync": {
                "last_sync": self._format_timestamp(last_sync),
                "source": "netbox",
            },
        }

    def _build_filter_query(self, site: Optional[str], role: Optional[str]) -> Optional[str]:
        parts = []
        if site:
            parts.append(f"site={site}")
        if role:
            parts.append(f"role={role}")
        return " ".join(parts) if parts else None

    def _build_ip_assignments(self, ip_addresses: Iterable[Mapping[str, Any]]) -> Dict[str, List[Dict[str, Any]]]:
        assignments: Dict[str, List[Dict[str, Any]]] = {}
        for record in ip_addresses:
            assigned_type = self._value(record, "assigned_object_type")
            assigned_id = self._value(record, "assigned_object_id")
            if not assigned_type or not assigned_id:
                continue
            if "interface" not in assigned_type.lower():
                continue
            assignments.setdefault(assigned_id, []).append(
                {
                    "id": self._value(record, "id"),
                    "address": self._value(record, "address") or self._value(record, "display"),
                    "family": self._value(record, "family"),
                    "status": self._value(record, "status"),
                    "dns_name": self._value(record, "dns_name"),
                    "description": self._value(record, "description"),
                }
            )
        return assignments

    def _normalize_device(self, record: Mapping[str, Any]) -> NormalizedRecord:
        device_type = self._nested_value(record, "device_type", "display") or self._nested_value(record, "device_type", "model")
        manufacturer = self._nested_value(record, "device_type", "manufacturer", "display") or self._nested_value(record, "device_type", "manufacturer", "name")
        hostname = self._value(record, "name") or self._value(record, "display") or self._value(record, "id")
        normalized = {
            "id": self._value(record, "id"),
            "hostname": hostname,
            "device_type": self._map_device_type(record),
            "vendor": manufacturer,
            "model": device_type,
            "role": self._nested_value(record, "device_role", "slug") or self._nested_value(record, "device_role", "name"),
            "site": self._nested_value(record, "site", "slug") or self._nested_value(record, "site", "name"),
            "management_ip": self._primary_ip(record),
            "serial_number": self._value(record, "serial"),
            "status": self._value(record, "status"),
            "last_updated": self._value(record, "last_updated"),
            "custom_fields": self._value(record, "custom_fields", allow_mapping=True),
        }
        metadata = {
            "netbox_id": self._value(record, "id"),
            "last_updated": self._value(record, "last_updated"),
            "endpoint": "dcim/devices",
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="device",
            source_id=self._value(record, "id") or hostname,
            payload=normalized,
            confidence=0.94,
            metadata=metadata,
        )

    def _normalize_interface(
        self,
        record: Mapping[str, Any],
        ip_assignments: Mapping[str, List[Dict[str, Any]]],
    ) -> NormalizedRecord:
        assigned_ips = ip_assignments.get(self._value(record, "id"), [])
        normalized = {
            "id": self._value(record, "id"),
            "device_id": self._nested_value(record, "device", "id"),
            "device_name": self._nested_value(record, "device", "name"),
            "name": self._value(record, "name"),
            "type": self._value(record, "type"),
            "enabled": self._bool_value(record, "enabled"),
            "description": self._value(record, "description"),
            "label": self._value(record, "label"),
            "mac_address": self._value(record, "mac_address"),
            "mtu": self._int_value(record, "mtu"),
            "lag": self._nested_value(record, "lag", "name") or self._nested_value(record, "lag", "id"),
            "untagged_vlan": self._nested_value(record, "untagged_vlan", "vid") or self._nested_value(record, "untagged_vlan", "name"),
            "tagged_vlans": [self._nested_value(vlan, "vid") or self._nested_value(vlan, "name") for vlan in self._iter_list(record.get("tagged_vlans"))],
            "ip_addresses": assigned_ips,
            "last_updated": self._value(record, "last_updated"),
            "custom_fields": self._value(record, "custom_fields", allow_mapping=True),
        }
        metadata = {
            "netbox_id": self._value(record, "id"),
            "device_id": self._nested_value(record, "device", "id"),
            "endpoint": "dcim/interfaces",
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="interface",
            source_id=self._value(record, "id") or f"{normalized['device_id']}:{normalized['name']}",
            payload=normalized,
            confidence=0.91,
            metadata=metadata,
        )

    def _normalize_cable(self, record: Mapping[str, Any]) -> NormalizedRecord:
        term_a = self._normalize_termination(record, "a")
        term_b = self._normalize_termination(record, "b")
        normalized = {
            "id": self._value(record, "id"),
            "status": self._value(record, "status"),
            "label": self._value(record, "label"),
            "type": self._value(record, "type"),
            "length": self._value(record, "length"),
            "length_unit": self._value(record, "length_unit"),
            "termination_a": term_a,
            "termination_b": term_b,
            "last_updated": self._value(record, "last_updated"),
            "custom_fields": self._value(record, "custom_fields", allow_mapping=True),
        }
        metadata = {
            "netbox_id": self._value(record, "id"),
            "endpoint": "dcim/cables",
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="link",
            source_id=self._value(record, "id") or f"{term_a.get('endpoint')}->{term_b.get('endpoint')}",
            payload=normalized,
            confidence=0.95,
            metadata=metadata,
        )

    def _normalize_prefix(self, record: Mapping[str, Any]) -> NormalizedRecord:
        normalized = {
            "id": self._value(record, "id"),
            "prefix": self._value(record, "prefix"),
            "status": self._value(record, "status"),
            "role": self._nested_value(record, "role", "slug") or self._nested_value(record, "role", "name"),
            "site": self._nested_value(record, "site", "slug") or self._nested_value(record, "site", "name"),
            "tenant": self._nested_value(record, "tenant", "slug") or self._nested_value(record, "tenant", "name"),
            "vlan": self._nested_value(record, "vlan", "vid") or self._nested_value(record, "vlan", "name"),
            "description": self._value(record, "description"),
            "family": self._value(record, "family"),
            "is_pool": self._bool_value(record, "is_pool"),
            "last_updated": self._value(record, "last_updated"),
            "custom_fields": self._value(record, "custom_fields", allow_mapping=True),
        }
        metadata = {
            "netbox_id": self._value(record, "id"),
            "endpoint": "ipam/prefixes",
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="subnet",
            source_id=self._value(record, "id") or normalized["prefix"],
            payload=normalized,
            confidence=0.88,
            metadata=metadata,
        )

    def _normalize_ip_address(self, record: Mapping[str, Any]) -> NormalizedRecord:
        assigned_type = self._value(record, "assigned_object_type")
        assigned_id = self._value(record, "assigned_object_id")
        normalized = {
            "id": self._value(record, "id"),
            "address": self._value(record, "address"),
            "family": self._value(record, "family"),
            "status": self._value(record, "status"),
            "assigned_object_type": assigned_type,
            "assigned_object_id": assigned_id,
            "assigned_object_name": self._nested_value(record, "assigned_object", "name"),
            "device_name": self._nested_value(record, "assigned_object", "device", "name"),
            "interface_name": self._nested_value(record, "assigned_object", "name"),
            "dns_name": self._value(record, "dns_name"),
            "description": self._value(record, "description"),
            "last_updated": self._value(record, "last_updated"),
            "custom_fields": self._value(record, "custom_fields", allow_mapping=True),
        }
        metadata = {
            "netbox_id": self._value(record, "id"),
            "endpoint": "ipam/ip-addresses",
            "connector": self.connector_type or self.__class__.__name__,
        }
        return self.build_normalized_record(
            record_type="address",
            source_id=self._value(record, "id") or normalized["address"],
            payload=normalized,
            confidence=0.86,
            metadata=metadata,
        )

    def _normalize_termination(self, record: Mapping[str, Any], suffix: str) -> Dict[str, Any]:
        endpoint_key = f"termination_{suffix}"
        nested = record.get(endpoint_key)
        if isinstance(nested, Mapping):
            return {
                "endpoint": self._value(nested, "name") or self._value(nested, "display") or self._value(nested, "id"),
                "device_id": self._nested_value(nested, "device", "id"),
                "device_name": self._nested_value(nested, "device", "name"),
                "interface_id": self._value(nested, "id"),
                "interface_name": self._value(nested, "name"),
                "termination_type": self._value(nested, "_type") or self._value(record, f"termination_{suffix}_type"),
            }
        return {
            "endpoint": self._value(record, endpoint_key),
            "device_id": self._value(record, f"termination_{suffix}_object_id") or self._value(record, f"termination_{suffix}_id"),
            "device_name": self._value(record, f"termination_{suffix}_object_name"),
            "interface_id": self._value(record, f"termination_{suffix}_id"),
            "interface_name": self._value(record, f"termination_{suffix}"),
            "termination_type": self._value(record, f"termination_{suffix}_type"),
        }

    def _map_device_type(self, record: Mapping[str, Any]) -> str:
        role = (
            self._nested_value(record, "device_role", "slug")
            or self._nested_value(record, "device_role", "name")
            or ""
        ).lower()
        model = (
            self._nested_value(record, "device_type", "display")
            or self._nested_value(record, "device_type", "model")
            or ""
        ).lower()
        haystack = f"{role} {model}"
        if any(token in haystack for token in ("firewall", "fw", "security")):
            return "firewall"
        if any(token in haystack for token in ("router", "wan", "edge")):
            return "router"
        if any(token in haystack for token in ("switch", "access", "leaf", "spine")):
            return "switch"
        if any(token in haystack for token in ("server", "compute", "host")):
            return "server"
        return "network_device"

    def _primary_ip(self, record: Mapping[str, Any]) -> str:
        return self._nested_value(record, "primary_ip4", "address") or self._nested_value(record, "primary_ip6", "address")

    def _nested_value(self, record: Mapping[str, Any], *fields: str) -> str:
        current: Any = record
        for field in fields:
            if not isinstance(current, Mapping):
                return ""
            current = current.get(field)
            if current is None:
                return ""
        if isinstance(current, Mapping):
            for key in ("display", "display_value", "name", "slug", "value", "address", "id"):
                if key in current and current[key] not in (None, ""):
                    return str(current[key])
            return ""
        return str(current or "")

    def _value(self, record: Mapping[str, Any], field: str, *, allow_mapping: bool = False) -> Any:
        value = record.get(field)
        if isinstance(value, Mapping):
            if allow_mapping:
                return dict(value)
            for key in ("display", "display_value", "name", "slug", "value", "address", "id"):
                nested = value.get(key)
                if nested not in (None, ""):
                    return str(nested)
            return ""
        return value if allow_mapping else str(value or "")

    def _iter_list(self, value: Any) -> List[Any]:
        if value is None:
            return []
        if isinstance(value, list):
            return value
        return [value]

    def _bool_value(self, record: Mapping[str, Any], field: str) -> bool:
        value = record.get(field)
        if isinstance(value, bool):
            return value
        if isinstance(value, str):
            return value.strip().lower() in {"true", "1", "yes", "on"}
        return bool(value)

    def _int_value(self, record: Mapping[str, Any], field: str) -> Optional[int]:
        value = record.get(field)
        if value in (None, ""):
            return None
        try:
            return int(value)
        except (TypeError, ValueError):
            return None

    def _extract_timestamp(self, value: Optional[str]) -> Optional[datetime]:
        if not value:
            return None
        text = str(value).strip()
        for candidate in (text, text.replace("Z", "+00:00")):
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


def test_connection(instance_url: str, api_token: str, **kwargs: Any) -> Dict[str, Any]:
    """Quick connection helper used by tests and smoke checks."""

    connector = NetBoxConnector(
        "netbox-smoke",
        {
            "name": "NetBox Smoke Test",
            "host": instance_url,
            "connector_type": "netbox",
            "credentials": {"api_token": api_token},
            "payload": kwargs,
        },
    )

    try:
        connector.connect()
        return {"status": "success", "connected": connector.connected}
    except Exception as exc:  # pragma: no cover - smoke helper only
        return {"status": "error", "error": str(exc)}
