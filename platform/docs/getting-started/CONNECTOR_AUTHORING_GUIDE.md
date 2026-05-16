# Connector Authoring Guide

GraphLayer connectors convert an external source of truth into normalized topology records. Sprint 2 defines a small contract so new sources can be added without changing orchestration code.

## Connector contract

Every connector inherits from `platform.connectors.BaseConnector` and implements or customizes four lifecycle methods:

1. `connect()` / `Connect()` — authenticate or verify source availability.
2. `collect()` / `Collect()` — fetch raw records from the source.
3. `transform(raw_data)` / `Transform(raw_data)` — map raw records to normalized records.
4. `close()` / `Close()` — release sessions, files, or sockets.

`BaseConnector` also provides `build_normalized_record()`, deterministic `raw_fingerprint` generation, correlation-key helpers, confidence filtering, and duplicate reporting.

## Normalization rules

Normalized connector output should be a list of `NormalizedRecord` objects:

| Field | Rule |
| --- | --- |
| `record_type` | Use topology nouns such as `device`, `interface`, `link`, `route`, `subnet`, `security_policy`. |
| `source_connector` | The connector type, e.g. `netbox`, `servicenow_cmdb`, `solarwinds_ncm`. |
| `source_id` | Stable source identifier such as a CMDB `sys_id`, NetBox ID, filename, or API object ID. |
| `payload` | Vendor/source-neutral topology fields. Prefer canonical names: `hostname`, `device_type`, `interfaces`, `ip_addresses`, `site`, `neighbors`, `correlation_keys`. |
| `confidence` | `0.0` to `1.0`; lower for inferred/stale data, higher for authoritative source data. |
| `raw_fingerprint` | Produced automatically from the normalized payload. |
| `metadata` | Evidence and provenance: source URL, collection timestamp, API table, last-updated field, fixture name. |

Correlation keys should be deterministic and lower-case. Common keys are `hostname:<name>`, `ip:<address>`, `mac:<address>`, `serial:<serial>`, and `source:<native-id>`.

## Runtime registration

Built-in connectors are registered automatically. Custom connectors can register with either a decorator or a direct call:

```python
from platform.connectors import BaseConnector, connector, create_connector

@connector("custom_rest")
class CustomRestConnector(BaseConnector):
    def connect(self):
        self.mark_connected()
        return True

    def collect(self):
        return [{"id": "device-1", "hostname": "edge-01"}]

    def transform(self, raw_data):
        return [
            self.build_normalized_record(
                record_type="device",
                source_id=item["id"],
                payload={
                    "hostname": self.normalize_hostname(item["hostname"]),
                    "correlation_keys": self.build_correlation_keys(("hostname", item["hostname"])),
                },
                metadata={"source": "custom_rest"},
            )
            for item in raw_data
        ]

    def close(self):
        self.mark_disconnected()
        return True

connector = create_connector("custom_rest", "connector-guid", {"host": "https://api.example"})
```

Optional plugin modules can be imported with `discover_connectors(["my_package.graphlayer_connectors"])` before calling `create_connector()`.

## Test harness pattern

A connector test should use a mock data source or fixture, run `Connect()`, `Collect()`, `Transform()`, and verify normalized output:

```python
raw = connector.Collect()
records = connector.Transform(raw)
assert records[0].record_type == "device"
assert records[0].raw_fingerprint.startswith("sha256:")
assert "hostname:edge-01" in records[0].payload["correlation_keys"]
```

Use `deduplicate_normalized_records()` and `build_correlation_report()` when tests cover multi-source overlap.

## Backlog priority

- Tier 1: SolarWinds NCM, SSH/file-share device collection.
- Tier 2: ServiceNow CMDB, NetBox, phpIPAM.
- Tier 3: Ansible inventory, Terraform state, AWS VPC, Azure VNet, GCP.
- Tier 4: Custom REST APIs and CSV imports.
