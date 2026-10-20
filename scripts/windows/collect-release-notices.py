#!/usr/bin/env python3
"""Build deterministic Windows third-party notices from locked tool reports."""

from __future__ import annotations

import argparse
import csv
import hashlib
import html
import json
from pathlib import Path
from pathlib import PurePosixPath
import re
import stat
import sys
from urllib.parse import urlparse


class NoticeError(ValueError):
    pass


def regular_file(path: Path) -> bytes:
    try:
        info = path.lstat()
    except OSError as exc:
        raise NoticeError(f"Missing required notice input: {path.name}") from exc
    if not stat.S_ISREG(info.st_mode) or getattr(info, "st_file_attributes", 0) & 0x400:
        raise NoticeError(f"Notice input must be a regular, non-reparse file: {path.name}")
    data = path.read_bytes()
    if not data or b"\x00" in data:
        raise NoticeError(f"Notice input is empty or binary: {path.name}")
    return data


def text_file(path: Path) -> str:
    return regular_file(path).decode("utf-8-sig", errors="strict").replace("\r\n", "\n").replace("\r", "\n")


def safe_tree(root: Path, allowed_name: re.Pattern[str], *, required: bool = True) -> list[tuple[str, str]]:
    try:
        root_info = root.lstat()
    except OSError as exc:
        raise NoticeError(f"Required notice directory is missing or unsafe: {root.name}") from exc
    if not stat.S_ISDIR(root_info.st_mode) or stat.S_ISLNK(root_info.st_mode) or getattr(root_info, "st_file_attributes", 0) & 0x400:
        raise NoticeError(f"Required notice directory is missing or unsafe: {root.name}")
    files: list[tuple[str, str]] = []
    for path in sorted(root.rglob("*"), key=lambda item: item.relative_to(root).as_posix().casefold()):
        relative = path.relative_to(root).as_posix()
        info = path.lstat()
        if stat.S_ISLNK(info.st_mode) or getattr(info, "st_file_attributes", 0) & 0x400:
            raise NoticeError(f"Reparse points are not allowed in notice inputs: {relative}")
        if path.is_dir():
            continue
        if not path.is_file() or not allowed_name.fullmatch(path.name):
            raise NoticeError(f"Unexpected file in notice materials (source or unreviewed file): {relative}")
        files.append((relative, text_file(path)))
    if required and not files:
        raise NoticeError(f"Notice directory has no accepted materials: {root.name}")
    return files


def check_directory(path: Path, label: str) -> None:
    try:
        info = path.lstat()
    except OSError as exc:
        raise NoticeError(f"Required {label} directory is missing or unsafe") from exc
    if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode) or getattr(info, "st_file_attributes", 0) & 0x400:
        raise NoticeError(f"Required {label} directory is missing or unsafe")


