from __future__ import annotations

import gzip
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import pytest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "check-repository-privacy.py"
SPEC = importlib.util.spec_from_file_location("repository_privacy", SCRIPT)
privacy = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(privacy)


def make_repo(tmp_path: Path) -> Path:
    root = tmp_path / "repo"
    root.mkdir()
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    subprocess.run(["git", "-C", str(root), "config", "user.email", "test@example.invalid"], check=True)
    subprocess.run(["git", "-C", str(root), "config", "user.name", "Test"], check=True)
    return root


def make_config(tmp_path: Path, protected: bytes = b"quasar") -> dict:
    digest = hashlib.sha256(privacy.normalized_ascii(protected)).hexdigest()
    path = tmp_path / "privacy.json"
    path.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [digest],
        "blocked_path_components": [
            "customer_configs", "customers", "sample_data", "raw_configs", "private_configs",
        ],
        "blocked_path_sequences": [["backup", "configs"]],
    }), encoding="utf-8")
    return privacy.load_config(path)


def track(root: Path, relative: str, data: bytes) -> None:
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)
    subprocess.run(["git", "-C", str(root), "add", "--", relative], check=True)


def commit(root: Path, message: str) -> None:
    subprocess.run(["git", "-C", str(root), "add", "-A"], check=True)
    subprocess.run(["git", "-C", str(root), "commit", "-qm", message], check=True)


def test_scans_tracked_paths_content_and_skips_deleted_entries(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/neutral.txt", b"plain content")
    track(root, "customer_configs/example.txt", b"plain content")
    track(root, "backup/configs/old.txt", b"plain content")
    track(root, "testdata/sample.txt", b"plain content")
    track(root, "Customers/export.txt", b"plain content")
    track(root, "sample_data/snapshot.txt", b"plain content")
    track(root, "raw_configs/device.txt", b"plain content")
    track(root, "private_configs/device.txt", b"plain content")
    track(root, "docs/deleted.txt", b"quasar")
    (root / "docs/deleted.txt").unlink()

    counts = privacy.scan(root, make_config(tmp_path))

    assert counts["blocked_path_matches"] == 6
    assert counts["protected_window_matches"] == 0
    assert counts["missing_tracked_files"] == 1


def test_detects_normalized_six_character_window_in_text_and_gzip(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/text.txt", b"prefix Q-U-A-S-A-R suffix")
    track(root, "docs/archive.gz", gzip.compress(b"a quasar occurs here"))

    counts = privacy.scan(root, make_config(tmp_path))

    assert counts["protected_window_matches"] == 2


def test_private_corpus_matches_git_bytes_and_lfs_oid_without_names(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    payload = b"opaque local payload"
    digest = hashlib.sha256(payload).hexdigest()
    track(root, "data/raw.bin", payload)
    track(root, "data/pointer.bin", (
        b"version https://git-lfs.github.com/spec/v1\n"
        + f"oid sha256:{digest}\nsize {len(payload)}\n".encode("ascii")
    ))
    track(root, "data/payload.gz", gzip.compress(payload))
    corpus = tmp_path / "local-corpus"
    corpus.mkdir()
    (corpus / "private-name-not-reported.conf").write_bytes(payload)

    counts = privacy.scan(root, make_config(tmp_path, b"planet"), corpus)

    assert counts["private_corpus_hash_matches"] == 3
    assert counts["lfs_pointers"] == 1
    assert "private-name-not-reported.conf" not in json.dumps(counts)


def test_private_corpus_file_hash_uses_streaming_helper(tmp_path: Path) -> None:
    candidate = tmp_path / "opaque.bin"
    candidate.write_bytes(b"abc")

    assert privacy.sha256_file(candidate) == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"


def test_cli_report_is_aggregate_only_and_rejects_worktree_destination(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/neutral.txt", b"plain content")
    config = tmp_path / "privacy.json"
    digest = hashlib.sha256(b"quasar").hexdigest()
    config.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [digest],
        "blocked_path_components": [],
    }), encoding="utf-8")
    report = tmp_path / "aggregate.json"
    command = [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config), "--report", str(report)]

    result = subprocess.run(command, capture_output=True, text=True)

    assert result.returncode == 0
    assert "neutral.txt" not in result.stdout
    assert json.loads(report.read_text(encoding="utf-8"))["tracked_files"] == 1
    blocked = subprocess.run(command[:-1] + [str(root / "leak.json")], capture_output=True, text=True)
    assert blocked.returncode == 2
    assert "neutral.txt" not in blocked.stderr


