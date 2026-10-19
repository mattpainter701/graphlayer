"""Small, fail-closed transport for the Rust engine's JSON-lines request protocol."""

from __future__ import annotations

import json
import os
import subprocess
import threading
import time
from typing import Any


PROTOCOL_VERSION = "graphlayer-engine-v1"
MAX_REQUEST_BYTES = 12 * 1024 * 1024
MAX_RESPONSE_BYTES = 8 * 1024 * 1024
DEFAULT_TIMEOUT_SECONDS = 120


class RustWorkerError(RuntimeError):
    """The required Rust worker is unavailable or rejected a request."""


def _worker_command(executable: str) -> list[str]:
    return [executable]


def _unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON object key")
        result[key] = value
    return result


def _reject_constant(_: str) -> None:
    raise ValueError("non-finite JSON number")


def _read_bounded(process: subprocess.Popen[bytes], output: bytearray,
                  overflow: threading.Event) -> None:
    assert process.stdout is not None
    while True:
        chunk = process.stdout.read(64 * 1024)
        if not chunk:
            return
        remaining = MAX_RESPONSE_BYTES + 1 - len(output)
        if len(chunk) > remaining:
            output.extend(chunk[:max(0, remaining)])
            overflow.set()
            try:
                process.terminate()
            except OSError:
                pass
            return
        output.extend(chunk)


def _write_request(process: subprocess.Popen[bytes], encoded: bytes) -> None:
    assert process.stdin is not None
    try:
        process.stdin.write(encoded)
        process.stdin.close()
    except (BrokenPipeError, OSError):
        pass


def call_rust(operation: str, input_data: Any, *, timeout: int = DEFAULT_TIMEOUT_SECONDS) -> Any:
    """Call a required Rust operation with credentials supplied only over stdin."""

    executable = os.environ.get("GRAPHLAYER_ENGINE", "")
    if not executable or not os.path.isabs(executable) or not os.path.isfile(executable):
        raise RustWorkerError("required Rust connector engine is unavailable")
    request = {
        "protocol_version": PROTOCOL_VERSION,
        "operation": operation,
        "input": input_data,
    }
    try:
        encoded = json.dumps(request, ensure_ascii=False, separators=(",", ":"),
                             allow_nan=False).encode("utf-8")
    except (TypeError, ValueError) as exc:
        raise RustWorkerError("connector request is not valid JSON") from exc
    if len(encoded) > MAX_REQUEST_BYTES:
        raise RustWorkerError("connector request is too large")
    try:
        command = _worker_command(executable)
        process = subprocess.Popen(
            command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL, shell=False,
        )
    except OSError as exc:
        raise RustWorkerError("required Rust connector engine failed") from exc

    output = bytearray()
    overflow = threading.Event()
    reader = threading.Thread(target=_read_bounded, args=(process, output, overflow), daemon=True)
    writer = threading.Thread(target=_write_request, args=(process, encoded), daemon=True)
    reader.start()
    writer.start()
    deadline = time.monotonic() + max(1, min(int(timeout), DEFAULT_TIMEOUT_SECONDS))
    timed_out = False
    try:
        process.wait(timeout=max(0, deadline - time.monotonic()))
    except subprocess.TimeoutExpired:
        timed_out = True
        try:
            process.terminate()
            process.wait(timeout=1)
        except (OSError, subprocess.TimeoutExpired):
            try:
                process.kill()
            except OSError:
                pass
            process.wait()
    finally:
        writer.join(timeout=1)
        reader.join(timeout=1)
        if process.poll() is None:
            try:
                process.kill()
            except OSError:
                pass
            process.wait()
    if timed_out:
        raise RustWorkerError("required Rust connector engine timed out")
    if overflow.is_set() or process.returncode != 0 or len(output) > MAX_RESPONSE_BYTES:
        raise RustWorkerError("required Rust connector engine returned an invalid response")
    try:
        response = json.loads(bytes(output), object_pairs_hook=_unique_object,
                              parse_constant=_reject_constant)
    except (UnicodeDecodeError, json.JSONDecodeError, ValueError) as exc:
        raise RustWorkerError("required Rust connector engine returned an invalid response") from exc
    if not isinstance(response, dict):
        raise RustWorkerError("required Rust connector engine returned an invalid response")
    if response.get("protocol_version") != PROTOCOL_VERSION or response.get("operation") != operation:
        raise RustWorkerError("required Rust connector engine returned an invalid response")
    if response.get("error"):
        raise RustWorkerError("Rust connector operation failed")
    if "result" not in response:
        raise RustWorkerError("required Rust connector engine returned an invalid response")
    return response["result"]
