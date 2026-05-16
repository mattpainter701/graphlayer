# NetBox Connector

Import network documentation from [NetBox](https://netbox.dev) as topology records.

## Overview

The NetBox connector fetches devices, interfaces, cables, and IPAM data via the NetBox REST API and normalizes them into GraphLayer topology records — devices, interfaces, links, subnets, and IP addresses — with confidence scores, correlation keys, and evidence provenance.

## Prerequisites

- A running NetBox instance (v3.5+ recommended) with API access
- An API token with read permissions on `dcim/*` and `ipam/*` endpoints
- Network connectivity between GraphLayer and the NetBox server

## Quick Start

```python
from platform.connectors import NetBoxConnector

connector = NetBoxConnector(
    "my-netbox",
    {
        "name": "My Network",
        "host": "https://netbox.example.com",
        "connector_type": "netbox",
        "credentials": {"api_token": "your-api-token-here"},
        "payload": {
            "verify_ssl": True,
            "page_size": 1000,
            "timeout_seconds": 30.0,
        },
    },
)

connector.Connect()
bundle = connector.Collect()
print(f"Normalized {bundle['count']} topology records")

# Inspect devices
for device in bundle["devices"]:
    print(device["payload"]["hostname"], device["payload"]["device_type"])
```

## Configuration Reference

| Parameter | Type | Default | Description |
|---|---|---|---|
| `instance_url` / `host` | string | — | NetBox base URL (e.g. `https://netbox.example.com`) |
| `api_token` | string | — | NetBox API token (from **Admin → API Tokens**) |
| `verify_ssl` | bool | `true` | Enable/disable SSL certificate verification |
| `page_size` | int | `1000` | Records per API page (max depends on NetBox config) |
| `timeout_seconds` | float | `30.0` | HTTP request timeout |
| `device_endpoint` | string | `dcim/devices/` | Custom device API endpoint path |
| `interface_endpoint` | string | `dcim/interfaces/` | Custom interface API endpoint path |
| `cable_endpoint` | string | `dcim/cables/` | Custom cable API endpoint path |
| `prefix_endpoint` | string | `ipam/prefixes/` | Custom prefix API endpoint path |
| `ip_endpoint` | string | `ipam/ip-addresses/` | Custom IP address API endpoint path |
| `site` | string | — | Filter results to a specific site slug |
| `role` | string | — | Filter results to a specific device role slug |

Credentials can be provided via the `credentials.api_token` field or as top-level `api_token`.

## Endpoint Mapping

| NetBox Endpoint | GraphLayer Record Type | Normalized Fields |
|---|---|---|
| `dcim/devices/` | `device` | hostname, device_type, vendor, model, role, site, rack, rack_position, rack_face, management_ip, serial_number, status, custom_fields |
| `dcim/interfaces/` | `interface` | device_id, device_name, name, type, enabled, description, mac_address, mtu, lag, vlans, ip_addresses |
| `dcim/cables/` | `link` | termination_a, termination_b, type, label, length |
| `ipam/prefixes/` | `subnet` | prefix, status, role, site, tenant, vlan, description |
| `ipam/ip-addresses/` | `address` | address, family, status, assigned_object, dns_name |

## Hierarchy Metadata

The connector extracts hierarchy context to enable cross-source correlation:

- **Sites** → captured as `payload.site` (slug) on devices, prefixes
- **Device roles** → captured as `payload.role` (slug) on devices
- **Racks** → captured as `payload.rack` (name), `rack_position`, `rack_face` on devices
- **Custom fields** → preserved as-is in `payload.custom_fields`

Correlation keys are built from hostname, management IP, serial number, site, rack, role, vendor, and model.

## Evidence Tracking

Every normalized record includes:

- **`netbox_id`** in metadata — the NetBox record primary key
- **`last_updated`** — timestamp from NetBox for staleness assessment
- **Confidence scores** — `0.94` (device), `0.91` (interface), `0.95` (cable), `0.88` (subnet), `0.86` (address)
- **Raw fingerprint** — SHA-256 of the normalized payload for deduplication
- **Source connector** — always `"netbox"`

## Testing

The connector ships with a self-contained mock HTTP server and fixture data:

```bash
# From the project root
pytest tests/test_netbox_connector.py -v
```

The integration test (`test_collect_normalizes_netbox_inventory_links_and_ipam`) starts a real HTTP server with paginated fixtures and verifies the full Collect → Normalize pipeline end-to-end without external dependencies.

## Using with the Registry

```python
from platform.connectors import create_connector

connector = create_connector(
    "netbox",
    "my-netbox",
    {
        "name": "Production NetBox",
        "host": "https://netbox.example.com",
        "payload": {"verify_ssl": True},
        "credentials": {"api_token": "token"},
    },
)
```

## Available via `do_download`

```python
bundle = connector.do_download(site="nyc-1", role="access-switch")
```

Optional keyword arguments `site` and `role` override the configured defaults per-collection.