def cargo_entries(report_path: Path, accepted: set[str]) -> list[str]:
    try:
        report = json.loads(text_file(report_path))
    except (json.JSONDecodeError, UnicodeError) as exc:
        raise NoticeError("cargo-about report is not valid UTF-8 JSON") from exc
    if not isinstance(report, dict) or not isinstance(report.get("licenses"), list) or not isinstance(report.get("crates"), list):
        raise NoticeError("cargo-about report is missing its licenses/crates arrays")
    if not report["licenses"] or not report["crates"]:
        raise NoticeError("cargo-about report contains no Windows dependency inventory")
    uses: dict[tuple[str, str], set[str]] = {}
    license_sections: list[tuple[str, str, str]] = []
    ids_seen: set[str] = set()
    for item in report["licenses"]:
        if not isinstance(item, dict):
            raise NoticeError("cargo-about license record is malformed")
        spdx = item.get("id")
        name = item.get("name")
        body = item.get("text")
        users = item.get("used_by")
        if not isinstance(spdx, str) or not spdx or spdx not in accepted:
            raise NoticeError(f"Unreviewed Rust dependency license: {spdx or 'unknown'}")
        if not isinstance(name, str) or not name.strip() or not isinstance(body, str) or not body.strip():
            raise NoticeError(f"Rust license {spdx} is missing its name or full text")
        if not isinstance(users, list) or not users:
            raise NoticeError(f"Rust license {spdx} has no dependency attribution")
        ids_seen.add(spdx)
        for used in users:
            crate = used.get("crate") if isinstance(used, dict) else None
            if not isinstance(crate, dict):
                raise NoticeError(f"Rust license {spdx} has malformed crate attribution")
            crate_name, version = crate.get("name"), crate.get("version")
            if not isinstance(crate_name, str) or not crate_name or not isinstance(version, str) or not version:
                raise NoticeError(f"Rust license {spdx} has incomplete crate attribution")
            uses.setdefault((crate_name, version), set()).add(spdx)
        license_sections.append((spdx, name.strip(), body.rstrip()))
    if not ids_seen:
        raise NoticeError("cargo-about produced no reviewed license identifiers")
    crate_keys: set[tuple[str, str]] = set()
    for item in report["crates"]:
        package = item.get("package") if isinstance(item, dict) else None
        if not isinstance(package, dict):
            raise NoticeError("cargo-about crate record is malformed")
        name, version = package.get("name"), package.get("version")
        if not isinstance(name, str) or not name or not isinstance(version, str) or not version:
            raise NoticeError("cargo-about crate has no name/version")
        crate_keys.add((name, version))
    if crate_keys != set(uses):
        missing = sorted(crate_keys - set(uses))
        orphaned = sorted(set(uses) - crate_keys)
        raise NoticeError(f"Rust dependency attribution is incomplete (unattributed={len(missing)}, unlisted={len(orphaned)})")
    lines = ["Rust dependencies (target: x86_64-pc-windows-msvc)", ""]
    for (name, version), spdx_set in sorted(uses.items(), key=lambda item: (item[0][0].casefold(), item[0][1])):
        lines.append(f"- {name} {version} — {', '.join(sorted(spdx_set))}")
    for spdx, name, body in sorted(set(license_sections), key=lambda entry: (entry[0], entry[1], hashlib.sha256(entry[2].encode()).hexdigest())):
        lines.extend(["", f"## Rust license text: {name} ({spdx})", "", body])
    return lines


def go_entries(report_path: Path, accepted: set[str]) -> tuple[list[str], set[str]]:
    try:
        rows = list(csv.reader(text_file(report_path).splitlines()))
    except csv.Error as exc:
        raise NoticeError("go-licenses report is not valid CSV") from exc
    if not rows:
        raise NoticeError("go-licenses report contains no Windows dependency inventory")
    packages: dict[str, tuple[str, str]] = {}
    for row in rows:
        if len(row) != 3:
            raise NoticeError("go-licenses report row must have package, license URL and license name")
        package, url, license_name = (field.strip() for field in row)
        package_path = PurePosixPath(package)
        parsed = urlparse(url)
        if (not package or package_path.is_absolute() or "\\" in package or
                any(part in ("", ".", "..") for part in package_path.parts) or
                not license_name or license_name not in accepted or parsed.scheme != "https" or not parsed.netloc):
            raise NoticeError(f"Unreviewed or incomplete Go dependency license: {license_name or 'unknown'}")
        if package in packages and packages[package] != (url, license_name):
            raise NoticeError(f"Conflicting Go license records for {package}")
        packages[package] = (url, license_name)
    return ["Go dependencies (target: windows/amd64, CGO_ENABLED=0)", ""] + [
        f"- {package} — {license_name}; license source: {url}"
        for package, (url, license_name) in sorted(packages.items(), key=lambda item: item[0].casefold())
    ], set(packages)


def validate_go_material_coverage(packages: set[str], materials: list[tuple[str, str]]) -> None:
    material_roots = {PurePosixPath(path).parent.as_posix() for path, _ in materials}
    uncovered = sorted(package for package in packages if not any(
        package == root or package.startswith(root + "/") for root in material_roots
    ))
    unmapped = sorted(root for root in material_roots if not any(
        package == root or package.startswith(root + "/") for package in packages
    ))
    if uncovered or unmapped:
        raise NoticeError(f"Go dependency notice coverage mismatch (uncovered={len(uncovered)}, unmapped={len(unmapped)})")


