#!/usr/bin/env python3
"""Reject tracked repository content matching local privacy indicators."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import threading
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = REPO_ROOT / "config" / "repository-privacy.json"
MAX_FILE_BYTES = 32 * 1024 * 1024
MAX_GZIP_BYTES = 16 * 1024 * 1024
HASH_CHUNK_BYTES = 1024 * 1024
LFS_POINTER = re.compile(rb"^version https://git-lfs\.github\.com/spec/v1\r?\noid sha256:([0-9a-f]{64})\r?\nsize ([0-9]+)\r?\n?$")
ASCII_NORMALIZE = bytes.maketrans(
    b"ABCDEFGHIJKLMNOPQRSTUVWXYZ", b"abcdefghijklmnopqrstuvwxyz"
)


def normalized_ascii(raw: bytes) -> bytes:
    lowered = raw.translate(ASCII_NORMALIZE)
    return bytes(byte for byte in lowered if 48 <= byte <= 57 or 97 <= byte <= 122)


def window_digests(raw: bytes, window: int) -> set[str]:
    normalized = normalized_ascii(raw)
    if len(normalized) < window:
        return set()
    return {
        hashlib.sha256(normalized[index:index + window]).hexdigest()
        for index in range(len(normalized) - window + 1)
    }


def is_reparse(path: Path, metadata: os.stat_result) -> bool:
    attributes = getattr(metadata, "st_file_attributes", 0)
    reparse_flag = getattr(stat, "FILE_ATTRIBUTE_REPARSE_POINT", 0x400)
    return path.is_symlink() or bool(attributes & reparse_flag)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while chunk := stream.read(HASH_CHUNK_BYTES):
            digest.update(chunk)
    return digest.hexdigest()


def tracked_paths(root: Path) -> list[bytes]:
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return [entry for entry in result.stdout.split(b"\0") if entry]


def worktree_roots(root: Path) -> list[Path]:
    result = subprocess.run(
        ["git", "-C", str(root), "worktree", "list", "--porcelain"],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    roots = []
    for line in result.stdout.splitlines():
        if line.startswith("worktree "):
            roots.append(Path(line[9:]).resolve())
    return roots


def git_bytes(root: Path, *arguments: str) -> bytes:
    result = subprocess.run(
        ["git", "-C", str(root), *arguments],
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    return result.stdout


def inside(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
        return True
    except ValueError:
        return False


def read_file(path: Path) -> tuple[bytes, bytes | None] | None:
    try:
        metadata = path.lstat()
    except FileNotFoundError:
        return None
    if is_reparse(path, metadata) or not stat.S_ISREG(metadata.st_mode):
        return b"", None
    if metadata.st_size > MAX_FILE_BYTES:
        raise ValueError("tracked file exceeds scan limit")
    raw = path.read_bytes()
    if path.suffix.lower() == ".gz":
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(raw)) as stream:
                expanded = stream.read(MAX_GZIP_BYTES + 1)
        except (OSError, EOFError):
            raise ValueError("tracked gzip file could not be safely decompressed") from None
        if len(expanded) > MAX_GZIP_BYTES:
            raise ValueError("tracked gzip file exceeds decompression limit")
        return raw, expanded
    return raw, None


def load_config(path: Path) -> dict[str, Any]:
    config = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(config, dict):
        raise ValueError("privacy config root must be an object")
    if config.get("schema_version") != 1:
        raise ValueError("privacy config has an unsupported schema version")
    window = config.get("window_length")
    digests = config.get("protected_digests")
    blocked = config.get("blocked_path_components", [])
    blocked_sequences = config.get("blocked_path_sequences", [])
    if window != 6 or not isinstance(digests, list) or not digests:
        raise ValueError("privacy config must define six-byte windows and protected digests")
    if any(not isinstance(value, str) or not re.fullmatch(r"[0-9a-f]{64}", value) for value in digests):
        raise ValueError("privacy config contains an invalid digest")
    if not isinstance(blocked, list) or any(not isinstance(value, str) for value in blocked):
        raise ValueError("privacy config contains invalid path components")
    if not isinstance(blocked_sequences, list) or any(
        not isinstance(sequence, list) or len(sequence) < 2 or any(not isinstance(value, str) for value in sequence)
        for sequence in blocked_sequences
    ):
        raise ValueError("privacy config contains invalid path sequences")
    return {
        "window_length": window,
        "protected_digests": set(digests),
        "blocked_path_components": {normalized_ascii(value.encode("ascii")) for value in blocked},
        "blocked_path_sequences": [
            [normalized_ascii(value.encode("ascii")) for value in sequence]
            for sequence in blocked_sequences
        ],
    }


def collect_corpus_hashes(corpus: Path | None, roots: list[Path]) -> set[str]:
    corpus_hashes: set[str] = set()
    if corpus is None:
        return corpus_hashes
    if not corpus.is_dir() or any(corpus == worktree or inside(corpus, worktree) for worktree in roots):
        raise ValueError("private corpus must be an available directory outside the checkout")
    for current, dirs, files in os.walk(corpus, followlinks=False):
        base = Path(current)
        kept_dirs = []
        for name in dirs:
            candidate = base / name
            try:
                metadata = candidate.lstat()
            except OSError:
                raise ValueError("private corpus could not be safely inventoried") from None
            if not is_reparse(candidate, metadata):
                kept_dirs.append(name)
        dirs[:] = kept_dirs
        for name in files:
            candidate = base / name
            try:
                metadata = candidate.lstat()
                if is_reparse(candidate, metadata) or not stat.S_ISREG(metadata.st_mode):
                    continue
                corpus_hashes.add(sha256_file(candidate))
            except OSError:
                raise ValueError("private corpus could not be safely hashed") from None
    return corpus_hashes


def read_exact(stream: Any, size: int) -> bytes:
    chunks = bytearray()
    while len(chunks) < size:
        chunk = stream.read(size - len(chunks))
        if not chunk:
            break
        chunks.extend(chunk)
    return bytes(chunks)


def visit_git_objects(root: Path, object_ids: list[str], visitor: Any) -> None:
    if not object_ids:
        return
    process = subprocess.Popen(
        ["git", "-C", str(root), "cat-file", "--batch"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    assert process.stdin is not None and process.stdout is not None

    def feed_objects() -> None:
        try:
            process.stdin.write(("\n".join(object_ids) + "\n").encode("ascii"))
            process.stdin.flush()
        except (BrokenPipeError, OSError):
            pass
        finally:
            try:
                process.stdin.close()
            except OSError:
                pass

    writer = threading.Thread(target=feed_objects, daemon=True)
    writer.start()
    read_complete = False
    try:
        for expected_oid in object_ids:
            header = process.stdout.readline()
            parts = header.rstrip(b"\n").split(b" ")
            if len(parts) != 3 or parts[0].decode("ascii", "ignore") != expected_oid:
                raise ValueError("Git object stream is incomplete")
            object_type = parts[1]
            try:
                size = int(parts[2])
            except ValueError:
                raise ValueError("Git object size is invalid") from None
            if size < 0 or size > MAX_FILE_BYTES:
                raise ValueError("Git object exceeds scan limit")
            payload = read_exact(process.stdout, size)
            if len(payload) != size or process.stdout.read(1) != b"\n":
                raise ValueError("Git object stream is truncated")
            visitor(expected_oid, object_type, payload)
        read_complete = True
    finally:
        if not read_complete and process.poll() is None:
            try:
                process.kill()
            except OSError:
                pass
        writer.join()
        process.wait()
    if process.returncode != 0:
        raise ValueError("Git object stream could not be completed")


def scan(root: Path, config: dict[str, Any], corpus: Path | None = None,
         corpus_hashes: set[str] | None = None) -> dict[str, int]:
    protected = config["protected_digests"]
    window = config["window_length"]
    blocked_components = config["blocked_path_components"]
    blocked_sequences = config["blocked_path_sequences"]
    roots = worktree_roots(root)
    if corpus_hashes is None:
        corpus_hashes = collect_corpus_hashes(corpus, roots)

    counts = {
        "tracked_files": 0,
        "missing_tracked_files": 0,
        "reparse_entries_skipped": 0,
        "blocked_path_matches": 0,
        "protected_window_matches": 0,
        "private_corpus_hash_matches": 0,
        "lfs_pointers": 0,
    }
    for entry in tracked_paths(root):
        counts["tracked_files"] += 1
        path_text = entry.decode("utf-8", errors="surrogateescape")
        components = path_text.replace("\\", "/").split("/")
        normalized_components = [
            normalized_ascii(component.encode("utf-8", errors="surrogateescape"))
            for component in components
        ]
        has_blocked_sequence = any(
            any(normalized_components[index:index + len(sequence)] == sequence
                for index in range(len(normalized_components) - len(sequence) + 1))
            for sequence in blocked_sequences
        )
        if any(component in blocked_components for component in normalized_components) or has_blocked_sequence:
            counts["blocked_path_matches"] += 1
        path = root / path_text
        try:
            file_data = read_file(path)
        except OSError:
            raise ValueError("tracked file could not be read") from None
        except ValueError:
            raise
        if file_data is None:
            counts["missing_tracked_files"] += 1
            continue
        raw, expanded = file_data
        if raw == b"":
            try:
                metadata = path.lstat()
            except FileNotFoundError:
                counts["missing_tracked_files"] += 1
                continue
            if is_reparse(path, metadata) or not stat.S_ISREG(metadata.st_mode):
                counts["reparse_entries_skipped"] += 1
                continue
        pointer = LFS_POINTER.fullmatch(raw)
        if pointer:
            counts["lfs_pointers"] += 1
            oid = pointer.group(1).decode("ascii")
            if oid in corpus_hashes:
                counts["private_corpus_hash_matches"] += 1
        if hashlib.sha256(raw).hexdigest() in corpus_hashes or (
            expanded is not None and hashlib.sha256(expanded).hexdigest() in corpus_hashes
        ):
            counts["private_corpus_hash_matches"] += 1
        scanned_content = raw + (b"\n" + expanded if expanded is not None else b"")
        if window_digests(entry, window) & protected or window_digests(scanned_content, window) & protected:
            counts["protected_window_matches"] += 1
    return counts


def scan_index(root: Path, config: dict[str, Any], corpus_hashes: set[str] | None = None) -> dict[str, int]:
    entries = git_bytes(root, "ls-files", "--stage", "-z").split(b"\0")
    indexed: list[tuple[str, bytes]] = []
    object_ids: list[str] = []
    seen: set[str] = set()
    for entry in entries:
        if not entry:
            continue
        header, separator, path_bytes = entry.partition(b"\t")
        fields = header.split(b" ")
        if not separator or len(fields) != 3 or fields[2] != b"0":
            raise ValueError("Git index contains an unmerged or invalid entry")
        mode, oid_bytes, _stage = fields
        if mode not in (b"100644", b"100755"):
            raise ValueError("Git index contains a non-regular file entry")
        if not re.fullmatch(rb"[0-9a-f]{40,64}", oid_bytes):
            raise ValueError("Git index object identifier is invalid")
        oid = oid_bytes.decode("ascii")
        indexed.append((oid, path_bytes))
        if oid not in seen:
            seen.add(oid)
            object_ids.append(oid)

    counts = {
        "index_files": len(indexed),
        "index_blocked_path_matches": 0,
        "index_protected_window_matches": 0,
        "index_private_corpus_hash_matches": 0,
        "index_lfs_pointers": 0,
    }
    blocked_components = config["blocked_path_components"]
    blocked_sequences = config["blocked_path_sequences"]
    window = config["window_length"]
    protected = config["protected_digests"]
    for _, path_bytes in indexed:
        normalized_components = [
            normalized_ascii(component)
            for component in path_bytes.replace(b"\\", b"/").split(b"/")
        ]
        sequence_hit = any(
            any(normalized_components[index:index + len(sequence)] == sequence
                for index in range(len(normalized_components) - len(sequence) + 1))
            for sequence in blocked_sequences
        )
        if any(component in blocked_components for component in normalized_components) or sequence_hit:
            counts["index_blocked_path_matches"] += 1
        if window_digests(path_bytes, window) & protected:
            counts["index_protected_window_matches"] += 1

    def inspect_index_blob(_oid: str, object_type: bytes, payload: bytes) -> None:
        if object_type != b"blob":
            raise ValueError("Git index entry does not reference a blob")
        expanded = None
        if payload.startswith(b"\x1f\x8b"):
            try:
                with gzip.GzipFile(fileobj=io.BytesIO(payload)) as stream:
                    expanded = stream.read(MAX_GZIP_BYTES + 1)
            except (OSError, EOFError):
                raise ValueError("indexed gzip blob could not be safely decompressed") from None
            if len(expanded) > MAX_GZIP_BYTES:
                raise ValueError("indexed gzip blob exceeds decompression limit")
        if window_digests(payload, window) & protected or (
            expanded is not None and window_digests(expanded, window) & protected
        ):
            counts["index_protected_window_matches"] += 1
        pointer = LFS_POINTER.fullmatch(payload)
        if pointer:
            counts["index_lfs_pointers"] += 1
            if corpus_hashes is not None and pointer.group(1).decode("ascii") in corpus_hashes:
                counts["index_private_corpus_hash_matches"] += 1
        if corpus_hashes is not None and (
            hashlib.sha256(payload).hexdigest() in corpus_hashes
            or (expanded is not None and hashlib.sha256(expanded).hexdigest() in corpus_hashes)
        ):
            counts["index_private_corpus_hash_matches"] += 1

    visit_git_objects(root, object_ids, inspect_index_blob)
    return counts


def scan_history(root: Path, config: dict[str, Any], revision: str = "HEAD",
                 corpus_hashes: set[str] | None = None) -> dict[str, int]:
    if git_bytes(root, "rev-parse", "--is-shallow-repository").strip() == b"true":
        raise ValueError("history scan requires complete repository history")
    commit = git_bytes(root, "rev-parse", "--verify", "--end-of-options", f"{revision}^{{commit}}").strip().decode("ascii")
    object_output = git_bytes(root, "rev-list", "--objects", "--no-object-names", commit)
    object_ids: list[str] = []
    seen: set[str] = set()
    for oid_bytes in object_output.splitlines():
        if not re.fullmatch(rb"[0-9a-f]{40,64}", oid_bytes):
            raise ValueError("history object inventory is invalid")
        oid = oid_bytes.decode("ascii")
        if oid not in seen:
            seen.add(oid)
            object_ids.append(oid)

    history_paths = git_bytes(
        root, "log", "--format=", "--name-only", "-z", "--root", "-m", "--full-history", commit
    )
    counts = {
        "history_commits": 0,
        "history_trees": 0,
        "history_blobs": 0,
        "history_paths": 0,
        "history_protected_window_matches": 0,
        "history_blocked_path_matches": 0,
        "history_private_corpus_hash_matches": 0,
        "history_lfs_pointers": 0,
    }
    protected = config["protected_digests"]
    window = config["window_length"]
    blocked_components = config["blocked_path_components"]
    blocked_sequences = config["blocked_path_sequences"]

    def count_path(path_bytes: bytes) -> None:
        counts["history_paths"] += 1
        normalized_components = [
            normalized_ascii(component)
            for component in path_bytes.replace(b"\\", b"/").split(b"/")
        ]
        sequence_hit = any(
            any(normalized_components[index:index + len(sequence)] == sequence
                for index in range(len(normalized_components) - len(sequence) + 1))
            for sequence in blocked_sequences
        )
        if any(component in blocked_components for component in normalized_components) or sequence_hit:
            counts["history_blocked_path_matches"] += 1
        if window_digests(path_bytes, window) & protected:
            counts["history_protected_window_matches"] += 1

    for path_bytes in history_paths.split(b"\0"):
        if path_bytes:
            count_path(path_bytes)

    def inspect_history_object(_oid: str, object_type: bytes, payload: bytes) -> None:
        if object_type == b"commit":
            counts["history_commits"] += 1
        elif object_type == b"tree":
            counts["history_trees"] += 1
        elif object_type == b"blob":
            counts["history_blobs"] += 1
        else:
            raise ValueError("history contains an unsupported object type")

        expanded = None
        if object_type == b"blob" and payload.startswith(b"\x1f\x8b"):
            try:
                with gzip.GzipFile(fileobj=io.BytesIO(payload)) as stream:
                    expanded = stream.read(MAX_GZIP_BYTES + 1)
            except (OSError, EOFError):
                raise ValueError("history gzip blob could not be safely decompressed") from None
            if len(expanded) > MAX_GZIP_BYTES:
                raise ValueError("history gzip blob exceeds decompression limit")
        if object_type in (b"commit", b"blob"):
            payload_matches = bool(window_digests(payload, window) & protected)
            if expanded is not None:
                payload_matches = payload_matches or bool(window_digests(expanded, window) & protected)
            if payload_matches:
                counts["history_protected_window_matches"] += 1
        if object_type == b"blob":
            pointer = LFS_POINTER.fullmatch(payload)
            if pointer:
                counts["history_lfs_pointers"] += 1
                if corpus_hashes is not None and pointer.group(1).decode("ascii") in corpus_hashes:
                    counts["history_private_corpus_hash_matches"] += 1
            if corpus_hashes is not None and (
                hashlib.sha256(payload).hexdigest() in corpus_hashes
                or (expanded is not None and hashlib.sha256(expanded).hexdigest() in corpus_hashes)
            ):
                counts["history_private_corpus_hash_matches"] += 1

    visit_git_objects(root, object_ids, inspect_history_object)
    return counts


def validate_report_destination(destination: Path, corpus: Path | None, roots: list[Path]) -> None:
    resolved = destination.resolve()
    if any(resolved == root or inside(resolved, root) for root in roots):
        raise ValueError("report destination must be outside every Git worktree")
    if corpus is not None:
        resolved_corpus = corpus.resolve()
        if resolved == resolved_corpus or inside(resolved, resolved_corpus):
            raise ValueError("report destination must be outside the private corpus")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=str(REPO_ROOT))
    parser.add_argument("--config", default=str(DEFAULT_CONFIG))
    parser.add_argument("--private-corpus")
    parser.add_argument("--report")
    parser.add_argument("--history", action="store_true", help="scan all objects reachable from a Git commit")
    parser.add_argument("--history-ref", default="HEAD", help="commitish to scan with --history")
    args = parser.parse_args(argv)
    root = Path(args.repo).expanduser().resolve()
    corpus = Path(args.private_corpus).expanduser().resolve() if args.private_corpus else None
    try:
        config = load_config(Path(args.config).expanduser().resolve())
        roots = worktree_roots(root)
        corpus_hashes = collect_corpus_hashes(corpus, roots)
        if args.history:
            bare = git_bytes(root, "rev-parse", "--is-bare-repository").strip() == b"true"
            counts = scan_history(root, config, args.history_ref, corpus_hashes)
            if not bare:
                counts.update(scan(root, config, corpus, corpus_hashes))
                counts.update(scan_index(root, config, corpus_hashes))
        elif args.history_ref != "HEAD":
            raise ValueError("--history-ref requires --history")
        else:
            counts = scan(root, config, corpus, corpus_hashes)
            counts.update(scan_index(root, config, corpus_hashes))
        if args.report:
            destination = Path(args.report).expanduser().resolve()
            validate_report_destination(destination, corpus, roots)
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(json.dumps(counts, sort_keys=True, indent=2) + "\n", encoding="utf-8")
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError, ValueError):
        print("repository privacy scan failed; no path or content details are emitted", file=sys.stderr)
        return 2
    print(json.dumps(counts, sort_keys=True))
    return 1 if any(counts.get(key, 0) for key in (
        "blocked_path_matches",
        "protected_window_matches",
        "private_corpus_hash_matches",
        "lfs_pointers",
        "reparse_entries_skipped",
        "index_blocked_path_matches",
        "index_protected_window_matches",
        "index_private_corpus_hash_matches",
        "index_lfs_pointers",
        "history_protected_window_matches",
        "history_blocked_path_matches",
        "history_private_corpus_hash_matches",
        "history_lfs_pointers",
    )) else 0


if __name__ == "__main__":
    raise SystemExit(main())
