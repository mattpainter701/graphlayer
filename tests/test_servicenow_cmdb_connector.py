from __future__ import annotations

import http.server
import json
import os
import socket
import sys
import tempfile
import threading
import unittest
from pathlib import Path
from typing import Any, Dict, List
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from platform.connectors.servicenow_cmdb_connector import ServiceNowCMDBConnector


FIXTURE_DIR = Path(__file__).resolve().parent / "fixtures" / "servicenow"


def _load_fixture(name: str) -> Dict[str, Any]:
    with open(FIXTURE_DIR / name, "r", encoding="utf-8") as handle:
        return json.load(handle)


class ServiceNowFixtureHandler(http.server.BaseHTTPRequestHandler):
    fixtures: Dict[str, Dict[str, Any]] = {}
    request_log: List[str] = []

    def log_message(self, format: str, *args: Any) -> None:  # pragma: no cover - keep test output quiet
        return

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler API
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query)
        ServiceNowFixtureHandler.request_log.append(self.path)

        if parsed.path.endswith("/cmdb_ci"):
            payload = self._payload_for("cmdb_ci_ping.json")
        elif parsed.path.endswith("/cmdb_ci_server"):
            payload = self._page_payload(
                base_name="cmdb_ci_server_page",
                offset=int(query.get("sysparm_offset", ["0"])[0]),
                page_count=2,
            )
        elif parsed.path.endswith("/cmdb_ci_netgear"):
            payload = self._page_payload(
                base_name="cmdb_ci_netgear_page",
                offset=int(query.get("sysparm_offset", ["0"])[0]),
                page_count=1,
            )
        elif parsed.path.endswith("/cmdb_rel_ci"):
            payload = self._page_payload(
                base_name="cmdb_rel_ci_page",
                offset=int(query.get("sysparm_offset", ["0"])[0]),
                page_count=1,
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
        return ServiceNowFixtureHandler.fixtures[filename]

    def _page_payload(self, base_name: str, offset: int, page_count: int) -> Dict[str, Any]:
        if offset >= page_count:
            return {"result": []}
        filename = f"{base_name}{offset + 1}.json"
        return ServiceNowFixtureHandler.fixtures.get(filename, {"result": []})


class ServiceNowCMDBConnectorTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        ServiceNowFixtureHandler.fixtures = {
            name: _load_fixture(name)
            for name in [
                "cmdb_ci_ping.json",
                "cmdb_ci_server_page1.json",
                "cmdb_ci_server_page2.json",
                "cmdb_ci_netgear_page1.json",
                "cmdb_rel_ci_page1.json",
            ]
        }

    def setUp(self) -> None:
        ServiceNowFixtureHandler.request_log = []
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ServiceNowFixtureHandler)
        self.server_thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.server_thread.start()
        self.addCleanup(self.server.shutdown)
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server_thread.join, timeout=2)

    def test_collect_normalizes_service_now_cmdb_inventory_and_links(self) -> None:
        host, port = self.server.server_address
        with tempfile.TemporaryDirectory() as tmpdir:
            connector = ServiceNowCMDBConnector(
                "servicenow-guid",
                {
                    "name": "ServiceNow CMDB",
                    "host": f"http://{host}:{port}",
                    "connector_type": "servicenow_cmdb",
                    "username": "admin",
                    "password": "secret",
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
            self.assertEqual(bundle["count"], 4)
            self.assertEqual(len(bundle["devices"]), 3)
            self.assertEqual(len(bundle["relationships"]), 1)
            self.assertTrue(bundle["sync"]["last_sync"].startswith("2024-01-04T11:00:00"))

            device_types = {record["payload"]["id"]: record["payload"]["device_type"] for record in bundle["devices"]}
            self.assertEqual(device_types["srv-001"], "server")
            self.assertEqual(device_types["sw-001"], "switch")

            server_record = next(record for record in bundle["devices"] if record["payload"]["id"] == "srv-001")
            self.assertEqual(server_record["metadata"]["sys_id"], "srv-001")
            self.assertEqual(server_record["metadata"]["source_table"], "cmdb_ci_server")
            self.assertEqual(server_record["payload"]["vendor"], "Dell")
            self.assertEqual(server_record["payload"]["model"], "PowerEdge R740")
            self.assertIn("hostname:app-server-01", server_record["payload"]["correlation_keys"])
            self.assertIn("ip_address:10.10.1.11", server_record["payload"]["correlation_keys"])
            self.assertIn("serial:sn-001", server_record["payload"]["correlation_keys"])

            relationship = bundle["relationships"][0]
            self.assertEqual(relationship["payload"]["source_id"], "sw-001")
            self.assertEqual(relationship["payload"]["target_id"], "srv-001")
            self.assertEqual(relationship["payload"]["relationship_type"], "Runs on::Runs")
            self.assertEqual(relationship["metadata"]["source_table"], "cmdb_rel_ci")
            self.assertIn("source_id:sw-001", relationship["payload"]["correlation_keys"])
            self.assertIn("target_id:srv-001", relationship["payload"]["correlation_keys"])

            state_path = Path(tmpdir) / "servicenow_cmdb_state.json"
            with open(state_path, "r", encoding="utf-8") as handle:
                state = json.load(handle)
            self.assertEqual(state["last_sync"], bundle["sync"]["last_sync"])

            ServiceNowFixtureHandler.request_log = []
            second_bundle = connector.Collect()
            self.assertEqual(second_bundle["count"], 4)
            self.assertTrue(
                any(
                    "sysparm_query=sys_updated_on%3E%3D2024-01-04T11%3A00%3A00%2B00%3A00" in request
                    for request in ServiceNowFixtureHandler.request_log
                ),
                msg=f"expected incremental sync query in {ServiceNowFixtureHandler.request_log}",
            )

    def test_transform_accepts_recorded_raw_bundle(self) -> None:
        connector = ServiceNowCMDBConnector(
            "servicenow-guid",
            {
                "name": "ServiceNow CMDB",
                "host": "http://example.invalid",
                "connector_type": "servicenow_cmdb",
                "username": "admin",
                "password": "secret",
                "payload": {"verify_ssl": False},
            },
            working_folder=tempfile.mkdtemp(),
        )

        raw_bundle = {
            "updated_since": "2024-01-01T00:00:00+00:00",
            "devices": [
                _load_fixture("cmdb_ci_server_page1.json")["result"][0],
                _load_fixture("cmdb_ci_netgear_page1.json")["result"][0],
            ],
            "relationships": [_load_fixture("cmdb_rel_ci_page1.json")["result"][0]],
        }

        bundle = connector.transform(raw_bundle)
        self.assertEqual(bundle["count"], 3)
        self.assertEqual(bundle["devices"][0]["payload"]["device_type"], "server")
        self.assertEqual(bundle["devices"][1]["payload"]["device_type"], "switch")
        self.assertEqual(bundle["relationships"][0]["payload"]["relationship_type"], "Runs on::Runs")
        self.assertTrue(bundle["records"][0]["raw_fingerprint"].startswith("sha256:"))


if __name__ == "__main__":
    unittest.main()
