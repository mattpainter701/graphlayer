from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from platform import rust_worker
from platform.rust_worker import RustWorkerError, call_rust


class RustWorkerTransportTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tempdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tempdir.cleanup)
        self.previous_executable = os.environ.get("GRAPHLAYER_ENGINE")
        self.addCleanup(self._restore_executable)

    def _restore_executable(self) -> None:
        if self.previous_executable is None:
            os.environ.pop("GRAPHLAYER_ENGINE", None)
        else:
            os.environ["GRAPHLAYER_ENGINE"] = self.previous_executable

    def _worker(self, source: str) -> None:
        path = Path(self.tempdir.name) / "fake_engine.py"
        path.write_text(source, encoding="utf-8")
        os.environ["GRAPHLAYER_ENGINE"] = str(path.resolve())

    def _call(self, *args, **kwargs):
        with patch.object(rust_worker, "_worker_command",
                          side_effect=lambda executable: [sys.executable, executable]):
            return call_rust(*args, **kwargs)

    @staticmethod
    def _response(result: object) -> str:
        return json.dumps({
            "protocol_version": "graphlayer-engine-v1",
            "operation": "connector.test",
            "result": result,
        })

    def test_transports_credentials_only_over_stdin_and_checks_operation(self) -> None:
        self._worker(
            "import json,sys\n"
            "request=json.load(sys.stdin)\n"
            "assert request['input']['password']=='transient-secret'\n"
            "print(json.dumps({'protocol_version':'graphlayer-engine-v1',"
            "'operation':request['operation'],'result':request['input']['password']}))\n"
        )
        self.assertEqual(self._call("connector.test", {"password": "transient-secret"}),
                         "transient-secret")

    def test_rejects_nested_duplicate_json_keys(self) -> None:
        response = self._response({"safe": 1})
        response = response.replace('"safe": 1', '"safe": 1, "safe": 2')
        self._worker(f"import sys\nsys.stdin.read()\nsys.stdout.write({response!r})\n")
        with self.assertRaisesRegex(RustWorkerError, "invalid response"):
            self._call("connector.test", {})

    def test_rejects_trailing_non_json_bytes(self) -> None:
        response = self._response(True) + " trailing"
        self._worker(f"import sys\nsys.stdin.read()\nsys.stdout.write({response!r})\n")
        with self.assertRaisesRegex(RustWorkerError, "invalid response"):
            self._call("connector.test", {})

    def test_bounds_output_and_does_not_expose_worker_output(self) -> None:
        self._worker(f"import sys\nsys.stdin.read()\nsys.stdout.write('X' * {rust_worker.MAX_RESPONSE_BYTES + 1})\n")
        with patch.object(rust_worker.json, "loads", side_effect=AssertionError("oversized output reached the JSON decoder")):
            with self.assertRaisesRegex(RustWorkerError, "invalid response") as caught:
                self._call("connector.test", {}, timeout=10)
        self.assertNotIn("X", str(caught.exception))

    def test_sanitizes_engine_errors_and_stderr(self) -> None:
        error = {
            "protocol_version": "graphlayer-engine-v1",
            "operation": "connector.test",
            "error": "credential-secret and raw-server-response",
        }
        source = ("import json,sys\nsys.stdin.read()\n"
                  f"sys.stderr.write({'stderr-secret'!r})\n"
                  f"sys.stdout.write({json.dumps(error)!r})\n")
        self._worker(source)
        with self.assertRaises(RustWorkerError) as caught:
            self._call("connector.test", {})
        for secret in ("credential-secret", "raw-server-response", "stderr-secret"):
            self.assertNotIn(secret, str(caught.exception))

    def test_rejects_ambiguous_or_unexpected_response_envelopes(self) -> None:
        base = {"protocol_version": "graphlayer-engine-v1", "operation": "connector.test"}
        invalid = [
            {**base, "result": None},
            {**base, "result": True, "error_kind": "domain"},
            {**base, "result": True, "unexpected": "value"},
            {**base, "error": "failed", "result": True},
            {**base, "error": 4},
            {**base, "result": True, "protocol_version": 1},
            {**base, "result": True, "operation": ["connector.test"]},
        ]
        for response in invalid:
            with self.subTest(response=response):
                self._worker(
                    "import json,sys\nsys.stdin.read()\n"
                    f"sys.stdout.write({json.dumps(response)!r})\n"
                )
                with self.assertRaisesRegex(RustWorkerError, "invalid response"):
                    self._call("connector.test", {})

    def test_rejects_non_finite_exponent_values(self) -> None:
        response = '{"protocol_version":"graphlayer-engine-v1","operation":"connector.test","result":{"n":1e999}}'
        self._worker(f"import sys\nsys.stdin.read()\nsys.stdout.write({response!r})\n")
        with self.assertRaisesRegex(RustWorkerError, "invalid response"):
            self._call("connector.test", {})

    def test_enforces_timeout(self) -> None:
        self._worker("import sys,time\nsys.stdin.read()\ntime.sleep(10)\n")
        with self.assertRaisesRegex(RustWorkerError, "timed out"):
            self._call("connector.test", {}, timeout=1)


if __name__ == "__main__":
    unittest.main()
