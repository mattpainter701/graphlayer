#!/usr/bin/env python3
"""Run the Rust parser and structural checks over a local, private config corpus."""

from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys
from collections import Counter
from dataclasses import dataclass, field
from typing import Any


MAX_FILE_BYTES = 2 * 1024 * 1024
WORKER_TIMEOUT_SECONDS = 30
TEXT_EXTENSIONS = {".txt", ".conf", ".cfg", ".xml", ".cli"}
ARCHIVE_EXTENSIONS = {".zip", ".gz", ".7z", ".rar", ".tar", ".tgz", ".bz2", ".xz"}
REPO_ROOT = Path(__file__).resolve().parents[1]


def _load_worker_client() -> Any:
    source = REPO_ROOT / "platform" / "rust_worker.py"
    spec = importlib.util.spec_from_file_location("graphlayer_private_corpus_rust_worker", source)
    if spec is None or spec.loader is None:
        raise ImportError("Rust worker transport is unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


rust_worker = _load_worker_client()


@dataclass
class Inventory:
    files: int = 0
    directories: int = 0
    bytes: int = 0
    candidates: list[Path] = field(default_factory=list)
    extensions: Counter[str] = field(default_factory=Counter)
    skipped: Counter[str] = field(default_factory=Counter)
    symlinks: int = 0


def extension(path: Path) -> str:
    return path.suffix.lower() or "[extensionless]"


def is_candidate(path: Path) -> bool:
    return path.suffix.lower() in TEXT_EXTENSIONS or not path.suffix


def is_link(path: Path) -> bool:
    junction_check = getattr(path, "is_junction", None)
    return path.is_symlink() or (junction_check is not None and junction_check())


def inventory_tree(root: Path) -> Inventory:
    """Count a tree without following symlinks or reading file contents."""
    result = Inventory()
    for current, dirs, files in os.walk(root, followlinks=False):
        base = Path(current)
        kept_dirs = []
        for dirname in dirs:
            child = base / dirname
            if is_link(child):
                result.symlinks += 1
            else:
                result.directories += 1
                kept_dirs.append(dirname)
        dirs[:] = kept_dirs
        for filename in files:
            path = base / filename
            try:
                if is_link(path):
                    result.symlinks += 1
                    continue
                stat = path.stat(follow_symlinks=False)
            except OSError:
                result.skipped["unreadable_metadata"] += 1
                continue
            if not path.is_file():
                continue
            result.files += 1
            result.bytes += stat.st_size
            ext = extension(path)
            result.extensions[ext] += 1
            if ext in ARCHIVE_EXTENSIONS:
                result.skipped["archive"] += 1
            elif not is_candidate(path):
                result.skipped["unsupported_extension"] += 1
            elif stat.st_size > MAX_FILE_BYTES:
                result.skipped["oversize"] += 1
            else:
                result.candidates.append(path)
    return result


def looks_binary(sample: bytes) -> bool:
    if b"\x00" in sample:
        return True
    if not sample:
        return False
    controls = sum(byte < 32 and byte not in (9, 10, 12, 13) for byte in sample)
    return controls / len(sample) > 0.02


def evidence_structure(parsed: Any, raw_sha256: str) -> tuple[bool, str]:
    """Check source identity and all line/evidence references without exposing values."""
    if not isinstance(parsed, dict) or parsed.get("artifact_sha256") != raw_sha256:
        return False, "artifact_sha256_mismatch"
    lines = parsed.get("source_lines")
    evidence = parsed.get("evidence")
    if not isinstance(lines, list) or not isinstance(evidence, list):
        return False, "missing_source_structure"
    line_numbers: set[int] = set()
    for line in lines:
        if not isinstance(line, dict):
            return False, "invalid_source_line"
        number, text = line.get("number"), line.get("text")
        if isinstance(number, bool) or not isinstance(number, int) or number < 1 or not isinstance(text, str):
            return False, "invalid_source_line"
        if number in line_numbers:
            return False, "duplicate_source_line"
        line_numbers.add(number)
    known: set[str] = set()
    for item in evidence:
        if not isinstance(item, dict):
            return False, "invalid_evidence"
        ref = item.get("id")
        start, end = item.get("start_line"), item.get("end_line")
        if not isinstance(ref, str) or not ref or ref in known:
            return False, "invalid_evidence"
        if any(isinstance(n, bool) or not isinstance(n, int) for n in (start, end)):
            return False, "invalid_evidence_range"
        if start < 1 or end < start or end - start + 1 > len(line_numbers):
            return False, "invalid_evidence_range"
        if any(n not in line_numbers for n in range(start, end + 1)):
            return False, "invalid_evidence_range"
        known.add(ref)
    stack = [parsed]
    while stack:
        value = stack.pop()
        if isinstance(value, dict):
            refs = value.get("evidence_refs")
            if refs is not None:
                if not isinstance(refs, list) or any(not isinstance(ref, str) or ref not in known for ref in refs):
                    return False, "unknown_evidence_reference"
            stack.extend(value.values())
        elif isinstance(value, list):
            stack.extend(value)
    return True, "ok"


def worker_request(engine: Path, operation: str, data: dict[str, Any]) -> tuple[dict[str, Any] | None, str]:
    previous = os.environ.get("GRAPHLAYER_ENGINE")
    was_present = "GRAPHLAYER_ENGINE" in os.environ
    try:
        os.environ["GRAPHLAYER_ENGINE"] = str(engine)
        result = rust_worker.call_rust(operation, data, timeout=WORKER_TIMEOUT_SECONDS)
    except rust_worker.RustWorkerError:
        return None, "worker_error"
    except Exception:
        return None, "worker_error"
    finally:
        if was_present:
            assert previous is not None
            os.environ["GRAPHLAYER_ENGINE"] = previous
        else:
            os.environ.pop("GRAPHLAYER_ENGINE", None)
    return (result, "ok") if isinstance(result, dict) else (None, "worker_invalid_response")


def inspect_candidate(path: Path, engine: Path) -> tuple[str, str, str | None]:
    try:
        with path.open("rb") as stream:
            prefix = stream.read(8192)
            if looks_binary(prefix):
                return "skipped_binary", "binary", None
            raw = prefix + stream.read(MAX_FILE_BYTES + 1 - len(prefix))
    except OSError:
        return "skipped_unreadable", "unreadable", None
    if len(raw) > MAX_FILE_BYTES:
        return "skipped_oversize", "oversize", None
    digest = hashlib.sha256(raw).hexdigest()
    parsed, status = worker_request(engine, "parse_config", {
        "config_base64": base64.b64encode(raw).decode("ascii"), "vendor": "auto",
    })
    if parsed is None:
        return "parse_error", status, digest
    valid, reason = evidence_structure(parsed, digest)
    if not valid:
        return "invalid_structure", reason, digest
    vendor = parsed.get("vendor")
    safe_vendor = vendor if vendor in {"cisco-asa", "paloalto-panos", "fortinet-fortigate"} else "unknown"
    if parsed.get("parser_version"):
        operations = ["validate_inventory"]
        if "path" in parsed:
            operations.append("validate_path_config")
        for operation in operations:
            validation, validation_status = worker_request(engine, operation, {"config": parsed})
            if validation is None or validation.get("valid") is not True:
                return "validation_error", validation_status if validation is None else "validation_failed", digest
    return "parsed", safe_vendor, digest


def summarize(inventory: Inventory, outcomes: list[tuple[str, str, str | None]]) -> dict[str, Any]:
    statuses = Counter(status for status, _, _ in outcomes)
    vendors = Counter(detail for status, detail, _ in outcomes if status == "parsed")
    errors = Counter(detail for status, detail, _ in outcomes if status != "parsed")
    return {
        "inventory": {
            "files": inventory.files,
            "directories": inventory.directories,
            "bytes": inventory.bytes,
            "extensions": dict(sorted(inventory.extensions.items())),
            "symlinks_skipped": inventory.symlinks,
            "noncandidate_skipped": dict(sorted(inventory.skipped.items())),
            "candidate_files": len(inventory.candidates),
        },
        "results": {
            "status_counts": dict(sorted(statuses.items())),
            "vendor_counts": dict(sorted(vendors.items())),
            "detail_counts": dict(sorted(errors.items())),
            "artifact_sha256": sorted(digest for _, _, digest in outcomes if digest),
        },
    }


def resolve_engine(value: str | None, repo: Path) -> Path | None:
    if value is not None:
        candidate = Path(value).expanduser().resolve()
        return candidate if candidate.is_file() else None
    if "GRAPHLAYER_ENGINE" in os.environ:
        configured = os.environ["GRAPHLAYER_ENGINE"]
        if not configured:
            return None
        candidate = Path(configured).expanduser().resolve()
        return candidate if candidate.is_file() else None
    executable = "graphlayer-engine.exe" if os.name == "nt" else "graphlayer-engine"
    candidates = (repo / "target" / "release" / executable, repo / "target" / "debug" / executable)
    return next((candidate.resolve() for candidate in candidates if candidate.is_file()), None)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--corpus", default=os.environ.get("GRAPHLAYER_PRIVATE_CORPUS"))
    parser.add_argument("--engine")
    parser.add_argument("--report", help="write aggregate results and opaque input hashes to a local JSON file")
    parser.add_argument("--inventory-only", action="store_true", help="count files without parsing any content")
    args = parser.parse_args(argv)
    if not args.corpus:
        parser.error("provide --corpus or GRAPHLAYER_PRIVATE_CORPUS")
    root = Path(args.corpus).expanduser().resolve()
    if not root.is_dir():
        print("error: corpus directory is unavailable", file=sys.stderr)
        return 2
    repo = REPO_ROOT
    if root == repo or repo in root.parents:
        print("error: corpus must be outside the checkout", file=sys.stderr)
        return 2
    engine = None if args.inventory_only else resolve_engine(args.engine, repo)
    if not args.inventory_only and engine is None:
        print("error: Rust engine is unavailable; set --engine or GRAPHLAYER_ENGINE", file=sys.stderr)
        return 2
    inventory = inventory_tree(root)
    outcomes = []
    if engine is not None:
        for path in inventory.candidates:
            outcomes.append(inspect_candidate(path, engine))
    report = summarize(inventory, outcomes)
    report["mode"] = "inventory-only" if args.inventory_only else "rust-parse-and-validate"
    console_report = dict(report)
    console_report["results"] = dict(report["results"])
    console_report["results"].pop("artifact_sha256", None)
    print(json.dumps(console_report, sort_keys=True))
    if args.report:
        destination = Path(args.report).expanduser().resolve()
        if destination == repo or repo in destination.parents or destination == root or root in destination.parents:
            print("error: report must be outside the checkout and corpus", file=sys.stderr)
            return 2
        try:
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        except OSError:
            print("error: could not write the selected report location", file=sys.stderr)
            return 2
    failed = any(status != "parsed" for status, _, _ in outcomes)
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