def add_material_section(lines: list[str], title: str, files: list[tuple[str, str]]) -> None:
    lines.extend(["", f"## {title}"])
    for name, body in files:
        lines.extend(["", f"### {name}", "", body.rstrip()])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, required=True)
    parser.add_argument("--cargo-report", type=Path, required=True)
    parser.add_argument("--go-report", type=Path, required=True)
    parser.add_argument("--go-materials", type=Path, required=True)
    parser.add_argument("--go-toolchain-notices", type=Path, required=True)
    parser.add_argument("--rust-stdlib-notices", type=Path, required=True)
    parser.add_argument("--output-directory", type=Path, required=True)
    args = parser.parse_args()
    repo = args.repo_root.resolve(strict=True)
    output = args.output_directory.absolute()
    if output.exists():
        raise NoticeError("Output directory must be new")
    license_file = repo / "LICENSE"
    project_license = regular_file(license_file)
    if (repo / "third_party/d3/LICENSE").resolve(strict=True).parent != (repo / "third_party/d3").resolve(strict=True):
        raise NoticeError("D3 license path escaped the checked-in source tree")
    d3_text = text_file(repo / "third_party/d3/LICENSE")
    if "Mike Bostock" not in d3_text or "Permission to use, copy, modify" not in d3_text:
        raise NoticeError("Checked-in D3 ISC license text is missing or unexpected")
    d3_bundle = repo / "platform/investigation/web/d3.min.js"
    d3_header = regular_file(d3_bundle).splitlines()[0].decode("utf-8", errors="strict")
    if not re.match(r"^// https://d3js\.org v7\.9\.0\b", d3_header):
        raise NoticeError("Vendored D3 bundle version does not match the notice inventory")
    try:
        policy = json.loads(text_file(repo / "config/licenses/license-policy.json"))
    except (OSError, json.JSONDecodeError, UnicodeError) as exc:
        raise NoticeError("Reviewed license policy is missing or invalid") from exc
    accepted_values = policy.get("accepted_spdx") if isinstance(policy, dict) else None
    if not isinstance(accepted_values, list) or not accepted_values or any(not isinstance(value, str) for value in accepted_values):
        raise NoticeError("Reviewed license policy has no valid accepted SPDX identifiers")
    accepted = set(accepted_values)
    cargo = cargo_entries(args.cargo_report, accepted)
    go_saved = safe_tree(args.go_materials, re.compile(r"(?i)(?:LICENSE|COPYING|NOTICE|PATENTS)(?:[._-][A-Za-z0-9._-]+)?"))
    go, go_packages = go_entries(args.go_report, accepted)
    validate_go_material_coverage(go_packages, go_saved)
    go_toolchain = safe_tree(args.go_toolchain_notices, re.compile(r"(?i)(?:LICENSE|COPYING|NOTICE|PATENTS)(?:[._-][A-Za-z0-9._-]+)?"))
    rust_root = args.rust_stdlib_notices
    check_directory(rust_root, "Rust standard-library notice")
    rust_root_names = sorted(path.name for path in rust_root.iterdir())
    if rust_root_names != ["COPYRIGHT-library.html", "licenses"]:
        raise NoticeError("Rust standard-library notice directory has unexpected files")
    rust_copyright = text_file(rust_root / "COPYRIGHT-library.html")
    if "Rust" not in html.unescape(rust_copyright[:2000]):
        raise NoticeError("Rust standard-library attribution file is unexpected")
    rust_licenses = safe_tree(rust_root / "licenses", re.compile(r"[A-Za-z0-9][A-Za-z0-9.-]*\.txt"))
    lines = [
        "GraphLayer Windows release third-party notices",
        "",
        "This inventory records locked build dependency metadata and upstream notice material. It is not a legal opinion or provider eligibility determination.",
        "",
        "## Project and bundled component licenses",
        "",
        "GraphLayer source and binaries: Apache-2.0 (see LICENSE.txt).",
        "D3 7.9.0: ISC; full checked-in text follows.",
        "",
        "### D3 ISC license",
        "",
        d3_text.rstrip(),
        "",
    ]
    lines.extend(cargo)
    lines.extend(["", *go])
    add_material_section(lines, "Go dependency license and notice files from go-licenses save", go_saved)
    add_material_section(lines, "Go toolchain and standard-library notice materials", go_toolchain)
    add_material_section(lines, "Rust standard-library attribution from the pinned toolchain", [("COPYRIGHT-library.html", rust_copyright)])
    add_material_section(lines, "Rust toolchain license text bundle", rust_licenses)
    notice_bytes = ("\n".join(lines).rstrip() + "\n").encode("utf-8")
    output.mkdir(parents=True, exist_ok=False)
    try:
        (output / "LICENSE.txt").write_bytes(project_license)
        (output / "THIRD-PARTY-NOTICES.txt").write_bytes(notice_bytes)
    except OSError:
        # The caller owns the new destination; leave partial output visible for diagnosis.
        raise
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (NoticeError, OSError, UnicodeError) as error:
        print(f"Release notice collection failed: {error}", file=sys.stderr)
        raise SystemExit(1)
