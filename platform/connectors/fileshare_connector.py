"""Compatibility facade for Rust SMB/SFTP configuration-share collection."""

from graphlayer_utils import journal as logger

from ..rust_worker import call_rust
from .base_connector import BaseConnector


class FileShareConnector(BaseConnector):
    SUPPORTS_DEVICES = False

    def __init__(self, guid, connector_data, working_folder=None, task_guid=None):
        super().__init__(guid, connector_data, working_folder, task_guid)
        payload = connector_data.get("payload", {})
        location = payload.get("location", {})
        filtering = payload.get("filtering", {})
        self.protocol = payload.get("protocol", "auto")
        self.share_name = location.get("share_name", "")
        self.device_name = location.get("device_name", self.host)
        self.remote_path = self._normalize_path(location.get("remote_path", "/"))
        self.recursive = payload.get("recursive", True)
        self.file_patterns = list(filtering.get("file_patterns", [r".*\.cfg$", r".*\.conf$", r".*\.txt$"]))
        self.exclude_patterns = list(filtering.get("exclude_patterns", []))
        self._connected = False

    def connect(self):
        result = call_rust("fileshare.connect", {
            "connector_data": self.connector_data,
            "working_folder": str(self.working_folder),
        })
        self._connected = bool(result.get("connected"))
        return self._connected

    def disconnect(self):
        if self._connected:
            call_rust("fileshare.disconnect", {"connector_data": self.connector_data})
        self._connected = False

    def do_download(self, **kwargs):
        if not self._connected:
            raise RuntimeError("Not connected to file share. Call connect() first.")
        return call_rust("fileshare.collect", {
            "connector_data": self.connector_data,
            "working_folder": str(self.working_folder),
        })

    def _normalize_path(self, path):
        return call_rust("fileshare.normalize_path", {"path": str(path or "")})

    def _matches_patterns(self, filename):
        return call_rust("fileshare.matches_patterns", {
            "filename": filename,
            "file_patterns": self.file_patterns,
            "exclude_patterns": self.exclude_patterns,
        })

    def _create_local_filename(self, remote_path):
        return call_rust("fileshare.create_local_filename", {
            "remote_path": remote_path,
            "root_path": self.remote_path,
        })


def test_connection(protocol, host, username, password, **kwargs):
    return call_rust("fileshare.ping", {
        "connector_data": {
            "host": host, "username": username, "password": password,
            "payload": {"protocol": protocol, **kwargs},
        },
    })
