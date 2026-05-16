# NetBox Connector — Config Examples

## Minimal Config

```json
{
  "connector_type": "netbox",
  "host": "https://netbox.example.com",
  "credentials": {
    "api_token": "0123456789abcdef0123456789abcdef01234567"
  },
  "payload": {}
}
```

## Production Config with All Options

```json
{
  "connector_type": "netbox",
  "host": "https://netbox.corp.example",
  "name": "Corp NetBox Production",
  "credentials": {
    "api_token": "0123456789abcdef0123456789abcdef01234567"
  },
  "payload": {
    "verify_ssl": true,
    "page_size": 500,
    "timeout_seconds": 60.0,
    "site": "nyc-1",
    "role": "access-switch"
  }
}
```

## Python Script — Full Pipeline

```python
#!/usr/bin/env python3
"""NetBox → GraphLayer topology export example."""

import json
import sys

sys.path.insert(0, ".")

from platform.connectors import NetBoxConnector, create_connector


def export_netbox_topology(host: str, token: str, site: str | None = None) -> dict:
    """Connect to NetBox and export normalized topology."""
    connector = NetBoxConnector(
        "netbox-export",
        {
            "name": "NetBox Export",
            "host": host,
            "connector_type": "netbox",
            "credentials": {"api_token": token},
            "payload": {
                "verify_ssl": True,
                "site": site,
            },
        },
    )

    connector.Connect()
    try:
        bundle = connector.Collect()
        print(f"Collected {bundle['count']} records:")
        print(f"  Devices:    {len(bundle['devices'])}")
        print(f"  Interfaces: {len(bundle['interfaces'])}")
        print(f"  Links:      {len(bundle['links'])}")
        print(f"  Subnets:    {len(bundle['subnets'])}")
        print(f"  Addresses:  {len(bundle['addresses'])}")
        return bundle
    finally:
        connector.Close()


if __name__ == "__main__":
    bundle = export_netbox_topology(
        host="https://netbox.example.com",
        token="your-api-token",
        site="nyc-1",
    )
    # Save output
    with open("netbox_topology.json", "w") as f:
        json.dump(bundle, f, indent=2, default=str)
```

## Registry Pattern

```python
from platform.connectors import create_connector, list_connectors

# Verify netbox is registered
print("Available connectors:", list_connectors())

# Create via registry
connector = create_connector(
    "netbox",
    "netbox-prod",
    {
        "host": "https://netbox.example.com",
        "payload": {"verify_ssl": True},
        "credentials": {"api_token": "token-here"},
    },
)
```

## Filtering by Site and Role

```python
# At construction time (persistent filter):
connector = NetBoxConnector("nb", connector_data={
    "host": "https://netbox.example.com",
    "credentials": {"api_token": "token"},
    "payload": {
        "site": "nyc-1",
        "role": "access-switch",
    },
})

# Or per-collection (overrides the configured filter):
bundle = connector.do_download(site="lax-1", role="edge-router")
```

## Expected Output Shape

```json
{
  "status": "completed",
  "connector": "netbox",
  "record_type": "topology",
  "count": 42,
  "devices": [
    {
      "record_type": "device",
      "source_connector": "netbox",
      "source_id": "101",
      "confidence": 0.94,
      "payload": {
        "id": "101",
        "hostname": "core-sw-01",
        "device_type": "switch",
        "vendor": "Cisco",
        "model": "C9300-48P",
        "role": "access-switch",
        "site": "nyc-1",
        "rack": "Rack-A01",
        "rack_position": 12,
        "rack_face": "front",
        "management_ip": "10.0.0.2/24",
        "serial_number": "FCW1234A1B2",
        "status": "active",
        "last_updated": "2024-03-02T16:20:00Z",
        "custom_fields": {"rack_u": 12},
        "correlation_keys": [
          "hostname:core-sw-01",
          "management_ip:10.0.0.2/24",
          "serial:fcw1234a1b2",
          "site:nyc-1",
          "rack:rack-a01",
          "role:access-switch",
          "vendor:cisco",
          "model:c9300-48p"
        ]
      },
      "metadata": {
        "netbox_id": "101",
        "last_updated": "2024-03-02T16:20:00Z",
        "endpoint": "dcim/devices",
        "connector": "netbox"
      }
    }
  ],
  "interfaces": [...],
  "links": [...],
  "subnets": [...],
  "addresses": [...],
  "records": [...],
  "sync": {
    "last_sync": "2024-03-02T16:30:00+00:00",
    "source": "netbox"
  }
}
```
