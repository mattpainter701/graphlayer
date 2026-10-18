"""Hosted-only packaging check: import through the real container binaries."""

import http.cookiejar
import json
from pathlib import Path
import time
import urllib.error
import urllib.request


base = "http://127.0.0.1:8091"
client = urllib.request.build_opener(
    urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())
)
deadline = time.monotonic() + 30
while True:
    try:
        with client.open(base + "/api/session", timeout=2) as response:
            session = json.load(response)
        break
    except (OSError, urllib.error.URLError):
        if time.monotonic() >= deadline:
            raise RuntimeError("Packaged application did not become ready") from None
        time.sleep(0.2)

config = Path("platform/firewall/testdata/asa_path_sample.txt").read_text()
request = urllib.request.Request(
    base + "/api/imports",
    data=json.dumps({"name": "container-sample.asa.txt", "vendor": "cisco-asa", "config": config}).encode(),
    headers={"Content-Type": "application/json", "Origin": base, "X-CSRF-Token": session["csrf_token"]},
    method="POST",
)
with client.open(request, timeout=15) as response:
    assert response.status == 201
    snapshot = json.load(response)["snapshot"]
assert snapshot["vendor"] == "cisco-asa"
assert snapshot["path"]["version"] == "asa-path-v1"
assert len(snapshot["path"]["interfaces"]) == 3
assert len(snapshot["path"]["nat_rules"]) == 2
assert snapshot["path"]["routing_complete"] and snapshot["path"]["nat_complete"]
assert snapshot["path"]["evidence_refs"]
print("Packaged application imported ASA through its required Rust engine.")
