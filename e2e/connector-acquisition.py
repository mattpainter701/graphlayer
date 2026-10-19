"""Hosted connector acquisition and import smoke test.

Requires the OpenSSH, Samba and application services provisioned by the hosted
connector CI job. This script intentionally has no skip path: missing services
or worker configuration fail the job.
"""

from __future__ import annotations

import http.cookiejar
import base64
import json
import os
from pathlib import Path
import queue
import subprocess
import sys
import threading
import tempfile
import time
import urllib.error
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from platform.connectors.fileshare_connector import FileShareConnector
from platform.rust_worker import RustWorkerError, call_rust


BASE = "http://127.0.0.1:8091"
HOST = os.environ.get("GRAPHLAYER_TEST_SSH_HOST", "127.0.0.1")
SSH_PORT = int(os.environ.get("SSH_PORT", "2222"))
SMB_HOST = os.environ.get("SMB_HOST", "127.0.0.1")
USERNAME = os.environ["GRAPHLAYER_TEST_USERNAME"]
PASSWORD = os.environ["GRAPHLAYER_TEST_PASSWORD"]
KNOWN_HOSTS = Path(os.environ["GRAPHLAYER_TEST_KNOWN_HOSTS"]).resolve(strict=True)
ROOT = Path(__file__).resolve().parents[1]


def read_http(opener: urllib.request.OpenerDirector, path: str) -> dict:
    with opener.open(BASE + path, timeout=5) as response:
        return json.load(response)


def request_http(opener: urllib.request.OpenerDirector, path: str, value: dict,
                 csrf: str) -> tuple[int, dict]:
    request = urllib.request.Request(
        BASE + path,
        data=json.dumps(value).encode("utf-8"),
        headers={"Content-Type": "application/json", "Origin": BASE,
                 "X-CSRF-Token": csrf},
        method="POST",
    )
    try:
        with opener.open(request, timeout=20) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


class RustSession:
    def __init__(self) -> None:
        binary = Path(os.environ["GRAPHLAYER_ENGINE"]).resolve(strict=True)
        self.process = subprocess.Popen(
            [str(binary), "--session"], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        )

    def call(self, operation: str, data: dict) -> object:
        assert self.process.stdin and self.process.stdout
        frame = {"protocol_version": "graphlayer-engine-v1", "operation": operation,
                 "input": data}
        try:
            self.process.stdin.write(json.dumps(frame).encode("utf-8") + b"\n")
            self.process.stdin.flush()
        except (BrokenPipeError, OSError):
            raise AssertionError("Rust session worker exited before responding") from None
        result: queue.Queue[bytes] = queue.Queue(maxsize=1)
        threading.Thread(target=lambda: result.put(self.process.stdout.readline()),
                         daemon=True).start()
        try:
            line = result.get(timeout=30)
        except queue.Empty:
            self.process.kill()
            self.process.wait(timeout=5)
            raise AssertionError(f"Rust session operation timed out: {operation}") from None
        assert line, "Rust session worker closed stdout before responding"
        response = json.loads(line)
        assert response["protocol_version"] == "graphlayer-engine-v1"
        assert response["operation"] == operation
        if response.get("error"):
            raise RustWorkerError("Rust connector operation failed")
        return response["result"]

    def close(self) -> None:
        if self.process.poll() is not None:
            return
        if self.process.stdin:
            self.process.stdin.close()
        assert self.process.wait(timeout=10) == 0


def connector_data(protocol: str, *, smb: bool = False, password: str = PASSWORD,
                   known_hosts: str = str(KNOWN_HOSTS)) -> dict:
    location = {"remote_path": "/", "port": 445, "share_name": "configs"} if smb else {
        "remote_path": "/srv/configs", "port": SSH_PORT,
        "known_hosts_path": known_hosts,
    }
    return {
        "name": "Hosted connector fixture", "connector_type": "fileshare",
        "host": SMB_HOST if smb else HOST,
        "username": USERNAME, "password": password,
        "credentials": {"username": USERNAME, "password": password},
        "payload": {"protocol": protocol, "location": location, "recursive": True,
                    "filtering": {"file_patterns": [r".*\.(?:txt|cfg)$"]}},
    }


