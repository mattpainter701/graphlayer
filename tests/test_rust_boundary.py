import datetime as dt
import importlib.util
from pathlib import Path
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "check-rust-boundary.py"
SPEC = importlib.util.spec_from_file_location("check_rust_boundary", SCRIPT)
guard = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(guard)


class RustBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.source = self.root / "platform" / "vendors" / "sample.go"
        self.source.parent.mkdir(parents=True)
        self.source.write_text("package sample\n", encoding="utf-8")
        self.config = {
            "guarded_roots": [{"path": "platform/vendors", "extensions": [".go", ".py"]}],
            "new_file_roots": [{"path": "platform", "extensions": [".go", ".py"]}],
            "baseline_paths": ["platform/vendors/sample.go"],
            "baseline_sha256": {
                "platform/vendors/sample.go": guard.sha256(self.source)
            },
            "exceptions": [],
        }

    def tearDown(self):
        self.temp.cleanup()

    def test_frozen_legacy_file_passes(self):
        self.assertEqual(guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1)), [])

    def test_new_production_legacy_file_fails(self):
        added = self.source.parent / "new_parser.go"
        added.write_text("package sample\n", encoding="utf-8")
        cross_language = self.source.parent / "new_parser.py"
        cross_language.write_text("def parse(): pass\n", encoding="utf-8")
        unguarded_area = self.root / "platform" / "other"
        unguarded_area.mkdir()
        unguarded_area.joinpath("new_engine.go").write_text("package other\n", encoding="utf-8")
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1))
        self.assertEqual(sum("new Go/Python production source" in error for error in errors), 3)

    def test_changed_frozen_file_fails_but_deletion_is_allowed(self):
        self.source.write_text("package sample\n// changed\n", encoding="utf-8")
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1))
        self.assertTrue(any("frozen legacy source changed" in error for error in errors))
        self.source.unlink()
        self.assertEqual(guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1)), [])

    def test_exception_is_exact_and_must_be_unexpired(self):
        self.source.write_text("package sample\n// reviewed adapter\n", encoding="utf-8")
        self.config["exceptions"] = [{
            "path": "platform/vendors/sample.go",
            "category": "migration_adapter",
            "owner": "network-runtime",
            "sha256": guard.sha256(self.source),
            "reason": "Temporary adapter until the Rust caller lands.",
            "expires": "2026-03-01",
        }]
        self.assertEqual(guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1)), [])
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 3, 2))
        self.assertTrue(any("expiry" in error for error in errors))

    def test_exception_requires_owner_and_hash(self):
        self.config["exceptions"] = [{
            "path": "platform/vendors/sample.go",
            "category": "correctness_fix",
            "reason": "A narrow fix.",
            "expires": "2027-01-01",
        }]
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1))
        self.assertTrue(any("must provide path, category, owner, sha256, reason, and expires" in error for error in errors))

    def test_exception_hash_must_match_approved_content(self):
        self.source.write_text("package sample\n// changed\n", encoding="utf-8")
        self.config["exceptions"] = [{
            "path": "platform/vendors/sample.go",
            "category": "correctness_fix",
            "owner": "network-runtime",
            "sha256": "0" * 64,
            "reason": "A narrow fix.",
            "expires": "2026-03-01",
        }]
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1))
        self.assertTrue(any("SHA256 does not match approved content" in error for error in errors))

    def test_new_transport_adapter_requires_and_accepts_exact_hash(self):
        adapter = self.root / "platform" / "transport" / "adapter.go"
        adapter.parent.mkdir()
        adapter.write_text("package transport\n", encoding="utf-8")
        self.config["exceptions"] = [{
            "path": "platform/transport/adapter.go",
            "category": "transport_adapter",
            "owner": "network-runtime",
            "sha256": guard.sha256(adapter),
            "reason": "Temporary compatibility adapter.",
            "expires": "2026-03-01",
        }]
        self.assertEqual(guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1)), [])

    def test_exception_expiry_must_be_within_90_days(self):
        self.source.write_text("package sample\n// changed\n", encoding="utf-8")
        self.config["exceptions"] = [{
            "path": "platform/vendors/sample.go",
            "category": "correctness_fix",
            "owner": "network-runtime",
            "sha256": guard.sha256(self.source),
            "reason": "A narrow fix.",
            "expires": "2026-04-02",
        }]
        errors = guard.check_boundary(self.root, self.config, dt.date(2026, 1, 1))
        self.assertTrue(any("more than 90 days" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
