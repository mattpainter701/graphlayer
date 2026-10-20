#!/usr/bin/env python3
"""Archive a self-hosted Actions artifact without uploading it to GitHub."""

from __future__ import annotations

import glob
import hashlib
import io
import json
import os
import re
import stat
import sys
import tarfile
import tempfile
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath

EVIDENCE = Path("/opt/graphlayer-ci-evidence")
NAME = re.compile(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}\Z")
SHA = re.compile(r"[0-9a-fA-F]{40}\Z")


class ArchiveError(RuntimeError):
    pass


def _within(path: Path, root: Path) -> bool:
    return path == root or root in path.parents


def _source_files(patterns: str, workspace: str, runner_temp: str) -> list[tuple[Path, str]]:
    roots = []
    for label, value in (("workspace", workspace), ("runner-temp", runner_temp)):
        root = Path(value)
        if not root.is_absolute() or root.is_symlink() or not root.is_dir():
            raise ArchiveError(f"Invalid {label} root")
        roots.append((label, root.resolve(strict=True)))
    if roots[0][1] == roots[1][1]:
        raise ArchiveError("Workspace and runner temp roots must differ")

    files: dict[Path, str] = {}
    excluded: set[Path] = set()
    lines = [line.strip() for line in patterns.splitlines() if line.strip()]
    if not lines:
        raise ArchiveError("At least one artifact path is required")
    for line in lines:
        exclude = line.startswith("!")
        raw = line[1:] if exclude else line
        item = Path(raw)
        if not raw or any(part == ".." for part in item.parts):
            raise ArchiveError("Artifact path contains traversal")
        expression = Path(os.path.abspath(item if item.is_absolute() else roots[0][1] / item))
        selected = next(((label, root) for label, root in roots if _within(expression, root)), None)
        if selected is None:
            raise ArchiveError("Artifact path escapes allowed roots")
        label, root = selected
        for match in map(Path, glob.glob(str(expression), recursive=True)):
            if not _within(match, root):
                raise ArchiveError("Artifact path escapes allowed roots")
            current = match
            while current != root:
                if current.is_symlink():
                    raise ArchiveError("Symlinks are not allowed in artifacts")
                current = current.parent
            resolved = match.resolve(strict=True)
            if not _within(resolved, root):
                raise ArchiveError("Artifact path escapes allowed roots")
            candidates = [resolved]
            if resolved.is_dir():
                candidates = []
                for directory, dirs, names in os.walk(resolved, followlinks=False):
                    base = Path(directory)
                    if any((base / name).is_symlink() for name in dirs):
                        raise ArchiveError("Symlinks are not allowed in artifacts")
                    candidates.extend(base / name for name in names)
            for candidate in candidates:
                current = candidate
                while current != root:
                    if current.is_symlink():
                        raise ArchiveError("Symlinks are not allowed in artifacts")
                    current = current.parent
                info = candidate.lstat()
                if not stat.S_ISREG(info.st_mode):
                    raise ArchiveError("Artifacts must contain regular files")
                if exclude:
                    excluded.add(candidate)
                else:
                    files[candidate] = label
    return [(path, label) for path, label in sorted(files.items()) if path not in excluded]


def _hash(path: Path) -> tuple[int, str]:
    digest = hashlib.sha256()
    size = 0
    with path.open("rb") as source:
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
            size += len(chunk)
    return size, digest.hexdigest()


def _append_summary(result: dict[str, object] | None, path: str) -> None:
    if result is None or not path:
        return
    summary = Path(path)
    if not summary.is_absolute() or summary.is_symlink():
        raise ArchiveError("Invalid GITHUB_STEP_SUMMARY path")
    if "warning" in result:
        text = f"\n### Local CI evidence\n\nNo files matched artifact `{result['name']}`.\n"
    else:
        text = (
            "\n### Local CI evidence\n\n"
            f"Artifact: `{result['name']}`\n"
            f"Files: {result['file_count']}\n"
            f"Bytes: {result['byte_count']}\n"
            f"Archive SHA-256: `{result['archive_sha256']}`\n"
            f"Manifest SHA-256: `{result['manifest_sha256']}`\n"
            f"Retention metadata: {result['retention_days']} days\n"
        )
    with summary.open("a", encoding="utf-8") as output:
        output.write(text)