def test_tracked_symlink_fails_closed_without_printing_path(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    target = tmp_path / "target.txt"
    target.write_text("plain content", encoding="utf-8")
    link = root / "docs" / "link.txt"
    link.parent.mkdir()
    try:
        os.symlink(target, link)
    except (OSError, NotImplementedError):
        pytest.skip("symlink creation is not available")
    subprocess.run(["git", "-C", str(root), "add", "--", "docs/link.txt"], check=True)
    config = tmp_path / "privacy.json"
    digest = hashlib.sha256(b"quasar").hexdigest()
    config.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [digest],
        "blocked_path_components": [],
    }), encoding="utf-8")

    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config)],
        capture_output=True,
        text=True,
    )

    assert result.returncode == 2
    assert "link.txt" not in result.stdout + result.stderr
    assert "Traceback" not in result.stderr


@pytest.mark.parametrize("kind", ["corrupt", "oversized"])
def test_rejects_corrupt_and_oversized_gzip_without_traceback(tmp_path: Path, kind: str) -> None:
    root = make_repo(tmp_path)
    payload = b"not a gzip stream" if kind == "corrupt" else gzip.compress(b"x" * (16 * 1024 * 1024 + 1))
    track(root, "docs/input.gz", payload)
    config_path = tmp_path / "privacy.json"
    config_path.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
    }), encoding="utf-8")
    command = [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config_path)]

    result = subprocess.run(command, capture_output=True, text=True)

    assert result.returncode == 2
    assert "Traceback" not in result.stderr
    assert str(root) not in result.stderr


def test_rejects_oversized_tracked_file_and_invalid_config_encoding(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    large = root / "docs" / "large.bin"
    large.parent.mkdir()
    with large.open("wb") as stream:
        stream.truncate(privacy.MAX_FILE_BYTES + 1)
    subprocess.run(["git", "-C", str(root), "add", "--", "docs/large.bin"], check=True)
    valid = tmp_path / "valid.json"
    valid.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
    }), encoding="utf-8")
    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(valid)],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 2
    assert "Traceback" not in result.stderr
    assert str(root) not in result.stderr

    invalid = tmp_path / "invalid.json"
    invalid.write_bytes(b"\xff")
    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(invalid)],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 2
    assert "Traceback" not in result.stderr
    assert str(invalid) not in result.stderr


def test_rejects_unsupported_config_version(tmp_path: Path) -> None:
    config = tmp_path / "version.json"
    config.write_text(json.dumps({
        "schema_version": 2,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
    }), encoding="utf-8")
    with pytest.raises(ValueError, match="unsupported schema version"):
        privacy.load_config(config)


def test_history_finds_removed_content_and_cli_reports_counts_only(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/ordinary.txt", b"ordinary content")
    commit(root, "baseline")
    track(root, "docs/retired.gz", gzip.compress(b"prefix q-u-a-s-a-r suffix"))
    track(root, "sample_data/retired.txt", b"ordinary content")
    commit(root, "add temporary fixture")
    (root / "docs" / "retired.gz").unlink()
    (root / "sample_data" / "retired.txt").unlink()
    commit(root, "remove temporary fixture")
    config = make_config(tmp_path)

    assert privacy.scan(root, config)["protected_window_matches"] == 0
    history = privacy.scan_history(root, config)
    assert history["history_protected_window_matches"] > 0
    assert history["history_blocked_path_matches"] > 0

    config_path = tmp_path / "privacy.json"
    config_path.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
        "blocked_path_components": [],
    }), encoding="utf-8")
    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config_path), "--history"],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert json.loads(result.stdout)["protected_window_matches"] == 0
    assert json.loads(result.stdout)["history_protected_window_matches"] > 0
    assert "retired" not in result.stdout + result.stderr

    bare = tmp_path / "history-mirror.git"
    subprocess.run(["git", "clone", "--bare", "-q", str(root), str(bare)], check=True)
    bare_result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(bare), "--config", str(config_path), "--history"],
        capture_output=True,
        text=True,
    )
    assert bare_result.returncode == 1
    assert json.loads(bare_result.stdout)["history_protected_window_matches"] > 0
    assert "retired" not in bare_result.stdout + bare_result.stderr


