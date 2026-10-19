"""Compatibility facade for the Rust SolarWinds SWIS connector."""

from graphlayer_utils import journal as logger

from .base_connector import BaseConnector
from ..rust_worker import call_rust


class SolarWindsNCMConnector(BaseConnector):
    SUPPORTS_DEVICES = True

    def __init__(self, guid, connector_data, working_folder=None, task_guid=None):
        super().__init__(guid, connector_data, working_folder, task_guid)
        payload = connector_data.get("payload", {})
        self.config_types = payload.get("config_types", ["Running"])
        self.use_https = payload.get("use_https", True)
        self.verify_ssl = payload.get("verify_ssl", True)
        self.download_all = len(self.devices) == 0
        self._connected = False

    def _request(self, operation, **values):
        return call_rust(operation, {"connector_data": self.connector_data, **values})

    def connect(self):
        result = self._request("solarwinds.ping")
        self._connected = result.get("status") == "success"
        if not self._connected:
            raise ConnectionError(result.get("message", "SolarWinds SWIS connection failed"))
        return True

    def disconnect(self):
        self._connected = False

    def do_download(self, **kwargs):
        if not self._connected:
            raise RuntimeError("Not connected to SolarWinds. Call connect() first.")
        return call_rust("solarwinds.collect", {
            "connector_data": self.connector_data,
            "working_folder": str(self.working_folder),
        })

    def _get_device_list(self):
        return self._request("solarwinds.devices").get("results", [])

    def _download_device_config(self, device, config_type):
        return self._request("solarwinds.config", device=device, config_type=config_type).get("config")

    def _save_config_file(self, device, config_type, config_data):
        return self._request(
            "solarwinds.save_config", device=device,
            config_type=config_type, config=config_data,
            working_folder=str(self.working_folder),
        )

    def _sanitize_filename(self, name):
        return call_rust("solarwinds.sanitize_filename", {"name": name})

    def trigger_ncm_download(self, node_ids, config_type="Running"):
        return self._request("solarwinds.trigger", node_ids=node_ids, config_type=config_type)


def test_connection(host, username, password, use_https=True, verify_ssl=False):
    result = call_rust("solarwinds.ping", {
        "connector_data": {
            "host": host, "username": username, "password": password,
            "payload": {"use_https": use_https, "verify_ssl": verify_ssl},
        },
    })
    return result
