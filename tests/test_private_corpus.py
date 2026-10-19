import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "check-private-corpus.py"
SPEC = importlib.util.spec_from_file_location("check_private_corpus", SCRIPT)
checker = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.modules[SPEC.name] = checker
SPEC.loader.exec_module(checker)


class PrivateCorpusTests(unittest.TestCase):
    def test_inventory_counts_candidates_and_skips_archives_without_opening_files(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "one.conf").write_bytes(b"config")
            (root / "extensionless").write_bytes(b"text")
            (root / "archive.zip").write_bytes(b"not read")
            (root / "ignored.json").write_bytes(b"not read")
            (root / "folder").mkdir()
            (root / "folder" / "two.XML").write_bytes(b"nested")
            found = checker.inventory_tree(root)
            self.assertEqual(found.files, 5)
            self.assertEqual(found.directories, 1)
            self.assertEqual(len(found.candidates), 3)
            self.assertEqual(found.skipped["archive"], 1)
            self.assertEqual(found.skipped["unsupported_extension"], 1)

    def test_inventory_skips_oversize_files_before_candidate_list(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "large.cli").write_bytes(b"x" * (checker.MAX_FILE_BYTES + 1))
            found = checker.inventory_tree(root)
            self.assertEqual(found.candidates, [])
            self.assertEqual(found.skipped["oversize"], 1)

    def test_binary_detector_handles_nuls_and_control_heavy_prefixes(self):
        self.assertTrue(checker.looks_binary(b"text\x00tail"))
        self.assertTrue(checker.looks_binary(bytes(range(32)) * 3))
        self.assertFalse(checker.looks_binary(b"interface inside\n  ip address 192.0.2.1\n"))
        self.assertFalse(checker.looks_binary(b""))

    def test_evidence_structure_accepts_valid_hash_and_nested_references(self):
        parsed = {
            "artifact_sha256": "a" * 64,
            "source_lines": [{"number": 1, "text": "interface outside"}, {"number": 2, "text": "shutdown"}],
            "evidence": [{"id": "opaque-ref", "start_line": 1, "end_line": 2}],
            "path": {"evidence_refs": ["opaque-ref"]},
            "chains": [{"rules": [{"evidence_refs": ["opaque-ref"]}]}],
        }
        self.assertEqual(checker.evidence_structure(parsed, "a" * 64), (True, "ok"))

    def test_evidence_structure_rejects_hash_range_and_reference_errors(self):
        parsed = {
            "artifact_sha256": "b" * 64,
            "source_lines": [{"number": 1, "text": "line"}],
            "evidence": [{"id": "ref", "start_line": 1, "end_line": 2}],
        }
        self.assertEqual(checker.evidence_structure(parsed, "a" * 64)[1], "artifact_sha256_mismatch")
        self.assertEqual(checker.evidence_structure(parsed, "b" * 64)[1], "invalid_evidence_range")
        parsed["evidence"][0]["end_line"] = 1
        parsed["path"] = {"evidence_refs": ["unknown"]}
        self.assertEqual(checker.evidence_structure(parsed, "b" * 64)[1], "unknown_evidence_reference")

    def test_report_contains_only_aggregate_data_and_opaque_hashes(self):
        inventory = checker.Inventory(files=1, directories=0, bytes=7, candidates=[Path("hidden.cfg")])
        result = checker.summarize(inventory, [("parsed", "cisco-asa", "f" * 64)])
        self.assertEqual(result["results"]["vendor_counts"], {"cisco-asa": 1})
        self.assertEqual(result["results"]["artifact_sha256"], ["f" * 64])
        self.assertNotIn("hidden.cfg", repr(result))

    def test_worker_request_uses_existing_client_with_fixed_deadline_and_selected_engine(self):
        with tempfile.TemporaryDirectory() as temp:
            engine = Path(temp) / "engine"
            with patch.dict("os.environ", {"GRAPHLAYER_ENGINE": "prior-engine"}, clear=False):
                with patch.object(checker.rust_worker, "call_rust", return_value={"vendor": "cisco-asa"}) as call:
                    result = checker.worker_request(engine, "parse_config", {"sample": True})
                self.assertEqual(result, ({"vendor": "cisco-asa"}, "ok"))
                call.assert_called_once_with("parse_config", {"sample": True}, timeout=30)
                self.assertEqual(__import__("os").environ["GRAPHLAYER_ENGINE"], "prior-engine")

    def test_worker_failures_are_mapped_without_exposing_transport_messages(self):
        with tempfile.TemporaryDirectory() as temp:
            with patch.object(checker.rust_worker, "call_rust", side_effect=checker.rust_worker.RustWorkerError("private response body")):
                result = checker.worker_request(Path(temp) / "engine", "parse_config", {})
        self.assertEqual(result, (None, "worker_error"))
        self.assertNotIn("private response body", repr(result))

    def test_invalid_explicit_engine_does_not_fall_back_to_discovered_engine(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "target" / "release").mkdir(parents=True)
            executable = "graphlayer-engine.exe" if __import__("os").name == "nt" else "graphlayer-engine"
            (root / "target" / "release" / executable).touch()
            with patch.dict("os.environ", {}, clear=False):
                self.assertIsNone(checker.resolve_engine(str(root / "missing-engine"), root))

    def test_corpus_inside_checkout_is_rejected_before_traversal(self):
        from contextlib import redirect_stderr
        import io

        errors = io.StringIO()
        with redirect_stderr(errors):
            status = checker.main(["--corpus", str(checker.REPO_ROOT), "--inventory-only"])
        self.assertEqual(status, 2)
        self.assertNotIn(str(checker.REPO_ROOT), errors.getvalue())


if __name__ == "__main__":
    unittest.main()
