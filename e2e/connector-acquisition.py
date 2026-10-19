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
from urllib.parse import quote

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


def require_evidence(snapshot: dict, refs: list[str]) -> None:
    evidence = {item["id"]: item for item in snapshot.get("evidence", [])}
    source_lines = {line["number"]: line["text"] for line in snapshot.get("source_lines", [])}
    assert refs, "investigation returned no source evidence references"
    for ref in refs:
        item = evidence.get(ref)
        assert item, f"evidence reference does not resolve in snapshot: {ref}"
        first, last = item["start_line"], item["end_line"]
        assert first <= last and all(number in source_lines for number in range(first, last + 1))
        assert any(source_lines[number].strip() for number in range(first, last + 1))


def saved_json(opener: urllib.request.OpenerDirector, path: str) -> dict:
    with opener.open(BASE + path, timeout=10) as response:
        assert response.status == 200
        return json.load(response)


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
    for filename in ("enterprise.asa.txt", "asa_path_sample.txt", "panos_sample.txt", "fortigate_sample.txt"):
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
    ("asa_path_sample.txt", "parse_asa", "cisco-asa"),
    ("panos_sample.txt", "parse_panos", "paloalto-panos"),
    ("fortigate_sample.txt", "parse_fortigate", "fortinet-fortigate"),
]
snapshots = {}
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
    snapshots[filename] = snapshot
    assert snapshot["vendor"] == vendor
    assert snapshot["source_lines"]
    assert snapshot["evidence"]
    assert snapshot["path"]["version"]
    if filename == "asa_path_sample.txt":
        assert snapshot["path"]["evidence_refs"]

# Run a saved ACL policy investigation against the acquired enterprise ASA bytes.
enterprise = snapshots["enterprise.asa.txt"]
assert enterprise["chains"], "acquired ASA policy sample imported without ACL chains"
policy_status, policy_response = request_http(
    client, f"/api/snapshots/{quote(enterprise['id'], safe='')}/evaluate",
    {"chain_id": enterprise["chains"][0]["id"], "source": "192.0.2.42",
     "destination": "10.20.4.15", "protocol": "tcp", "source_port": 51515,
     "destination_port": 443}, session["csrf_token"],
)
assert policy_status == 200, policy_response
policy_investigation = policy_response["investigation"]
assert policy_investigation["snapshot_id"] == enterprise["id"]
assert policy_investigation["result"]["disposition"] == "permit"
require_evidence(enterprise, policy_investigation["result"].get("evidence_refs", []))
policy_saved = saved_json(client, f"/api/investigations/{quote(policy_investigation['id'], safe='')}")["investigation"]
assert policy_saved == policy_investigation
policy_export = json.dumps(policy_saved, ensure_ascii=False, indent=2).encode("utf-8")
assert json.loads(policy_export) == policy_saved

# Evaluate the acquired ASA route/NAT sample, resolve every returned source
# reference against the imported snapshot, and verify durable replay/export.
path_snapshot = snapshots["asa_path_sample.txt"]
path_status, path_response = request_http(
    client, f"/api/snapshots/{quote(path_snapshot['id'], safe='')}/path-evaluate",
    {"ingress_interface": "outside", "flow": {
        "source": "198.51.100.42", "destination": "198.51.100.15",
        "protocol": "tcp", "source_port": 52000, "destination_port": 443,
    }}, session["csrf_token"],
)
assert path_status == 200, path_response
path_investigation = path_response["investigation"]
path_result = path_investigation["result"]
assert path_investigation["snapshot_id"] == path_snapshot["id"]
assert path_result["disposition"] == "modeled_forward"
assert path_result["egress_interface"] == "dmz"
assert path_result["egress_flow"]["destination"] == "10.20.4.15"
assert path_result["selected_route"]["interface"] == "dmz"
assert path_result["nat_rule_ids"]
assert any(stage["name"] == "destination_nat" and stage["status"] == "matched"
           for stage in path_result["stages"])
refs = set(path_result.get("evidence_refs", []))
for stage in path_result.get("stages", []):
    refs.update(stage.get("evidence_refs", []))
refs.update(path_result["selected_route"].get("evidence_refs", []))
require_evidence(path_snapshot, sorted(refs))
path_saved = saved_json(client, f"/api/path-investigations/{quote(path_investigation['id'], safe='')}")["investigation"]
assert path_saved == path_investigation
path_export = json.dumps(path_saved, ensure_ascii=False, indent=2).encode("utf-8")
assert json.loads(path_export) == path_saved

# The network API composes forward and return legs, even for one acquired ASA
# node. This keeps return modeling in the Rust engine and gives each leg an
# independently snapshot-qualified evidence set.
node_id = "acquired-asa-path"
network_status, network_response = request_http(
    client, "/api/network-investigations",
    {"nodes": [{"node_id": node_id, "snapshot_id": path_snapshot["id"]}],
     "links": [],
     "source": {"node_id": node_id, "interface": "outside"},
     "destination": {"node_id": node_id, "interface": "dmz"},
     "flow": {"source": "192.0.2.42", "destination": "198.51.100.15",
              "protocol": "tcp", "source_port": 52000, "destination_port": 443}},
    session["csrf_token"],
)
assert network_status == 200, network_response
network_investigation = network_response["investigation"]
network_result = network_investigation["result"]
assert network_result["disposition"] == "modeled_round_trip"
assert network_result["forward"]["disposition"] == "modeled_forward"
assert network_result["return"]["disposition"] == "modeled_forward"
assert network_result["forward"]["hops"] and network_result["return"]["hops"]
assert network_result["return"]["final_flow"]["destination"] == "192.0.2.42"
assert any(hop["path"].get("nat_rule_ids") for hop in network_result["forward"]["hops"])
for direction in (network_result["forward"], network_result["return"]):
    qualified = direction["hops"][0]["evidence"]
    assert qualified and all(item["snapshot_id"] == path_snapshot["id"] for item in qualified)
    require_evidence(path_snapshot, [item["ref"] for item in qualified])
network_saved = saved_json(
    client, f"/api/network-investigations/{quote(network_investigation['id'], safe='')}"
)["investigation"]
assert network_saved == network_investigation
network_export = json.dumps(network_saved, ensure_ascii=False, indent=2).encode("utf-8")
assert json.loads(network_export) == network_saved

print("SFTP, SMB and persistent SSH acquisitions reached Rust parsers, saved policy/path/round-trip investigations, evidence and replay/export.")
