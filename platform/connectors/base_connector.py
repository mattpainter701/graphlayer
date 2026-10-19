"""Compatibility connector lifecycle and Rust-backed normalization helpers."""

from __future__ import annotations

import abc
import os
import tempfile
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, Iterable, Mapping, Optional

from ..rust_worker import RustWorkerError, call_rust


@dataclass(frozen=True)
class NormalizedRecord:
    record_type: str
    source_connector: str
    source_id: str
    payload: Dict[str, Any]
    confidence: float = 1.0
    collected_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    raw_fingerprint: str = ""
    metadata: Dict[str, Any] = field(default_factory=dict)


class DeltaAction(str, Enum):
    ADDED = "added"
    UPDATED = "updated"
    UNCHANGED = "unchanged"
    STALE = "stale"


@dataclass(frozen=True)
class ConnectorCheckpoint:
    connector: str
    high_watermark: Optional[str] = None
    fingerprints: Dict[str, str] = field(default_factory=dict)
    source_ids: list[str] = field(default_factory=list)
    updated_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())

    def to_dict(self) -> Dict[str, Any]:
        return {"connector": self.connector, "high_watermark": self.high_watermark,
                "fingerprints": dict(sorted(self.fingerprints.items())),
                "source_ids": list(self.source_ids), "updated_at": self.updated_at}

    @classmethod
    def from_dict(cls, data: Mapping[str, Any] | None) -> "ConnectorCheckpoint":
        data = dict(data or {})
        return cls(connector=str(data.get("connector") or ""),
                   high_watermark=data.get("high_watermark"),
                   fingerprints={str(k): str(v) for k, v in dict(data.get("fingerprints") or {}).items()},
                   source_ids=[str(v) for v in data.get("source_ids") or []],
                   updated_at=str(data.get("updated_at") or datetime.now(timezone.utc).isoformat()))


@dataclass(frozen=True)
class DeltaSyncRecord:
    action: DeltaAction
    record: NormalizedRecord
    previous_fingerprint: Optional[str] = None


@dataclass(frozen=True)
class DeltaSyncResult:
    records: list[DeltaSyncRecord]
    checkpoint: ConnectorCheckpoint

    @property
    def added(self) -> list[DeltaSyncRecord]: return [item for item in self.records if item.action == DeltaAction.ADDED]
    @property
    def updated(self) -> list[DeltaSyncRecord]: return [item for item in self.records if item.action == DeltaAction.UPDATED]
    @property
    def unchanged(self) -> list[DeltaSyncRecord]: return [item for item in self.records if item.action == DeltaAction.UNCHANGED]
    @property
    def stale(self) -> list[DeltaSyncRecord]: return [item for item in self.records if item.action == DeltaAction.STALE]

    def summary(self) -> Dict[str, int]:
        return {"added": len(self.added), "updated": len(self.updated),
                "unchanged": len(self.unchanged), "stale": len(self.stale), "total": len(self.records)}


class ConnectorContract(abc.ABC):
    @abc.abstractmethod
    def connect(self, *args: Any, **kwargs: Any) -> Any: ...
    @abc.abstractmethod
    def collect(self, *args: Any, **kwargs: Any) -> Any: ...
    @abc.abstractmethod
    def transform(self, raw_data: Any, **kwargs: Any) -> Any: ...
    @abc.abstractmethod
    def close(self, *args: Any, **kwargs: Any) -> Any: ...


