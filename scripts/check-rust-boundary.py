#!/usr/bin/env python3
"""Enforce the Rust domain boundary and freeze the remaining legacy engine."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
from pathlib import Path
import re
import sys
from typing import Any


DEFAULT_CONFIG = Path("config/rust-boundary.json")
EXCLUDED_PARTS = {"backup", "docs", "generated", "test", "tests", "testdata", "tools", "__pycache__"}
EXCLUDED_SUFFIXES = ("_test.go", "_test.py")


def sha256(path: Path) -> str:
    normalized = path.read_bytes().replace(b"\r\n", b"\n")
    return hashlib.sha256(normalized).hexdigest()


def is_production_source(relative: str, extensions: set[str]) -> bool:
    path = Path(relative)
    if path.suffix not in extensions or any(part.lower() in EXCLUDED_PARTS for part in path.parts):
        return False
    return not path.name.endswith(EXCLUDED_SUFFIXES)


def _load_exceptions(config: dict[str, Any], today: dt.date) -> tuple[dict[str, tuple[str, str]], list[str]]:
    allowed: dict[str, tuple[str, str]] = {}
    errors: list[str] = []
    valid_categories = {"security_fix", "correctness_fix", "migration_adapter", "transport_adapter"}
    for item in config.get("exceptions", []):
        if not isinstance(item, dict):
            errors.append("exception entries must be objects")
            continue
        path = item.get("path")
        category = item.get("category")
        reason = item.get("reason")
        owner = item.get("owner")
        approved_hash = item.get("sha256")
        expiry = item.get("expires")
        if not all(isinstance(value, str) and value.strip() for value in (path, category, reason, owner, approved_hash, expiry)):
            errors.append(f"exception must provide path, category, owner, sha256, reason, and expires: {item!r}")
            continue
        if re.fullmatch(r"[0-9a-f]{64}", approved_hash) is None:
            errors.append(f"exception for {path} must pin a lowercase SHA256 digest")
            continue
        if category not in valid_categories:
            errors.append(f"exception for {path} has unsupported category {category!r}")
            continue
        try:
            expires = dt.date.fromisoformat(expiry)
        except ValueError:
            errors.append(f"exception for {path} has invalid expiry {expiry!r} (use YYYY-MM-DD)")
            continue
        if expires <= today:
            errors.append(f"exception for {path} must have a future expiry; current expiry is {expiry}")
            continue
        if expires > today + dt.timedelta(days=90):
            errors.append(f"exception for {path} expires more than 90 days from {today.isoformat()}")
            continue
        normalized = Path(path).as_posix()
        if normalized.startswith("/") or ".." in Path(normalized).parts:
            errors.append(f"exception path must be repository-relative: {path!r}")
            continue
        if normalized in allowed:
            errors.append(f"duplicate exception for {normalized}")
            continue
        description = f"{category}, owner {owner.strip()}: {reason.strip()} (expires {expiry})"
        allowed[normalized] = (approved_hash, description)
    return allowed, errors


def check_boundary(root: Path, config: dict[str, Any], today: dt.date | None = None) -> list[str]:
    """Return policy violations for a repository tree and parsed policy config."""
    today = today or dt.date.today()
    baseline = config.get("baseline_sha256", {})
    baseline_paths = config.get("baseline_paths", [])
    roots = config.get("guarded_roots", [])
    new_file_roots = config.get("new_file_roots", [])
    errors: list[str] = []
    if not isinstance(baseline, dict) or not isinstance(baseline_paths, list) or not isinstance(roots, list) or not isinstance(new_file_roots, list):
        return ["config requires baseline_sha256 object, baseline_paths list, and guarded_roots/new_file_roots lists"]
    if not all(isinstance(path, str) for path in baseline_paths):
        return ["baseline_paths must contain repository-relative paths"]

    exceptions, exception_errors = _load_exceptions(config, today)
    errors.extend(exception_errors)

    def discover(entries: list[Any], label: str) -> dict[str, Path]:
        found: dict[str, Path] = {}
        for entry in entries:
            if not isinstance(entry, dict) or not isinstance(entry.get("path"), str):
                errors.append(f"invalid {label} root entry: {entry!r}")
                continue
            relative_root = Path(entry["path"])
            extensions = entry.get("extensions")
            if not isinstance(extensions, list) or not all(isinstance(ext, str) and ext.startswith(".") for ext in extensions):
                errors.append(f"{label} root {entry['path']!r} must list file extensions")
                continue
            scan_root = root / relative_root
            if not scan_root.exists():
                continue
            paths = [scan_root] if scan_root.is_file() else scan_root.rglob("*")
            for path in paths:
                if path.is_file():
                    relative = path.relative_to(root).as_posix()
                    if is_production_source(relative, set(extensions)):
                        found[relative] = path
        return found

    discovered_domain = discover(roots, "guarded")
    discovered_all = discover(new_file_roots, "new-file")

    for relative, path in sorted(discovered_all.items()):
        actual = sha256(path)
        exception = exceptions.get(relative)
        if relative not in baseline_paths and exception is None:
            errors.append(f"new Go/Python production source is prohibited: {relative}; implement domain behavior in Rust")
        if exception is not None and actual != exception[0]:
            errors.append(f"exception SHA256 does not match approved content for {relative}")

    for relative, path in sorted(discovered_domain.items()):
        actual = sha256(path)
        expected = baseline.get(relative)
        exception = exceptions.get(relative)
        if expected is None and exception is None:
            errors.append(f"new legacy production source is prohibited: {relative}; implement it in Rust")
        elif expected is not None and actual != expected and exception is None:
            errors.append(
                f"frozen legacy source changed: {relative}; port it to Rust or add a narrow, expiring exception"
            )
    for relative, (_, description) in exceptions.items():
        if relative not in discovered_all:
            errors.append(f"stale exception for missing or unguarded file: {relative} ({description})")
    return errors


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parent.parent,
                        help="repository root (defaults to the parent of scripts/)")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG,
                        help="policy JSON path relative to --root")
    args = parser.parse_args(argv)
    root = args.root.resolve()
    config_path = args.config if args.config.is_absolute() else root / args.config
    try:
        config = json.loads(config_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"Rust boundary policy: cannot read {config_path}: {exc}", file=sys.stderr)
        return 2
    errors = check_boundary(root, config)
    if errors:
        print("Rust boundary check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print("Rust boundary check passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
