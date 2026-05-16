"""Shared connector contract and normalization helpers for Python connectors.

This module gives the Python connector implementations a small, explicit
contract that mirrors the Sprint 2 roadmap:

- Connect() / Collect() / Transform() / Close()
- normalized records with provenance and deterministic fingerprints
- lightweight state tracking for tests and harnesses
"""

from __future__ import annotations

import abc
import hashlib
import json
import os
import tempfile
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, Mapping, MutableMapping, Optional


@dataclass(frozen=True)
class NormalizedRecord:
    """Canonical connector output with provenance attached."""

    record_type: str
    source_connector: str
    source_id: str
    payload: Dict[str, Any]
    confidence: float = 1.0
    collected_at: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    raw_fingerprint: str = ""
    metadata: Dict[str, Any] = field(default_factory=dict)


class ConnectorContract(abc.ABC):
    """Minimal interface expected by the Sprint 2 connector harness."""

    @abc.abstractmethod
    def connect(self, *args: Any, **kwargs: Any) -> Any:
        raise NotImplementedError

    @abc.abstractmethod
    def collect(self, *args: Any, **kwargs: Any) -> Any:
        raise NotImplementedError

    @abc.abstractmethod
    def transform(self, *args: Any, **kwargs: Any) -> Any:
        raise NotImplementedError

    @abc.abstractmethod
    def close(self, *args: Any, **kwargs: Any) -> Any:
        raise NotImplementedError


