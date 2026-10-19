"""Package hosted release outputs without regenerating expected behavior."""
import argparse
import collections
import gzip
import hashlib
import json
from pathlib import Path

parser = argparse.ArgumentParser()
parser.add_argument("source", type=Path)
parser.add_argument("--output", type=Path, required=True)
args = parser.parse_args()
args.output.mkdir(parents=True, exist_ok=True)
cases = {}
artifacts = {}
for source in sorted(args.source.glob("*.ndjson")):
    data = source.read_bytes()
    artifacts[source.name] = hashlib.sha256(data).hexdigest()
    for line in data.splitlines():
        record = json.loads(line)
        canonical = json.dumps(record, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        cases[canonical] = record
records = sorted(cases.values(), key=lambda r: (r["operation"], json.dumps(r["input"], sort_keys=True)))
payload = "\n".join(json.dumps(r, ensure_ascii=False, sort_keys=True, separators=(",", ":")) for r in records).encode("utf-8") + b"\n"
compressed = gzip.compress(payload, mtime=0)
(args.output / "released-investigation.ndjson.gz").write_bytes(compressed)
provenance = {
    "release_commit": "418e550f173938bad7dc7d86995d4a12945f9f23",
    "capture_run": "https://github.com/mattpainter701/graphlayer/actions/runs/39182018248",
    "capture_method": "Instrumented public entry points in an isolated exact release checkout; all released firewall, policy, pathanalysis, networkpath, investigation and HTTP application tests passed.",
    "artifact_sha256": artifacts,
    "fixture_sha256": hashlib.sha256(compressed).hexdigest(),
    "case_count": len(records),
    "operations": dict(sorted(collections.Counter(r["operation"] for r in records).items())),
}
(args.output / "released-investigation.provenance.json").write_text(json.dumps(provenance, indent=2) + "\n", encoding="utf-8")
print(json.dumps(provenance, indent=2))