def create_archive(
    *, name: str, patterns: str, if_no_files_found: str, retention_days: int,
    workspace: str, runner_temp: str, sha: str, run_id: str, job: str,
    evidence: Path = EVIDENCE,
) -> dict[str, object] | None:
    policy = if_no_files_found.casefold()
    if not NAME.fullmatch(name) or ".." in name:
        raise ArchiveError("Invalid artifact name")
    if not NAME.fullmatch(job) or ".." in job:
        raise ArchiveError("Invalid GITHUB_JOB")
    if not SHA.fullmatch(sha):
        raise ArchiveError("GITHUB_SHA must be a 40-character commit SHA")
    if not run_id.isdigit() or int(run_id) < 1:
        raise ArchiveError("GITHUB_RUN_ID must be positive")
    if policy not in {"warn", "error", "ignore"}:
        raise ArchiveError("if-no-files-found must be warn, error, or ignore")
    if not 1 <= retention_days <= 90:
        raise ArchiveError("retention-days must be between 1 and 90")

    sources = _source_files(patterns, workspace, runner_temp)
    if not sources:
        if policy == "error":
            raise ArchiveError("No files matched artifact paths")
        return {"warning": "no_files", "name": name} if policy == "warn" else None

    if evidence.is_symlink():
        raise ArchiveError("Evidence root may not be a symlink")
    evidence.mkdir(parents=True, exist_ok=True)
    resolved_evidence = evidence.resolve(strict=True)
    if resolved_evidence != evidence.absolute():
        raise ArchiveError("Evidence root path may not contain symlinks")
    evidence = resolved_evidence
    target = evidence / sha.lower() / run_id / job
    cursor = evidence
    for part in target.relative_to(evidence).parts:
        cursor /= part
        if cursor.is_symlink():
            raise ArchiveError("Evidence path may not contain symlinks")
        cursor.mkdir(exist_ok=True)
        if cursor.resolve(strict=True) != cursor:
            raise ArchiveError("Evidence path may not contain symlinks")

    files = []
    for path, root_name in sources:
        root = {"workspace": Path(workspace).resolve(), "runner-temp": Path(runner_temp).resolve()}[root_name]
        size, digest = _hash(path)
        files.append((path, f"{root_name}/{path.relative_to(root).as_posix()}", size, digest))
    manifest = json.dumps(
        {
            "artifact": name,
            "created_utc": datetime.now(timezone.utc).isoformat(),
            "retention_days": retention_days,
            "file_count": len(files),
            "byte_count": sum(item[2] for item in files),
            "files": [{"path": item[1], "size": item[2], "sha256": item[3]} for item in files],
        },
        sort_keys=True,
        separators=(",", ":"),
    ).encode()
    fd, temp_name = tempfile.mkstemp(dir=target, prefix=f".{name}.", suffix=".tmp")
    os.close(fd)
    temporary = Path(temp_name)
    archive_path = target / f"{name}.tar.gz"
    try:
        with temporary.open("wb") as output:
            with tarfile.open(fileobj=output, mode="w:gz") as archive:
                for path, member, size, digest in files:
                    if PurePosixPath(member).is_absolute() or ".." in PurePosixPath(member).parts:
                        raise ArchiveError("Unsafe archive member path")
                    info = tarfile.TarInfo(member)
                    info.size, info.mode, info.mtime = size, stat.S_IMODE(path.stat().st_mode), 0
                    with path.open("rb") as source:
                        archive.addfile(info, source)
                info = tarfile.TarInfo("MANIFEST.json")
                info.size, info.mode, info.mtime = len(manifest), 0o600, 0
                archive.addfile(info, io.BytesIO(manifest))
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, archive_path)
    except BaseException:
        temporary.unlink(missing_ok=True)
        raise

    archive_sha = _hash(archive_path)[1]
    return {
        "name": name,
        "file_count": len(files),
        "byte_count": sum(item[2] for item in files),
        "archive_sha256": archive_sha,
        "manifest_sha256": hashlib.sha256(manifest).hexdigest(),
        "retention_days": retention_days,
    }


def main() -> int:
    try:
        result = create_archive(
            name=os.getenv("INPUT_NAME", ""),
            patterns=os.getenv("INPUT_PATH", ""),
            if_no_files_found=os.getenv("INPUT_IF_NO_FILES_FOUND", "warn"),
            retention_days=int(os.getenv("INPUT_RETENTION_DAYS", "7")),
            workspace=os.getenv("GITHUB_WORKSPACE", ""),
            runner_temp=os.getenv("RUNNER_TEMP", ""),
            sha=os.getenv("GITHUB_SHA", ""),
            run_id=os.getenv("GITHUB_RUN_ID", ""),
            job=os.getenv("GITHUB_JOB", ""),
        )
        _append_summary(result, os.getenv("GITHUB_STEP_SUMMARY", ""))
        if result is None:
            print("[local-ci-artifact] no_files=ignored")
        elif "warning" in result:
            print(f"::warning title=Local artifact not found::{result['name']}")
        else:
            print(
                f"[local-ci-artifact] archived name={result['name']} files={result['file_count']} "
                f"bytes={result['byte_count']} sha256={result['archive_sha256']} "
                f"manifest_sha256={result['manifest_sha256']}"
            )
        return 0
    except (ArchiveError, OSError, ValueError, tarfile.TarError) as exc:
        print(f"[local-ci-artifact] error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
