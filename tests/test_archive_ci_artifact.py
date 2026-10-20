from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "ci" / "archive-artifact.py"
SPEC = importlib.util.spec_from_file_location("archive_ci_artifact", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
archive_module = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = archive_module
SPEC.loader.exec_module(archive_module)
ArchiveError = archive_module.ArchiveError
create_archive = archive_module.create_archive


SHA = "a" * 40


class LocalArtifactArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.workspace = self.root / "workspace"
        self.runner_temp = self.root / "runner-temp"
        self.evidence = self.root / "evidence"
        self.workspace.mkdir()
        self.runner_temp.mkdir()

    def tearDown(self):
        self.temp.cleanup()

    def create(self, **overrides):
        values = {
            "name": "coverage",
            "patterns": "platform/coverage.out",
            "if_no_files_found": "error",
            "retention_days": 7,
            "workspace": str(self.workspace),
            "runner_temp": str(self.runner_temp),
            "sha": SHA,
            "run_id": "12345",
            "job": "test",
            "evidence": self.evidence,
        }
        values.update(overrides)
        return create_archive(**values)

    def test_archive_is_atomic_relative_and_has_content_manifest(self):
        source = self.workspace / "platform" / "coverage.out"
        source.parent.mkdir()
        source.write_bytes(b"synthetic coverage data\n")

        result = self.create()

        archive_path = self.evidence / SHA / "12345" / "test" / "coverage.tar.gz"
        self.assertTrue(archive_path.is_file())
        self.assertFalse(list(archive_path.parent.glob("*.tmp")))
        with tarfile.open(archive_path, "r:gz") as archive:
            members = archive.getnames()
            self.assertEqual(members, ["workspace/platform/coverage.out", "MANIFEST.json"])
            self.assertTrue(all(not Path(name).is_absolute() and ".." not in Path(name).parts for name in members))
            manifest_bytes = archive.extractfile("MANIFEST.json").read()
            manifest = json.loads(manifest_bytes)
            self.assertEqual(manifest["file_count"], 1)
            self.assertEqual(manifest["byte_count"], len(b"synthetic coverage data\n"))
            self.assertEqual(manifest["files"][0]["sha256"], hashlib.sha256(source.read_bytes()).hexdigest())
        self.assertEqual(result["manifest_sha256"], hashlib.sha256(manifest_bytes).hexdigest())
        self.assertEqual(result["archive_sha256"], hashlib.sha256(archive_path.read_bytes()).hexdigest())

    def test_runner_temp_absolute_patterns_get_relative_archive_names(self):
        source = self.runner_temp / "topology-scale" / "result.json"
        source.parent.mkdir()
        source.write_text("{}\n", encoding="utf-8")

        result = self.create(patterns=str(source.parent))

        archive_path = self.evidence / SHA / "12345" / "test" / "coverage.tar.gz"
        with tarfile.open(archive_path, "r:gz") as archive:
            self.assertIn("runner-temp/topology-scale/result.json", archive.getnames())
        self.assertEqual(result["file_count"], 1)

    def test_path_traversal_and_paths_outside_allowed_roots_are_rejected(self):
        outside = self.root / "outside.txt"
        outside.write_text("synthetic", encoding="utf-8")
        with self.assertRaisesRegex(ArchiveError, "traversal"):
            self.create(patterns="../outside.txt")
        with self.assertRaisesRegex(ArchiveError, "escapes"):
            self.create(patterns=str(outside))

    def test_symlinked_artifact_is_rejected(self):
        outside = self.root / "outside.txt"
        outside.write_text("synthetic", encoding="utf-8")
        link = self.workspace / "link.txt"
        try:
            os.symlink(outside, link)
        except (OSError, NotImplementedError):
            self.skipTest("symlink creation is unavailable")
        with self.assertRaisesRegex(ArchiveError, "Symlinks are not allowed"):
            self.create(patterns="link.txt")

    def test_no_files_policy_warn_ignore_and_error(self):
        warning = self.create(patterns="missing/**", if_no_files_found="warn")
        self.assertEqual(warning["warning"], "no_files")
        self.assertIsNone(self.create(patterns="missing/**", if_no_files_found="ignore"))
        with self.assertRaisesRegex(ArchiveError, "No files matched"):
            self.create(patterns="missing/**", if_no_files_found="error")

    def test_step_summary_contains_metadata_without_source_paths(self):
        source = self.workspace / "private-name-not-shown.txt"
        source.write_text("synthetic", encoding="utf-8")
        result = self.create(patterns=source.name)
        summary = self.runner_temp / "summary.md"

        archive_module._append_summary(result, str(summary))
        text = summary.read_text(encoding="utf-8")
        self.assertIn("Manifest SHA-256", text)
        self.assertIn("Files: 1", text)
        self.assertNotIn(source.name, text)
        self.assertNotIn(str(self.evidence), text)

    def test_artifact_name_and_run_identity_cannot_escape_evidence_root(self):
        with self.assertRaisesRegex(ArchiveError, "artifact name"):
            self.create(name="../outside")
        with self.assertRaisesRegex(ArchiveError, "GITHUB_SHA"):
            self.create(sha="not-a-commit")


if __name__ == "__main__":
    unittest.main()