class BaseConnector(ConnectorContract):
    """Lifecycle adapter retaining the Python constructor and method contracts."""

    def __init__(self, guid: str, connector_data: Optional[Mapping[str, Any]] = None,
                 working_folder: Optional[str] = None, task_guid: Optional[str] = None) -> None:
        connector_data = dict(connector_data or {})
        payload = dict(connector_data.get("payload", {}) or {})
        credentials = dict(connector_data.get("credentials", {}) or {})
        self.guid = guid
        self.task_guid = task_guid
        self.connector_data = connector_data
        self.name = connector_data.get("name", guid)
        self.connector_type = connector_data.get("connector_type", "")
        self.host = connector_data.get("host", payload.get("host", ""))
        self.username = connector_data.get("username", credentials.get("username", ""))
        self.password = connector_data.get("password", credentials.get("password", ""))
        self.devices = list(connector_data.get("devices", []))
        self.payload = payload
        self.credentials = credentials
        self.working_folder = working_folder or connector_data.get("working_folder") or os.path.join(
            tempfile.gettempdir(), f"graphlayer-{guid}")
        self.state = "disconnected"
        self.connected = False
        os.makedirs(self.working_folder, exist_ok=True)

    def Connect(self, *args: Any, **kwargs: Any) -> Any: return self.connect(*args, **kwargs)
    def Collect(self, *args: Any, **kwargs: Any) -> Any: return self.collect(*args, **kwargs)
    def Transform(self, raw_data: Any, **kwargs: Any) -> Any: return self.transform(raw_data, **kwargs)
    def Close(self, *args: Any, **kwargs: Any) -> Any: return self.close(*args, **kwargs)

    def collect(self, *args: Any, **kwargs: Any) -> Any:
        if hasattr(self, "do_download") and callable(getattr(self, "do_download")):
            return self.do_download(*args, **kwargs)
        raise NotImplementedError("collect() requires a do_download() implementation")

    def transform(self, raw_data: Any, **kwargs: Any) -> Any: return raw_data

    def close(self, *args: Any, **kwargs: Any) -> Any:
        disconnect = getattr(self, "disconnect", None)
        result = disconnect(*args, **kwargs) if callable(disconnect) else None
        self.connected = False
        self.state = "disconnected"
        return result

    @staticmethod
    def _rust(operation: str, input_data: Any) -> Any:
        try:
            return call_rust(operation, input_data)
        except RustWorkerError as exc:
            raise RuntimeError(str(exc)) from exc

    @staticmethod
    def normalize_hostname(name: str) -> str:
        return BaseConnector._rust("connector.normalize_hostname", {"hostname": name})

    @staticmethod
    def normalize_path(path: str) -> str:
        return BaseConnector._rust("connector.normalize_path", {"path": path})

    @staticmethod
    def canonical_fingerprint(data: Any) -> str:
        return BaseConnector._rust("connector.fingerprint", {"data": data})

    def build_normalized_record(self, record_type: str, source_id: str,
                                payload: Mapping[str, Any], confidence: float = 1.0,
                                metadata: Optional[Mapping[str, Any]] = None) -> NormalizedRecord:
        raw = self._rust("connector.build_record", {
            "record_type": record_type, "source_connector": self.connector_type or self.__class__.__name__,
            "source_id": source_id, "payload": dict(payload), "confidence": confidence,
            "metadata": dict(metadata or {}),
        })
        return _record_from_dict(raw)

    @staticmethod
    def normalize_correlation_value(value: Any) -> str:
        return BaseConnector._rust("connector.normalize_correlation_value", {"value": value})

    def build_correlation_keys(self, *pairs: Any) -> list[str]:
        normalized = [[item[0], item[1]] for item in pairs if isinstance(item, tuple) and len(item) == 2]
        return self._rust("connector.correlation_keys", {"pairs": normalized})

    @staticmethod
    def _dedup_key_for_record(record: NormalizedRecord) -> str:
        return BaseConnector._rust("connector.dedup_key", {"record": record.__dict__})

    def deduplicate_normalized_records(self, records: Iterable[NormalizedRecord]) -> list[NormalizedRecord]:
        raw = self._rust("connector.deduplicate", {"records": [_record_dict(r) for r in records]})
        return [_record_from_dict(item) for item in raw]

    def build_correlation_report(self, records: Iterable[NormalizedRecord]) -> Dict[str, Any]:
        return self._rust("connector.correlation_report", {"records": [_record_dict(r) for r in records]})

    @staticmethod
    def filter_normalized_records_by_confidence(records: Iterable[NormalizedRecord],
                                                minimum_confidence: float) -> list[NormalizedRecord]:
        try:
            raw = BaseConnector._rust("connector.filter_confidence", {
                "records": [_record_dict(r) for r in records], "minimum_confidence": minimum_confidence})
        except RuntimeError as exc:
            raise ValueError(str(exc)) from exc
        return [_record_from_dict(item) for item in raw]

    @staticmethod
    def apply_staleness_decay(confidence: float, collected_at: str, *,
                              now: Optional[datetime] = None, half_life_days: float = 30.0) -> float:
        try:
            return BaseConnector._rust("connector.staleness_decay", {
                "confidence": confidence, "collected_at": collected_at,
                "now": now.isoformat() if now else None, "half_life_days": half_life_days})
        except RuntimeError as exc:
            raise ValueError(str(exc)) from exc

    def build_normalized_payload(self, record_type: str, records: Iterable[Mapping[str, Any]],
                                 confidence: float = 1.0,
                                 metadata: Optional[Mapping[str, Any]] = None,
                                 minimum_confidence: Optional[float] = None) -> Dict[str, Any]:
        return self._rust("connector.build_payload", {
            "record_type": record_type, "source_connector": self.connector_type or self.__class__.__name__,
            "records": [dict(record) for record in records], "confidence": confidence,
            "metadata": dict(metadata or {}), "minimum_confidence": minimum_confidence})

    def build_delta_sync_result(self, records: Iterable[NormalizedRecord],
                                previous_checkpoint: Optional[ConnectorCheckpoint | Mapping[str, Any]] = None,
                                *, high_watermark: Optional[str] = None,
                                include_stale: bool = True) -> DeltaSyncResult:
        prior = (previous_checkpoint.to_dict() if isinstance(previous_checkpoint, ConnectorCheckpoint)
                 else dict(previous_checkpoint or {}))
        result = self._rust("connector.delta_sync", {
            "source_connector": self.connector_type or self.__class__.__name__,
            "records": [_record_dict(r) for r in records], "previous_checkpoint": prior,
            "high_watermark": high_watermark, "include_stale": include_stale})
        deltas = [DeltaSyncRecord(DeltaAction(item["action"]), _record_from_dict(item["record"]),
                                 item.get("previous_fingerprint")) for item in result["records"]]
        return DeltaSyncResult(deltas, ConnectorCheckpoint.from_dict(result["checkpoint"]))

    def validate_required_fields(self, fields: Iterable[str]) -> None:
        fields = list(fields)
        self._rust("connector.validate_required_fields", {
            "fields": fields, "values": {field: getattr(self, field, None) for field in fields}})

    def mark_connected(self) -> None:
        self.connected = True
        self.state = "connected"

    def mark_disconnected(self) -> None:
        self.connected = False
        self.state = "disconnected"


def _record_dict(record: NormalizedRecord) -> Dict[str, Any]:
    return record.__dict__


def _record_from_dict(data: Mapping[str, Any]) -> NormalizedRecord:
    return NormalizedRecord(
        record_type=str(data.get("record_type") or ""),
        source_connector=str(data.get("source_connector") or ""),
        source_id=str(data.get("source_id") or ""),
        payload=dict(data.get("payload") or {}),
        confidence=float(data.get("confidence", 1.0)),
        collected_at=str(data.get("collected_at") or datetime.now(timezone.utc).isoformat()),
        raw_fingerprint=str(data.get("raw_fingerprint") or ""),
        metadata=dict(data.get("metadata") or {}),
    )
