from __future__ import annotations

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from platform.connectors import NetBoxConnector
from platform.connectors.base_connector import BaseConnector
from platform.connectors.fileshare_connector import FileShareConnector
from platform.connectors.solarwinds_ncm_connector import (
    SolarWindsNCMConnector,
    test_connection as solarwinds_test_connection,
)


class DummyConnector(BaseConnector):
    def connect(self, *args, **kwargs):
        self.mark_connected()
        return True

    def disconnect(self, *args, **kwargs):
        self.mark_disconnected()
        return True

    def do_download(self, **kwargs):
        return {"status": "ok", "kwargs": kwargs}


class ConnectorContractTests(unittest.TestCase):
    def test_base_connector_contract_aliases(self):
        connector = DummyConnector(
            "dummy-guid",
            {
                "name": "Dummy Connector",
                "connector_type": "dummy",
                "host": "example.local",
                "payload": {"host": "example.local"},
            },
        )

        self.assertEqual(connector.state, "disconnected")
        self.assertFalse(connector.connected)

        self.assertTrue(connector.Connect())
        self.assertTrue(connector.connected)
        self.assertEqual(connector.state, "connected")

        collected = connector.Collect(sample=True)
        self.assertEqual(collected["status"], "ok")
        self.assertEqual(collected["kwargs"], {"sample": True})

        normalized = connector.build_normalized_record(
            record_type="device",
            source_id="device-1",
            payload={"hostname": "edge-01", "ip": "10.0.0.1"},
            metadata={"source": "unit-test"},
        )
        self.assertEqual(normalized.record_type, "device")
        self.assertEqual(normalized.source_connector, "dummy")
        self.assertTrue(normalized.raw_fingerprint.startswith("sha256:"))

        self.assertTrue(connector.Close())
        self.assertFalse(connector.connected)
        self.assertEqual(connector.state, "disconnected")

    def test_file_share_normalization_helpers(self):
        connector = FileShareConnector(
            "fileshare-guid",
            {
                "name": "File Share",
                "host": "files.example.local",
                "connector_type": "fileshare",
                "payload": {
                    "location": {
                        "remote_path": "/backups/network/",
                    },
                    "filtering": {
                        "file_patterns": [r".*\.cfg$", r".*running-config.*"],
                        "exclude_patterns": [r".*\.tmp$"]
                    },
                },
            },
        )

        self.assertEqual(connector._normalize_path("/foo/bar/"), "/foo/bar")
        self.assertEqual(connector._normalize_path("foo/bar"), "/foo/bar")
        self.assertTrue(connector._matches_patterns("switch.cfg"))
        self.assertFalse(connector._matches_patterns("switch.tmp"))
        self.assertEqual(
            connector._create_local_filename("/backups/network/vendor/a.cfg"),
            "vendor/a.cfg",
        )

        connector.do_download = lambda **kwargs: {"files_downloaded": 2, "kwargs": kwargs}
        result = connector.Collect(recursive=False)
        self.assertEqual(result["files_downloaded"], 2)
        self.assertEqual(result["kwargs"], {"recursive": False})

    def test_solarwinds_module_import_and_sanitization(self):
        connector = SolarWindsNCMConnector(
            "solarwinds-guid",
            {
                "name": "SolarWinds",
                "host": "swis.example.local",
                "connector_type": "solarwinds_ncm",
                "payload": {},
            },
        )
        self.assertEqual(connector._sanitize_filename("Core Switch/01"), "Core_Switch01")

        # The legacy test helper should fail cleanly if the external SDK is unavailable.
        result = solarwinds_test_connection(
            "swis.example.local",
            "user",
            "pass",
        )
        self.assertIn(result["status"], {"success", "error"})

    def test_netbox_package_export(self):
        connector = NetBoxConnector(
            "netbox-guid",
            {
                "name": "NetBox",
                "host": "netbox.example.local",
                "connector_type": "netbox",
                "credentials": {"api_token": "token-123"},
                "payload": {"verify_ssl": False},
            },
        )
        self.assertEqual(connector.connector_type, "netbox")


if __name__ == "__main__":
    unittest.main()