def acquire(protocol: str, *, smb: bool = False) -> Path:
    folder = Path(tempfile.mkdtemp(prefix=f"graphlayer-{protocol}-"))
    connector = FileShareConnector(f"hosted-{protocol}", connector_data(protocol, smb=smb),
                                   working_folder=str(folder))
    assert connector.connect()
    result = connector.do_download()
    connector.disconnect()
    assert result["status"] == "completed"
    assert result["files_downloaded"] >= 3
    for filename in ("enterprise.asa.txt", "panos_sample.txt", "fortigate_sample.txt"):
        assert (folder / filename).is_file(), f"{protocol} missed {filename}"
        expected = (ROOT / "platform" / "firewall" / "testdata" / filename).read_bytes()
        assert (folder / filename).read_bytes() == expected, f"{protocol} changed {filename} bytes"
    for path in folder.rglob("*"):
        assert folder.resolve() in path.resolve().parents or path.resolve() == folder.resolve()
    return folder


deadline = time.monotonic() + 30
client = urllib.request.build_opener(
    urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
)
while True:
    try:
        session = read_http(client, "/api/session")
        break
    except (OSError, urllib.error.URLError):
        if time.monotonic() >= deadline:
            raise RuntimeError("Packaged application did not become ready") from None
        time.sleep(0.2)

sftp_folder = acquire("ssh")
smb_folder = acquire("smb", smb=True)

# Authentication and strict host-key validation fail closed.
try:
    call_rust("fileshare.ping", {"connector_data": connector_data("ssh", password="wrong-password")})
except RustWorkerError:
    pass
else:
    raise AssertionError("SFTP accepted an invalid password")
with tempfile.NamedTemporaryFile("w", encoding="utf-8") as bad_keys:
    bad_keys.write("127.0.0.1 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKeyForRejection\n")
    bad_keys.flush()
    try:
        call_rust("fileshare.ping", {"connector_data": connector_data("ssh", known_hosts=bad_keys.name)})
    except RustWorkerError:
        pass
    else:
        raise AssertionError("SFTP accepted an untrusted host key")
try:
    call_rust("fileshare.ping", {"connector_data": connector_data("smb", smb=True, password="wrong-password")})
except RustWorkerError:
    pass
else:
    raise AssertionError("SMB accepted an invalid password")

# Two commands use one persistent Rust SSH session and one authenticated connection.
ssh = RustSession()
ssh_id = "hosted-connector-session"
ssh_config = {"host": HOST, "port": SSH_PORT, "username": USERNAME,
              "password": PASSWORD, "known_hosts_path": str(KNOWN_HOSTS),
              "timeout_seconds": 15}
assert ssh.call("ssh.connect", {"session_id": ssh_id, "config": ssh_config})["connected"]
version = ssh.call("ssh.execute", {"session_id": ssh_id, "command": "show version"})
running = ssh.call("ssh.execute", {"session_id": ssh_id, "command": "show running-config"})
assert "fixture" in version["stdout"].lower()
assert "hostname" in running["stdout"].lower()
ssh.call("ssh.disconnect", {"session_id": ssh_id})
ssh.close()

# Rust parsers inspect the exact acquired bytes before the same files enter the app.
samples = [
    ("enterprise.asa.txt", "parse_asa", "cisco-asa"),
    ("panos_sample.txt", "parse_panos", "paloalto-panos"),
    ("fortigate_sample.txt", "parse_fortigate", "fortinet-fortigate"),
]
for filename, operation, vendor in samples:
    source = (sftp_folder / filename).read_text(encoding="utf-8")
    parsed = call_rust(operation, {"config_base64": base64.b64encode(source.encode("utf-8")).decode("ascii")})
    assert parsed["vendor"] == vendor
    assert parsed["evidence"], f"Rust parser produced no source evidence for {filename}"
    status, imported = request_http(client, "/api/imports",
                                    {"name": filename, "vendor": vendor, "config": source},
                                    session["csrf_token"])
    assert status == 201, imported
    snapshot = imported["snapshot"]
    assert snapshot["vendor"] == vendor
    assert snapshot["source_lines"]
    assert snapshot["evidence"]
    assert snapshot["path"]["version"]
    assert snapshot["path"]["evidence_refs"]

print("SFTP, SMB and persistent SSH acquired fixture configs; Rust parsers and the live app retained source evidence.")