class BaseConnector(ConnectorContract):
    """Common setup, lifecycle wrappers, and normalization helpers."""

    def __init__(
        self,
        guid: str,
        connector_data: Optional[Mapping[str, Any]] = None,
        working_folder: Optional[str] = None,
        task_guid: Optional[str] = None,
    ) -> None:
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
            tempfile.gettempdir(), f"graphlayer-{guid}"
        )
        self.state = "disconnected"
        self.connected = False

        os.makedirs(self.working_folder, exist_ok=True)

    # ------------------------------------------------------------------
    # Sprint 2 contract aliases
    # ------------------------------------------------------------------
    def Connect(self, *args: Any, **kwargs: Any) -> Any:
        return self.connect(*args, **kwargs)

    def Collect(self, *args: Any, **kwargs: Any) -> Any:
        return self.collect(*args, **kwargs)

    def Transform(self, *args: Any, **kwargs: Any) -> Any:
        return self.transform(*args, **kwargs)

    def Close(self, *args: Any, **kwargs: Any) -> Any:
        return self.close(*args, **kwargs)

    # ------------------------------------------------------------------
    # Default lifecycle helpers
    # ------------------------------------------------------------------
    def collect(self, *args: Any, **kwargs: Any) -> Any:
        if hasattr(self, "do_download") and callable(getattr(self, "do_download")):
            return self.do_download(*args, **kwargs)
        raise NotImplementedError("collect() requires a do_download() implementation")

    def transform(self, raw_data: Any, **kwargs: Any) -> Any:
        """Default transform is identity; harnesses can override per connector."""

        return raw_data

    def close(self, *args: Any, **kwargs: Any) -> Any:
        disconnect = getattr(self, "disconnect", None)
        if callable(disconnect):
            result = disconnect(*args, **kwargs)
        else:
            result = None
        self.connected = False
        self.state = "disconnected"
        return result

    # ------------------------------------------------------------------
    # Normalization helpers
    # ------------------------------------------------------------------
    @staticmethod
    def normalize_hostname(name: str) -> str:
        return name.strip().lower()

    @staticmethod
    def normalize_path(path: str) -> str:
        cleaned = str(path or "").strip().replace("\\", "/")
        if not cleaned or cleaned == "/":
            return ""
        cleaned = os.path.normpath(cleaned)
        cleaned = cleaned.replace("\\", "/")
        if cleaned == ".":
            return ""
        if cleaned.startswith("./"):
            cleaned = cleaned[2:]
        if cleaned != "/" and cleaned.endswith("/"):
            cleaned = cleaned[:-1]
        return cleaned

    @staticmethod
    def canonical_fingerprint(data: Any) -> str:
        serialized = json.dumps(data, sort_keys=True, separators=(",", ":"), ensure_ascii=False, default=str)
        return f"sha256:{hashlib.sha256(serialized.encode('utf-8')).hexdigest()}"

    def build_normalized_record(
        self,
        record_type: str,
        source_id: str,
        payload: Mapping[str, Any],
        confidence: float = 1.0,
        metadata: Optional[Mapping[str, Any]] = None,
    ) -> NormalizedRecord:
        raw_payload = dict(payload)
        return NormalizedRecord(
            record_type=record_type,
            source_connector=self.connector_type or self.__class__.__name__,
            source_id=source_id,
            payload=raw_payload,
            confidence=confidence,
            raw_fingerprint=self.canonical_fingerprint(raw_payload),
            metadata=dict(metadata or {}),
        )

    @staticmethod
    def normalize_correlation_value(value: Any) -> str:
        text = str(value or "").strip()
        if not text:
            return ""
        return text.lower()

    def build_correlation_keys(self, *pairs: Any) -> list[str]:
        """Build deterministic cross-source correlation keys.

        Each input should be a ``(label, value)`` pair. Labels are normalized to
        lower-case and values are stripped/lower-cased so the resulting keys are
        stable across connector sources.
        """

        keys: list[str] = []
        seen: set[str] = set()
        for item in pairs:
            if not isinstance(item, tuple) or len(item) != 2:
                continue
            label, value = item
            normalized_value = self.normalize_correlation_value(value)
            if not normalized_value:
                continue
            normalized_label = str(label or "").strip().lower()
            if not normalized_label:
                continue
            key = f"{normalized_label}:{normalized_value}"
            if key in seen:
                continue
            seen.add(key)
            keys.append(key)
        return keys

    @staticmethod
    def _dedup_key_for_record(record: NormalizedRecord) -> str:
        """Derive a stable deduplication key for a normalized record.

        Prefer the raw fingerprint when available because it reflects the full
        normalized payload. Fall back to correlation keys when a caller is
        deduplicating partially normalized records or transformed payloads.
        """

        if record.raw_fingerprint:
            return f"fingerprint:{record.raw_fingerprint}"

        correlation_keys = record.payload.get("correlation_keys", [])
        if isinstance(correlation_keys, (list, tuple, set)) and correlation_keys:
            normalized_keys = sorted({str(key).strip().lower() for key in correlation_keys if str(key).strip()})
            if normalized_keys:
                return f"correlation:{'|'.join(normalized_keys)}"

        return f"source:{record.source_connector}:{record.record_type}:{record.source_id}"

    def deduplicate_normalized_records(self, records: Iterable[NormalizedRecord]) -> list[NormalizedRecord]:
        """Collapse duplicate normalized records, keeping the highest-confidence copy.

        Records are considered duplicates when they share the same raw fingerprint
        or, if that is unavailable, the same correlation-key signature. If two
        duplicates have the same confidence, the first record wins to preserve
        input order stability.
        """

        winners: dict[str, NormalizedRecord] = {}
        order: list[str] = []
        for record in records:
            key = self._dedup_key_for_record(record)
            existing = winners.get(key)
            if existing is None:
                winners[key] = record
                order.append(key)
                continue
            if record.confidence > existing.confidence:
                winners[key] = record

        return [winners[key] for key in order]

    def build_correlation_report(self, records: Iterable[NormalizedRecord]) -> Dict[str, Any]:
        """Summarize correlation groups and confidence signals for normalized records.

        The report is intentionally lightweight: it exposes the grouped dedup keys,
        the source IDs that contributed to each group, the shared correlation keys,
        and a small confidence summary. This makes it easy for harnesses and tests
        to inspect why a record was treated as a duplicate without altering the
        underlying deduplication behavior.
        """

        records_list = list(records)
        grouped_records: dict[str, list[NormalizedRecord]] = {}
        ordered_keys: list[str] = []
        for record in records_list:
            key = self._dedup_key_for_record(record)
            if key not in grouped_records:
                grouped_records[key] = []
                ordered_keys.append(key)
            grouped_records[key].append(record)

        deduped_records = self.deduplicate_normalized_records(records_list)
        winner_by_key = {self._dedup_key_for_record(record): record for record in deduped_records}

        groups: list[Dict[str, Any]] = []
        for key in ordered_keys:
            group_records = grouped_records[key]
            confidences = [record.confidence for record in group_records]
            correlation_keys = sorted(
                {
                    str(correlation_key).strip().lower()
                    for record in group_records
                    for correlation_key in record.payload.get("correlation_keys", [])
                    if str(correlation_key).strip()
                }
            )
            groups.append(
                {
                    "dedup_key": key,
                    "record_count": len(group_records),
                    "source_ids": [record.source_id for record in group_records],
                    "source_connectors": [record.source_connector for record in group_records],
                    "correlation_keys": correlation_keys,
                    "confidence": {
                        "min": min(confidences),
                        "max": max(confidences),
                        "mean": round(sum(confidences) / len(confidences), 3),
                    },
                    "selected_source_id": winner_by_key[key].source_id,
                    "selected_confidence": winner_by_key[key].confidence,
                }
            )

        return {
            "total_records": len(records_list),
            "unique_groups": len(groups),
            "duplicate_groups": sum(1 for group in groups if group["record_count"] > 1),
            "groups": groups,
        }

    def build_normalized_payload(
        self,
        record_type: str,
        records: Iterable[Mapping[str, Any]],
        confidence: float = 1.0,
        metadata: Optional[Mapping[str, Any]] = None,
    ) -> Dict[str, Any]:
        normalized_records = [
            self.build_normalized_record(
                record_type=record_type,
                source_id=str(record.get("id") or record.get("name") or index),
                payload=record,
                confidence=confidence,
                metadata=metadata,
            )
            for index, record in enumerate(records)
        ]
        return {
            "record_type": record_type,
            "connector": self.connector_type or self.__class__.__name__,
            "count": len(normalized_records),
            "records": [record.__dict__ for record in normalized_records],
        }

    def validate_required_fields(self, fields: Iterable[str]) -> None:
        missing = [field for field in fields if not getattr(self, field, None)]
        if missing:
            raise ValueError(f"missing required connector fields: {', '.join(missing)}")

    def mark_connected(self) -> None:
        self.connected = True
        self.state = "connected"

    def mark_disconnected(self) -> None:
        self.connected = False
        self.state = "disconnected"