def test_history_scans_newline_filename_if_supported(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    candidate = root / "docs" / "q-u-a-s-a-r\nentry.txt"
    try:
        candidate.write_text("ordinary content", encoding="utf-8")
    except OSError:
        pytest.skip("host filesystem does not allow newline in filenames")
    subprocess.run(["git", "-C", str(root), "add", "--", str(candidate.relative_to(root))], check=True)
    commit(root, "add unusual path")
    candidate.unlink()
    commit(root, "remove unusual path")

    history = privacy.scan_history(root, make_config(tmp_path))

    assert history["history_protected_window_matches"] > 0


def test_history_scans_commit_metadata(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/safe.txt", b"ordinary content")
    commit(root, "baseline")
    subprocess.run(["git", "-C", str(root), "commit", "--allow-empty", "-qm", "q-u-a-s-a-r metadata"], check=True)

    history = privacy.scan_history(root, make_config(tmp_path))

    assert history["history_protected_window_matches"] > 0


def test_history_compares_lfs_pointer_oid_to_local_corpus(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    payload = b"opaque historical payload"
    oid = hashlib.sha256(payload).hexdigest()
    track(root, "archive/payload.bin", (
        b"version https://git-lfs.github.com/spec/v1\n"
        + f"oid sha256:{oid}\nsize {len(payload)}\n".encode("ascii")
    ))
    commit(root, "add opaque pointer")
    corpus = tmp_path / "local-corpus"
    corpus.mkdir()
    (corpus / "local-only.bin").write_bytes(payload)
    corpus_hashes = privacy.collect_corpus_hashes(corpus, privacy.worktree_roots(root))

    history = privacy.scan_history(root, make_config(tmp_path, b"planet"), corpus_hashes=corpus_hashes)

    assert history["history_lfs_pointers"] == 1
    assert history["history_private_corpus_hash_matches"] == 1


def test_cli_rejects_lfs_pointer_without_printing_oid_or_path(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    oid = hashlib.sha256(b"unavailable historical payload").hexdigest()
    track(root, "opaque/payload.bin", (
        b"version https://git-lfs.github.com/spec/v1\n"
        + f"oid sha256:{oid}\nsize 30\n".encode("ascii")
    ))
    commit(root, "add opaque pointer")
    config = tmp_path / "privacy.json"
    config.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
        "blocked_path_components": [],
    }), encoding="utf-8")

    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config), "--history"],
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    assert json.loads(result.stdout)["lfs_pointers"] == 1
    assert json.loads(result.stdout)["index_lfs_pointers"] == 1
    assert json.loads(result.stdout)["history_lfs_pointers"] == 1
    assert oid not in result.stdout + result.stderr
    assert "payload.bin" not in result.stdout + result.stderr


def test_history_mode_scans_tracked_worktree_edits(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/tracked.txt", b"ordinary content")
    commit(root, "baseline")
    (root / "docs" / "tracked.txt").write_bytes(b"quasar in a pending tracked edit")
    config = tmp_path / "privacy.json"
    config.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
        "blocked_path_components": [],
    }), encoding="utf-8")

    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config), "--history"],
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    assert json.loads(result.stdout)["protected_window_matches"] == 1
    assert "tracked.txt" not in result.stdout + result.stderr


def test_history_mode_scans_index_when_worktree_is_clean(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    tracked = root / "docs" / "staged.txt"
    track(root, "docs/staged.txt", b"ordinary content")
    commit(root, "baseline")
    tracked.write_bytes(b"quasar in staged blob")
    subprocess.run(["git", "-C", str(root), "add", "--", "docs/staged.txt"], check=True)
    tracked.write_bytes(b"ordinary content")
    config = tmp_path / "privacy.json"
    config.write_text(json.dumps({
        "schema_version": 1,
        "window_length": 6,
        "protected_digests": [hashlib.sha256(b"quasar").hexdigest()],
        "blocked_path_components": [],
    }), encoding="utf-8")

    result = subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(root), "--config", str(config), "--history"],
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    counts = json.loads(result.stdout)
    assert counts["protected_window_matches"] == 0
    assert counts["history_protected_window_matches"] == 0
    assert counts["index_protected_window_matches"] == 1
    assert "staged.txt" not in result.stdout + result.stderr


def test_history_refuses_shallow_repository(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    track(root, "docs/safe.txt", b"ordinary content")
    commit(root, "baseline")
    head = subprocess.run(
        ["git", "-C", str(root), "rev-parse", "HEAD"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()
    (root / ".git" / "shallow").write_text(head + "\n", encoding="ascii")

    with pytest.raises(ValueError, match="complete repository history"):
        privacy.scan_history(root, make_config(tmp_path))


def test_history_rejects_oversized_blob(tmp_path: Path) -> None:
    root = make_repo(tmp_path)
    large = root / "docs" / "large.bin"
    large.parent.mkdir()
    with large.open("wb") as stream:
        stream.truncate(privacy.MAX_FILE_BYTES + 1)
    subprocess.run(["git", "-C", str(root), "add", "--", "docs/large.bin"], check=True)
    commit(root, "add oversized opaque blob")

    with pytest.raises(ValueError, match="Git object exceeds scan limit"):
        privacy.scan_history(root, make_config(tmp_path))
