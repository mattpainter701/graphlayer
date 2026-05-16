from __future__ import annotations

import http.server
import json
import os
import sys
import tempfile
import threading
import unittest
from pathlib import Path
from typing import Any, Dict, List
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from platform.connectors.netbox_connector import NetBoxConnector, test_connection


FIXTURE_DIR = Path(__file__).resolve().parent / "fixtures" / "netbox"


def _load_fixture(name: str) -> Dict[str, Any]:
    with open(FIXTURE_DIR / name, "r", encoding="utf-8") as handle:
        return json.load(handle)


class NetBoxFixtureHandler(http.server.BaseHTTPRequestHandler):
    fixtures: Dict[str, Dict[str, Any]] = {}
    request_log: List[str] = []

    def log_message(self, format: str, *args: Any) -> None:  # pragma: no cover - keep test output quiet
        return

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query)
        NetBoxFixtureHandler.request_log.append(self.path)

        if parsed.path == "/api/":
            payload = self._payload_for("root.json")
        elif parsed.path.endswith("/dcim/devices/"):
            payload = self._page_payload(
                base_name="dcim_devices_page",
                offset=int(query.get("offset", ["0"])[0]),
                page_count=2,
            )
        elif parsed.path.endswith("/dcim/interfaces/"):
            payload = self._page_payload(
                base_name="dcim_interfaces_page",
                offset=int(query.get("offset", ["0"])[0]),
                page_count=2,
            )
        elif parsed.path.endswith("/dcim/cables/"):
            payload = self._page_payload(
                base_name="dcim_cables_page",
                offset=int(query.get("offset", ["0"])[0]),
                page_count=1,
            )
        elif parsed.path.endswith("/ipam/prefixes/"):
            payload = self._page_payload(
                base_name="ipam_prefixes_page",
                offset=int(query.get("offset", ["0"])[0]),
                page_count=1,
            )
        elif parsed.path.endswith("/ipam/ip-addresses/"):
            payload = self._page_payload(
                base_name="ipam_ip_addresses_page",
                offset=int(query.get("offset", ["0"])[0]),
                page_count=2,
            )
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"not found")
            return

        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _payload_for(self, filename: str) -> Dict[str, Any]:
        return NetBoxFixtureHandler.fixtures[filename]

    def _page_payload(self, base_name: str, offset: int, page_count: int) -> Dict[str, Any]:
        if offset >= page_count:
            return {"results": []}
        filename = f"{base_name}{offset + 1}.json"
        return NetBoxFixtureHandler.fixtures.get(filename, {"results": []})


class NetBoxConnectorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        NetBoxFixtureHandler.fixtures = {
            name: _load_fixture(name)
            for name in [
                "root.json",
                "dcim_devices_page1.json",
                "dcim_devices_page2.json",
                "dcim_interfaces_page1.json",
                "dcim_interfaces_page2.json",
                "dcim_cables_page1.json",
                "ipam_prefixes_page1.json",
                "ipam_ip_addresses_page1.json",
                "ipam_ip_addresses_page2.json",
            ]
        }

    def setUp(self) -> None:
        NetBoxFixtureHandler.request_log = []
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), NetBoxFixtureHandler)
        self.server_thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.server_thread.start()
        self.addCleanup(self.server.shutdown)
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server_thread.join, timeout=2)

    def test_collect_normalizes_netbox_inventory_links_and_ipam(self) -> None:
        host, port = self.server.server_address
        with tempfile.TemporaryDirectory() as tmpdir:
            connector = NetBoxConnector(
                "netbox-guid",
                {
                    "name": "NetBox",
                    "host": f"http://{host}:{port}",
                    "connector_type": "netbox",
                    "credentials": {"api_token": "token-123"},
                    "payload": {
                        "page_size": 1,
                        "verify_ssl": False,
                    },
                },
                working_folder=tmpdir,
            )

            self.assertTrue(connector.Connect())
            self.assertTrue(connector.connected)

            bundle = connector.Collect()
            self.assertEqual(bundle["status"], "completed")
            self.assertEqual(bundle["record_type"], "topology")
            self.assertEqual(bundle["count"], 8)
            self.assertEqual(len(bundle["devices"]), 2)
            self.assertEqual(len(bundle["interfaces"]), 2)
            self.assertEqual(len(bundle["links"]), 1)
            self.assertEqual(len(bundle["subnets"]), 1)
            self.assertEqual(len(bundle["addresses"]), 2)
            self.assertTrue(bundle["sync"]["last_sync"].startswith("2024-03-02T16:30:00"))

            device = next(record for record in bundle["devices"] if record["payload"]["id"] == "101")
            self.assertEqual(device["payload"]["hostname"], "core-sw-01")
            self.assertEqual(device["payload"]["vendor"], "Cisco")
            self.assertEqual(device["payload"]["model"], "C9300-48P")
            self.assertEqual(device["payload"]["device_type"], "switch")
            self.assertEqual(device["payload"]["site"], "nyc-1")
            self.assertIn("hostname:core-sw-01", device["payload"]["correlation_keys"])
            self.assertIn("management_ip:10.0.0.2/24", device["payload"]["correlation_keys"])
            self.assertIn("serial:fcw1234a1b2", device["payload"]["correlation_keys"])

            interface = next(record for record in bundle["interfaces"] if record["payload"]["id"] == "1001")
            self.assertEqual(interface["payload"]["device_name"], "core-sw-01")
            self.assertEqual(interface["payload"]["name"], "GigabitEthernet1/0/1")
            self.assertEqual(interface["payload"]["ip_addresses"][0]["address"], "10.0.0.2/24")

            link = bundle["links"][0]
            self.assertEqual(link["payload"]["termination_a"]["device_name"], "core-sw-01")
            self.assertEqual(link["payload"]["termination_b"]["device_name"], "edge-router-01")
            self.assertEqual(link["payload"]["type"], "cat6")
            self.assertIn("a_device:core-sw-01", link["payload"]["correlation_keys"])
            self.assertIn("b_device:edge-router-01", link["payload"]["correlation_keys"])

            subnet = bundle["subnets"][0]
            self.assertEqual(subnet["payload"]["prefix"], "10.0.0.0/24")
            self.assertEqual(subnet["payload"]["site"], "nyc-1")

            self.assertTrue(
                any("/api/dcim/devices/?limit=1&offset=0" in request for request in NetBoxFixtureHandler.request_log)
            )
            self.assertTrue(
                any("/api/ipam/ip-addresses/?limit=1&offset=1" in request for request in NetBoxFixtureHandler.request_log)
            )

    def test_transform_accepts_recorded_raw_bundle(self) -> None:
        connector = NetBoxConnector(
            "netbox-guid",
            {
                "name": "NetBox",
                "host": "http://example.invalid",
                "connector_type": "netbox",
                "credentials": {"api_token": "token-123"},
                "payload": {"verify_ssl": False},
            },
            working_folder=tempfile.mkdtemp(),
        )

        raw_bundle = {
            "devices": [
                _load_fixture("dcim_devices_page1.json")["results"][0],
                _load_fixture("dcim_devices_page2.json")["results"][0],
            ],
            "interfaces": [
                _load_fixture("dcim_interfaces_page1.json")["results"][0],
                _load_fixture("dcim_interfaces_page2.json")["results"][0],
            ],
            "cables": _load_fixture("dcim_cables_page1.json")["results"],
            "prefixes": _load_fixture("ipam_prefixes_page1.json")["results"],
            "ip_addresses": _load_fixture("ipam_ip_addresses_page1.json")["results"]
            + _load_fixture("ipam_ip_addresses_page2.json")["results"],
        }

        bundle = connector.transform(raw_bundle)
        self.assertEqual(bundle["count"], 8)
        self.assertEqual(bundle["devices"][0]["payload"]["device_type"], "switch")
        self.assertEqual(bundle["interfaces"][0]["payload"]["ip_addresses"][0]["address"], "10.0.0.2/24")
        self.assertEqual(bundle["links"][0]["payload"]["termination_b"]["device_name"], "edge-router-01")
        self.assertTrue(bundle["records"][0]["raw_fingerprint"].startswith("sha256:"))

    def test_smoke_connection_helper(self) -> None:
        host, port = self.server.server_address
        result = test_connection(f"http://{host}:{port}", "token-123")
        self.assertEqual(result["status"], "success")
        self.assertTrue(result["connected"])


if __name__ == "__main__":
    unittest.main()
