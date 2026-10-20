#!/usr/bin/env python3
"""Run this revision's Linux Actions jobs on the dedicated GraphLayerCI WSL runner."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import signal
import subprocess
import sys
import time
import uuid
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Protocol
from urllib.parse import urlencode


REPOSITORY = "mattpainter701/graphlayer"
BRANCH = "codex/repository-privacy"
RUNNER_LABELS = ("self-hosted", "Linux", "X64", "graphlayer-private")
RUNNER_LABEL_SET = frozenset(label.casefold() for label in RUNNER_LABELS)
RUNNER_PREFIX = "graphlayer-private-"
EXPECTED_WORKFLOWS = (
    "Test & Coverage",
    "Rust engine and architecture",
    "Lint",
    "QA Docker Stack",
    "Performance Benchmarks",
)
EXPECTED_JOBS = {
    "test",
    "python-test",
    "browser-e2e",
    "connector-go",
    "connector-e2e",
    "rust-core",
    "rust-boundary",
    "rust-container",
    "vet-supported",
    "fuzz",
    "qa",
    "bench",
}
RUNNER_HOME = "/opt/graphlayer-runner"
RUN_JIT_SCRIPT = "/opt/graphlayer-tools/run-jit-runner.sh"
WSL_DISTRIBUTION = "GraphLayerCI"


class ControllerError(RuntimeError):
    """A safe-to-display controller error without command output or credentials."""


class Api(Protocol):
    def repository(self) -> dict[str, Any]: ...

    def branch(self, branch: str) -> dict[str, Any]: ...

    def workflow_runs(self, branch: str, head: str) -> list[dict[str, Any]]: ...

    def active_runs(self) -> list[dict[str, Any]]: ...

    def jobs(self, run_id: int) -> list[dict[str, Any]]: ...

    def runners(self) -> list[dict[str, Any]]: ...

    def generate_jit(self, payload: dict[str, Any]) -> dict[str, Any]: ...

    def delete_runner(self, runner_id: int) -> None: ...


class Runner(Protocol):
    def start(self, encoded_config: str, log_path: Path) -> Any: ...

    def poll(self, handle: Any) -> int | None: ...

    def wait(self, handle: Any, timeout: float) -> int: ...

    def stop(self, handle: Any) -> None: ...

    def clean_job_state(self) -> None: ...


class GhApi:
    """Use the already-authenticated Windows gh CLI; never print API responses."""

    def __init__(self, timeout: float = 45.0) -> None:
        self.timeout = timeout

    def _call(
        self,
        args: list[str],
        *,
        input_text: str | None = None,
        timeout: float | None = None,
    ) -> str:
        try:
            result = subprocess.run(
                ["gh", "api", *args],
                input=input_text,
                capture_output=True,
                text=True,
                timeout=timeout or self.timeout,
                check=False,
            )
        except (OSError, subprocess.TimeoutExpired) as exc:
            raise ControllerError("GitHub API command failed or timed out") from None
        if result.returncode != 0:
            raise ControllerError("GitHub API request failed")
        return result.stdout

    def _get(self, path: str, *, paginate: bool = False) -> Any:
        args = ["--method", "GET"]
        if paginate:
            args.extend(["--paginate", "--slurp"])
        args.append(path)
        output = self._call(args)
        try:
            return json.loads(output)
        except json.JSONDecodeError:
            raise ControllerError("GitHub API returned invalid JSON") from None

    @staticmethod
    def _pages(value: Any) -> list[dict[str, Any]]:
        if isinstance(value, dict):
            return [value]
        if not isinstance(value, list):
            raise ControllerError("GitHub API returned an unexpected response")
        pages: list[dict[str, Any]] = []
        for item in value:
            if isinstance(item, dict):
                pages.append(item)
            elif isinstance(item, list):
                pages.extend(entry for entry in item if isinstance(entry, dict))
        return pages

    @classmethod
    def _items(cls, value: Any, key: str) -> list[dict[str, Any]]:
        result: list[dict[str, Any]] = []
        for page in cls._pages(value):
            items = page.get(key)
            if not isinstance(items, list):
                continue
            result.extend(item for item in items if isinstance(item, dict))
        return result

    def repository(self) -> dict[str, Any]:
        value = self._get(f"repos/{REPOSITORY}")
        if not isinstance(value, dict):
            raise ControllerError("GitHub returned invalid repository metadata")
        return value

    def branch(self, branch: str) -> dict[str, Any]:
        value = self._get(f"repos/{REPOSITORY}/branches/{branch}")
        if not isinstance(value, dict):
            raise ControllerError("GitHub returned invalid branch metadata")
        return value

    def workflow_runs(self, branch: str, head: str) -> list[dict[str, Any]]:
        query = urlencode(
            {
                "branch": branch,
                "event": "push",
                "head_sha": head,
                "per_page": "100",
            }
        )
        value = self._get(
            f"repos/{REPOSITORY}/actions/runs?{query}", paginate=True
        )
        return self._items(value, "workflow_runs")

    def active_runs(self) -> list[dict[str, Any]]:
        runs: dict[int, dict[str, Any]] = {}
        for status in ("queued", "in_progress"):
            query = urlencode({"status": status, "per_page": "100"})
            value = self._get(
                f"repos/{REPOSITORY}/actions/runs?{query}", paginate=True
            )
            for run in self._items(value, "workflow_runs"):
                try:
                    runs[int(run["id"])] = run
                except (KeyError, TypeError, ValueError):
                    raise ControllerError("GitHub returned an invalid workflow run") from None
        return list(runs.values())

    def jobs(self, run_id: int) -> list[dict[str, Any]]:
        query = urlencode({"per_page": "100"})
        value = self._get(
            f"repos/{REPOSITORY}/actions/runs/{run_id}/jobs?{query}",
            paginate=True,
        )
        return self._items(value, "jobs")

    def runners(self) -> list[dict[str, Any]]:
        value = self._get(f"repos/{REPOSITORY}/actions/runners?per_page=100", paginate=True)
        return self._items(value, "runners")

    def generate_jit(self, payload: dict[str, Any]) -> dict[str, Any]:
        body = json.dumps(payload, separators=(",", ":"))
        output = self._call(
            [
                "--method",
                "POST",
                f"repos/{REPOSITORY}/actions/runners/generate-jitconfig",
                "--input",
                "-",
            ],
            input_text=body,
        )
        try:
            value = json.loads(output)
        except json.JSONDecodeError:
            raise ControllerError("GitHub returned invalid JIT runner metadata") from None
        if not isinstance(value, dict):
            raise ControllerError("GitHub returned invalid JIT runner metadata")
        return value

    def delete_runner(self, runner_id: int) -> None:
        path = f"repos/{REPOSITORY}/actions/runners/{runner_id}"
        try:
            result = subprocess.run(
                ["gh", "api", "--include", "--method", "DELETE", path],
                capture_output=True,
                text=True,
                timeout=self.timeout,
                check=False,
            )
        except (OSError, subprocess.TimeoutExpired):
            raise ControllerError("Could not remove the JIT runner registration") from None
        if result.returncode == 0:
            return
        if re.search(r"^HTTP/\S+\s+404\b", result.stdout, re.MULTILINE):
            return
        raise ControllerError("Could not remove the JIT runner registration")


@dataclass(frozen=True)
class JobRecord:
    run_id: int
    workflow: str
    name: str
    status: str
    conclusion: str | None
    labels: tuple[str, ...]
    runner_name: str | None


@dataclass(frozen=True)
class Snapshot:
    jobs: dict[str, JobRecord]
    run_ids: frozenset[int]


@dataclass
class RunnerHandle:
    process: subprocess.Popen[bytes]
    pidfile: str
    log_handle: Any


_WSL_WRAPPER = r"""
import os, subprocess, sys
pidfile, runner = sys.argv[1:]
fd = os.open(pidfile, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
try:
    child = subprocess.Popen(
        [runner], cwd="/opt/graphlayer-runner", stdin=sys.stdin.buffer,
        stdout=sys.stdout.buffer, stderr=sys.stderr.buffer, start_new_session=True
    )
    os.write(fd, (str(child.pid) + "\n").encode("ascii"))
    os.fsync(fd)
finally:
    os.close(fd)
try:
    result = child.wait()
finally:
    try:
        os.unlink(pidfile)
    except FileNotFoundError:
        pass
sys.exit(result)
"""

_WSL_STOP = r"""
import os, signal, sys
pidfile, signal_name = sys.argv[1:]
try:
    with open(pidfile, "r", encoding="ascii") as source:
        pid = int(source.read().strip())
except FileNotFoundError:
    sys.exit(0)
except (OSError, ValueError):
    sys.exit(3)
if pid <= 1 or os.getpgid(pid) != pid:
    sys.exit(4)
try:
    with open(f"/proc/{pid}/cmdline", "rb") as source:
        command = source.read()
except OSError:
    sys.exit(0)
if b"run-jit-runner.sh" not in command and b"run.sh" not in command:
    sys.exit(5)
try:
    os.killpg(pid, getattr(signal, signal_name))
except ProcessLookupError:
    pass
"""

_WSL_CLEAN = r"""
import os, pwd, shutil, stat, sys
root = "/opt/graphlayer-runner"
if os.path.islink(root) or os.path.realpath(root) != root or not os.path.isdir(root):
    sys.exit(10)
work = os.path.join(root, "_work")
for directory in (work, os.path.join(root, "_diag")):
    if os.path.lexists(directory):
        if os.path.islink(directory) or os.path.realpath(directory) != directory or not os.path.isdir(directory):
            sys.exit(11)
        shutil.rmtree(directory)
for name in (".runner", ".credentials", ".credentials_rsaparams", ".env", ".path"):
    target = os.path.join(root, name)
    if not os.path.lexists(target):
        continue
    if os.path.islink(target) or not os.path.isdir(target):
        os.unlink(target)
    else:
        if os.path.realpath(target) != target:
            sys.exit(12)
        shutil.rmtree(target)
account = pwd.getpwnam("graphlayer")
os.umask(0o077)
os.mkdir(work, 0o700)
os.chown(work, account.pw_uid, account.pw_gid)
"""


class WslRunner:
    def __init__(self, executable: str = "wsl.exe") -> None:
        self.executable = executable

    def _command(self, *args: str, timeout: float = 30.0) -> subprocess.CompletedProcess[bytes]:
        try:
            result = subprocess.run(
                [
                    self.executable,
                    "--distribution",
                    WSL_DISTRIBUTION,
                    "--cd",
                    "/",
                    *args,
                ],
                capture_output=True,
                timeout=timeout,
                check=False,
            )
        except (OSError, subprocess.TimeoutExpired):
            raise ControllerError("WSL runner command failed or timed out") from None
        if result.returncode != 0:
            raise ControllerError("WSL runner command failed")
        return result

    def start(self, encoded_config: str, log_path: Path) -> RunnerHandle:
        if not encoded_config or "\n" in encoded_config or "\r" in encoded_config:
            raise ControllerError("GitHub returned an invalid encoded JIT configuration")
        token = uuid.uuid4().hex
        pidfile = f"/tmp/graphlayer-runner-{token}.pid"
        log_handle = log_path.open("xb")
        command = [
            self.executable,
            "--distribution",
            WSL_DISTRIBUTION,
            "--cd",
            "/",
            "--user",
            "graphlayer",
            "--exec",
            "/usr/bin/python3",
            "-c",
            _WSL_WRAPPER,
            pidfile,
            RUN_JIT_SCRIPT,
        ]
        try:
            process = subprocess.Popen(
                command,
                stdin=subprocess.PIPE,
                stdout=log_handle,
                stderr=subprocess.STDOUT,
                creationflags=getattr(subprocess, "CREATE_NEW_PROCESS_GROUP", 0),
            )
            assert process.stdin is not None
            process.stdin.write(encoded_config.encode("ascii") + b"\n")
            process.stdin.close()
        except (OSError, UnicodeEncodeError, BrokenPipeError):
            if "process" in locals() and process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
            log_handle.close()
            raise ControllerError("Could not start the dedicated WSL runner") from None
        return RunnerHandle(process=process, pidfile=pidfile, log_handle=log_handle)

    @staticmethod
    def poll(handle: RunnerHandle) -> int | None:
        return handle.process.poll()

    @staticmethod
    def wait(handle: RunnerHandle, timeout: float) -> int:
        try:
            return handle.process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            raise ControllerError("WSL runner did not exit after its job completed") from None

    def stop(self, handle: RunnerHandle) -> None:
        if handle.process.poll() is None:
            self._command(
                "--user",
                "graphlayer",
                "--exec",
                "/usr/bin/python3",
                "-c",
                _WSL_STOP,
                handle.pidfile,
                "SIGTERM",
            )
            try:
                handle.process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                # The WSL-side pid is verified and only that process group is targeted.
                self._command(
                    "--user",
                    "graphlayer",
                    "--exec",
                    "/usr/bin/python3",
                    "-c",
                    _WSL_STOP,
                    handle.pidfile,
                    "SIGKILL",
                )
                try:
                    handle.process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    raise ControllerError("Could not stop the dedicated WSL runner") from None
        handle.log_handle.close()

    def clean_job_state(self) -> None:
        self._command(
            "--user",
            "root",
            "--exec",
            "/usr/bin/python3",
            "-c",
            _WSL_CLEAN,
            timeout=60.0,
        )


class LocalCIRunnerController:
    def __init__(
        self,
        head: str,
        *,
        api: Api,
        runner: Runner,
        runtime_dir: Path,
        checkout: Path,
        max_jobs: int = 12,
        job_timeout_minutes: int = 120,
        poll_seconds: float = 30.0,
        sleep=time.sleep,
        monotonic=time.monotonic,
        emit=print,
    ) -> None:
        if not re.fullmatch(r"[0-9a-fA-F]{40}", head):
            raise ControllerError("--head must be a 40-character commit SHA")
        if not 1 <= max_jobs <= len(EXPECTED_JOBS):
            raise ControllerError("--max-jobs must be between 1 and 12")
        if job_timeout_minutes < 1:
            raise ControllerError("--job-timeout-minutes must be positive")
        self.head = head.lower()
        self.prefix = f"{RUNNER_PREFIX}{self.head[:8]}-"
        self.api = api
        self.runner = runner
        self.runtime_dir = runtime_dir
        self.checkout = checkout.resolve()
        self.max_jobs = max_jobs
        self.job_timeout = job_timeout_minutes * 60
        self.poll_seconds = poll_seconds
        self.sleep = sleep
        self.monotonic = monotonic
        self.emit = emit
        self._validate_runtime_dir()

    def _validate_runtime_dir(self) -> None:
        runtime = self.runtime_dir.expanduser().resolve()
        if runtime == self.checkout or self.checkout in runtime.parents:
            raise ControllerError("--runtime-dir must be outside the Git checkout")
        for ancestor in (runtime, *runtime.parents):
            if (ancestor / ".git").exists():
                raise ControllerError("--runtime-dir must be outside every Git worktree")
            if (
                (ancestor / "HEAD").is_file()
                and (ancestor / "objects").is_dir()
                and (ancestor / "refs").is_dir()
            ):
                raise ControllerError("--runtime-dir must be outside every bare Git repository")
        self.runtime_dir = runtime

    def _log(self, event: str, **metadata: Any) -> None:
        fields = " ".join(f"{key}={value}" for key, value in metadata.items())
        self.emit(f"[local-ci] {event}" + (f" {fields}" if fields else ""))

    @staticmethod
    def _run_id(run: dict[str, Any]) -> int:
        try:
            return int(run["id"])
        except (KeyError, TypeError, ValueError):
            raise ControllerError("GitHub returned a workflow run without a valid ID") from None

    @staticmethod
    def _label_set(job: dict[str, Any]) -> frozenset[str]:
        labels = job.get("labels")
        if not isinstance(labels, list) or not all(isinstance(x, str) for x in labels):
            return frozenset()
        return frozenset(label.casefold() for label in labels)

    @classmethod
    def _can_match_runner(cls, job: dict[str, Any]) -> bool:
        labels = cls._label_set(job)
        return bool(labels) and labels <= RUNNER_LABEL_SET

    def _trusted_runs(self) -> dict[str, dict[str, Any]]:
        repo = self.api.repository()
        if repo.get("full_name", "").casefold() != REPOSITORY.casefold():
            raise ControllerError("Authenticated GitHub repository is not the expected repository")
        if repo.get("private") is not True:
            raise ControllerError("Local runner is allowed only while the repository is private")

        branch = self.api.branch(BRANCH)
        branch_sha = branch.get("commit", {}).get("sha") if isinstance(branch.get("commit"), dict) else None
        if not isinstance(branch_sha, str) or branch_sha.lower() != self.head:
            raise ControllerError("Remote privacy branch head does not match --head")

        matches = self.api.workflow_runs(BRANCH, self.head)
        by_name: dict[str, dict[str, Any]] = {}
        for run in matches:
            if (
                run.get("head_branch") != BRANCH
                or run.get("event") != "push"
                or str(run.get("head_sha", "")).lower() != self.head
            ):
                continue
            repository = run.get("head_repository")
            if not isinstance(repository, dict) or str(repository.get("full_name", "")).casefold() != REPOSITORY.casefold():
                raise ControllerError("Workflow run did not originate from the expected repository")
            name = run.get("name")
            if name not in EXPECTED_WORKFLOWS:
                raise ControllerError("Unexpected workflow run matched the requested revision")
            if name in by_name:
                raise ControllerError("Multiple workflow runs matched the requested revision")
            by_name[name] = run

        missing = set(EXPECTED_WORKFLOWS) - set(by_name)
        if missing:
            raise ControllerError("Expected workflow runs for this push are not all available")
        return by_name

    def _job_record(self, run: dict[str, Any], item: dict[str, Any]) -> JobRecord:
        name = item.get("name")
        status = item.get("status")
        if not isinstance(name, str) or not isinstance(status, str):
            raise ControllerError("GitHub returned invalid job metadata")
        labels_value = item.get("labels", [])
        labels = tuple(str(label) for label in labels_value) if isinstance(labels_value, list) else ()
        runner_name = item.get("runner_name")
        try:
            run_id = self._run_id(run)
        except ControllerError:
            raise
        return JobRecord(
            run_id=run_id,
            workflow=str(run.get("name", "")),
            name=name,
            status=status,
            conclusion=item.get("conclusion") if isinstance(item.get("conclusion"), str) else None,
            labels=labels,
            runner_name=runner_name if isinstance(runner_name, str) else None,
        )

    def _snapshot(self, current_runner: str | None = None) -> Snapshot:
        trusted_runs = self._trusted_runs()
        run_ids = frozenset(self._run_id(run) for run in trusted_runs.values())
        jobs: dict[str, JobRecord] = {}
        for run in trusted_runs.values():
            for item in self.api.jobs(self._run_id(run)):
                record = self._job_record(run, item)
                if record.name not in EXPECTED_JOBS:
                    if self._can_match_runner(item):
                        raise ControllerError("Unexpected job can claim the dedicated runner")
                    continue
                if record.name in jobs:
                    raise ControllerError("Duplicate Linux job name in the workflow runs")
                if frozenset(label.casefold() for label in record.labels) != RUNNER_LABEL_SET:
                    raise ControllerError("Linux job runner labels do not match the dedicated runner")
                jobs[record.name] = record

        if set(jobs) != EXPECTED_JOBS:
            raise ControllerError("The exact set of 12 Linux jobs is not present")

        for run in self.api.active_runs():
            run_id = self._run_id(run)
            if run_id in run_ids:
                run_records = [job for job in jobs.values() if job.run_id == run_id]
                raw_jobs = None
            else:
                raw_jobs = self.api.jobs(run_id)
                run_records = []
            if raw_jobs is None:
                candidate_jobs = [
                    {
                        "name": job.name,
                        "status": job.status,
                        "labels": list(job.labels),
                        "runner_name": job.runner_name,
                    }
                    for job in run_records
                ]
            else:
                candidate_jobs = raw_jobs
            for item in candidate_jobs:
                status = item.get("status")
                if status not in {"queued", "in_progress"} or not self._can_match_runner(item):
                    continue
                if run_id not in run_ids:
                    raise ControllerError("Another queued job can claim the dedicated runner")
                job_name = item.get("name")
                if job_name not in EXPECTED_JOBS:
                    raise ControllerError("Unexpected queued job can claim the dedicated runner")
                if status == "in_progress" and item.get("runner_name") != current_runner:
                    raise ControllerError("A different runner already owns a Linux job")

        for name, record in jobs.items():
            if record.status == "completed":
                if record.conclusion != "success":
                    raise ControllerError(f"Linux job {name} completed with a non-success conclusion")
                if not record.runner_name or not record.runner_name.startswith(self.prefix):
                    raise ControllerError(f"Linux job {name} did not use this revision's JIT runner")
            elif record.status == "in_progress":
                if record.runner_name != current_runner:
                    raise ControllerError(f"Linux job {name} is already running on another runner")
        return Snapshot(jobs=jobs, run_ids=run_ids)

    def _validate_no_online_runner(self) -> None:
        # A queued job must not be claimable by a stale or separately managed runner.
        method = getattr(self.api, "runners", None)
        if method is None:
            return
        for item in method():
            if item.get("status") != "online":
                continue
            labels = item.get("labels", [])
            if not isinstance(labels, list):
                continue
            label_names = {
                label.get("name", "").casefold()
                for label in labels
                if isinstance(label, dict) and isinstance(label.get("name"), str)
            }
            if any(labels and labels <= label_names for labels in self._pending_label_sets):
                raise ControllerError("Another online runner can claim Linux jobs")

    @staticmethod
    def _jit_runner_id(value: dict[str, Any], expected_name: str) -> int:
        runner = value.get("runner")
        if not isinstance(runner, dict) or runner.get("name") != expected_name:
            raise ControllerError("GitHub returned invalid JIT runner identity")
        try:
            runner_id = int(runner["id"])
        except (KeyError, TypeError, ValueError):
            raise ControllerError("GitHub returned invalid JIT runner identity") from None
        if runner_id <= 0:
            raise ControllerError("GitHub returned invalid JIT runner identity")
        return runner_id

    @staticmethod
    def _jit_configuration(value: dict[str, Any]) -> str:
        encoded = value.get("encoded_jit_config")
        if not isinstance(encoded, str) or not encoded or "\n" in encoded or "\r" in encoded:
            raise ControllerError("GitHub returned invalid JIT configuration")
        return encoded

    def _wait_for_job(self, handle: Any, runner_name: str) -> JobRecord:
        deadline = self.monotonic() + self.job_timeout
        first_exit_at: float | None = None
        last_state: tuple[str, str, str] | None = None
        while self.monotonic() < deadline:
            snapshot = self._snapshot(current_runner=runner_name)
            assigned = [job for job in snapshot.jobs.values() if job.runner_name == runner_name]
            if len(assigned) > 1:
                raise ControllerError("JIT runner was assigned more than one job")
            if assigned:
                job = assigned[0]
                state = (job.workflow, job.name, job.status)
                if state != last_state:
                    self._log("job_state", workflow=job.workflow, job=job.name, status=job.status)
                    last_state = state
                if job.status == "completed":
                    if job.conclusion != "success":
                        raise ControllerError(f"Linux job {job.name} failed")
                    return job
            exit_code = self.runner.poll(handle)
            if exit_code is not None:
                if first_exit_at is None:
                    first_exit_at = self.monotonic()
                if assigned and assigned[0].status == "completed":
                    if exit_code != 0:
                        raise ControllerError("JIT runner process exited unsuccessfully")
                    return assigned[0]
                if self.monotonic() - first_exit_at > 30:
                    raise ControllerError("JIT runner exited without completing a tracked Linux job")
            self.sleep(self.poll_seconds)
        raise ControllerError("Linux job exceeded the configured timeout")

    def _run_one(self, snapshot: Snapshot) -> JobRecord:
        # Refresh the repository, branch, run set, labels, and queue immediately
        # before creating a JIT registration.
        snapshot = self._snapshot()
        queued = [job for job in snapshot.jobs.values() if job.status == "queued"]
        if not queued:
            raise ControllerError("No queued Linux job is available for the JIT runner")
        self._pending_label_sets = [
            frozenset(label.casefold() for label in job.labels) for job in queued
        ]
        self._validate_no_online_runner()
        name = f"{self.prefix}{uuid.uuid4()}"
        payload = {
            "name": name,
            "runner_group_id": 1,
            "work_folder": "_work",
            "labels": list(RUNNER_LABELS),
        }
        jit = self.api.generate_jit(payload)
        runner_id: int | None = None
        handle: Any | None = None
        primary_error: BaseException | None = None
        try:
            runner_id = self._jit_runner_id(jit, name)
            encoded = self._jit_configuration(jit)
            self._log("runner_registered", name=name, queued_jobs=len(queued))
            self.runtime_dir.mkdir(parents=True, exist_ok=True)
            log_path = self.runtime_dir / f"{name}.log"
            handle = self.runner.start(encoded, log_path)
            job = self._wait_for_job(handle, name)
            try:
                code = self.runner.wait(handle, 30.0)
            except ControllerError:
                self.runner.stop(handle)
                raise
            if code != 0:
                raise ControllerError("JIT runner process exited unsuccessfully")
            self._log("job_completed", workflow=job.workflow, job=job.name, conclusion="success")
            return job
        except BaseException as exc:
            primary_error = exc
            raise
        finally:
            cleanup_error: ControllerError | None = None
            if handle is not None:
                try:
                    if self.runner.poll(handle) is None:
                        self.runner.stop(handle)
                    else:
                        # Close the local log handle after the WSL process has exited.
                        log_handle = getattr(handle, "log_handle", None)
                        if log_handle is not None and not log_handle.closed:
                            log_handle.close()
                except ControllerError as exc:
                    cleanup_error = exc
            if runner_id is not None:
                try:
                    self.api.delete_runner(runner_id)
                    self._log("runner_unregistered", id=runner_id)
                except ControllerError as exc:
                    cleanup_error = cleanup_error or exc
            process_stopped = handle is None or self.runner.poll(handle) is not None
            if process_stopped:
                try:
                    self.runner.clean_job_state()
                    self._log("job_state_cleaned")
                except ControllerError as exc:
                    cleanup_error = cleanup_error or exc
            elif cleanup_error is None:
                cleanup_error = ControllerError("Runner remains active; workspace was not cleaned")
            if cleanup_error is not None and primary_error is None:
                raise cleanup_error
            if cleanup_error is not None and primary_error is not None:
                self._log("cleanup_incomplete")

    def run(self) -> None:
        self._log("start", repository=REPOSITORY, branch=BRANCH, head=self.head)
        launched = 0
        wait_deadline = self.monotonic() + self.job_timeout
        while True:
            snapshot = self._snapshot()
            completed = sum(job.status == "completed" for job in snapshot.jobs.values())
            if completed == len(EXPECTED_JOBS):
                self._log("all_linux_jobs_passed", count=completed)
                return
            if launched >= self.max_jobs:
                raise ControllerError("--max-jobs limit reached before all 12 Linux jobs passed")
            if any(job.status == "queued" for job in snapshot.jobs.values()):
                self._run_one(snapshot)
                launched += 1
                wait_deadline = self.monotonic() + self.job_timeout
                continue
            if self.monotonic() >= wait_deadline:
                raise ControllerError("Timed out waiting for the next Linux job")
            self.sleep(self.poll_seconds)


def default_runtime_dir() -> Path:
    local_app_data = os.environ.get("LOCALAPPDATA")
    if local_app_data:
        return Path(local_app_data) / "GraphLayer" / "runner-controller"
    return Path.home() / "AppData" / "Local" / "GraphLayer" / "runner-controller"


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--head", required=True, help="exact 40-character privacy branch commit SHA")
    parser.add_argument("--max-jobs", type=int, default=12, help="maximum JIT registrations in this invocation")
    parser.add_argument("--job-timeout-minutes", type=int, default=120)
    parser.add_argument("--runtime-dir", type=Path, default=default_runtime_dir())
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    checkout = Path(__file__).resolve().parents[2]
    try:
        controller = LocalCIRunnerController(
            args.head,
            api=GhApi(),
            runner=WslRunner(),
            runtime_dir=args.runtime_dir,
            checkout=checkout,
            max_jobs=args.max_jobs,
            job_timeout_minutes=args.job_timeout_minutes,
        )
        controller.run()
    except KeyboardInterrupt:
        print("[local-ci] cancelled", file=sys.stderr)
        return 130
    except ControllerError as exc:
        print(f"[local-ci] error: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
