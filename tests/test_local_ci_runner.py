"""Security checks for the private local Actions runner controller."""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "ci" / "run-local-ci.py"
SPEC = importlib.util.spec_from_file_location("run_local_ci", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
local_ci = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = local_ci
SPEC.loader.exec_module(local_ci)

HEAD = "a" * 40
WORKFLOW_JOBS = {
    "Test & Coverage": [
        "test",
        "python-test",
        "browser-e2e",
        "connector-go",
        "connector-e2e",
    ],
    "Rust engine and architecture": ["rust-core", "rust-boundary", "rust-container"],
    "Lint": ["vet-supported", "fuzz"],
    "QA Docker Stack": ["qa"],
    "Performance Benchmarks": ["bench"],
}


def workflow_run(run_id: int, name: str, *, event: str = "push", head: str = HEAD,
                 branch: str = local_ci.BRANCH) -> dict:
    return {
        "id": run_id,
        "name": name,
        "head_branch": branch,
        "event": event,
        "head_sha": head,
        "head_repository": {"full_name": local_ci.REPOSITORY},
        "status": "in_progress",
    }


def linux_job(name: str, *, labels=None, status="queued") -> dict:
    return {
        "name": name,
        "status": status,
        "conclusion": None,
        "labels": list(labels if labels is not None else local_ci.RUNNER_LABELS),
        "runner_name": None,
    }


class FakeApi:
    def __init__(self) -> None:
        self.private = True
        self.branch_sha = HEAD
        self.branch_calls: list[str] = []
        self.workflow_run_calls: list[tuple[str, str]] = []
        self.target_runs = [
            workflow_run(index + 10, name)
            for index, name in enumerate(local_ci.EXPECTED_WORKFLOWS)
        ]
        self.jobs_by_run = {
            run["id"]: [linux_job(job_name) for job_name in WORKFLOW_JOBS[run["name"]]]
            for run in self.target_runs
        }
        # The Windows job has labels that cannot be matched by the Linux runner.
        self.jobs_by_run[self.target_runs[0]["id"]].append(
            linux_job("windows-native-go", labels=["windows-latest"])
        )
        self.extra_runs: list[dict] = []
        self.extra_jobs: dict[int, list[dict]] = {}
        self.generate_calls: list[dict] = []
        self.delete_calls: list[int] = []

    def repository(self):
        return {"full_name": local_ci.REPOSITORY, "private": self.private}

    def branch(self, branch):
        self.branch_calls.append(branch)
        return {"name": branch, "commit": {"sha": self.branch_sha}}

    def workflow_runs(self, branch, head):
        self.workflow_run_calls.append((branch, head))
        return list(self.target_runs)

    def active_runs(self):
        return [*self.target_runs, *self.extra_runs]

    def jobs(self, run_id):
        return [*self.jobs_by_run.get(run_id, []), *self.extra_jobs.get(run_id, [])]

    def runners(self):
        return []

    def generate_jit(self, payload):
        self.generate_calls.append(payload)
        return {
            "runner": {"id": 123, "name": payload["name"]},
            "encoded_jit_config": "short-lived-jit-value",
        }

    def delete_runner(self, runner_id):
        self.delete_calls.append(runner_id)


class FakeRunner:
    def __init__(self) -> None:
        self.started = 0
        self.cleaned = 0

    def start(self, encoded_config, log_path):
        self.started += 1
        raise AssertionError("runner must not start for rejected input")

    def poll(self, handle):
        return 0

    def wait(self, handle, timeout):
        return 0

    def stop(self, handle):
        pass

    def clean_job_state(self):
        self.cleaned += 1


class LocalCIRunnerSecurityTests(unittest.TestCase):
    def make_controller(self, api: FakeApi, root: Path, *, branch=local_ci.BRANCH) -> tuple[local_ci.LocalCIRunnerController, FakeRunner]:
        runner = FakeRunner()
        controller = local_ci.LocalCIRunnerController(
            HEAD,
            branch=branch,
            api=api,
            runner=runner,
            runtime_dir=root / "runtime",
            checkout=root / "checkout",
            sleep=lambda _: None,
            monotonic=lambda: 0.0,
            emit=lambda _: None,
        )
        return controller, runner

    def test_public_repository_is_rejected_before_workflow_or_jit_calls(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            api.private = False
            controller, runner = self.make_controller(api, Path(directory))

            with self.assertRaisesRegex(local_ci.ControllerError, "only while the repository is private"):
                controller.run()

            self.assertEqual(api.generate_calls, [])
            self.assertEqual(runner.started, 0)

    def test_remote_branch_head_drift_is_rejected_before_jit(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            api.branch_sha = "b" * 40
            controller, runner = self.make_controller(api, Path(directory))

            with self.assertRaisesRegex(local_ci.ControllerError, "branch head does not match"):
                controller.run()

            self.assertEqual(api.generate_calls, [])
            self.assertEqual(runner.started, 0)

    def test_main_is_accepted_when_branch_head_and_push_runs_match_exact_sha(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            api.target_runs = [
                workflow_run(run["id"], run["name"], branch="main")
                for run in api.target_runs
            ]
            controller, _ = self.make_controller(api, Path(directory), branch="main")

            trusted = controller._trusted_runs()

            self.assertEqual(set(trusted), set(local_ci.EXPECTED_WORKFLOWS))
            self.assertEqual(api.branch_calls, ["main"])
            self.assertEqual(api.workflow_run_calls, [("main", HEAD)])

    def test_reviewed_recovery_branch_requires_its_exact_head_and_push_runs(self):
        branch = "codex/windows-worker-release-recovery"
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            api.target_runs = [
                workflow_run(run["id"], run["name"], branch=branch)
                for run in api.target_runs
            ]
            controller, _ = self.make_controller(api, Path(directory), branch=branch)
            self.assertEqual(set(controller._trusted_runs()), set(local_ci.EXPECTED_WORKFLOWS))
            self.assertEqual(api.branch_calls, [branch])
            api.branch_sha = "b" * 40
            with self.assertRaisesRegex(local_ci.ControllerError, "branch head does not match"):
                controller._trusted_runs()
            self.assertEqual(api.generate_calls, [])

    def test_unreviewed_branch_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            with self.assertRaisesRegex(local_ci.ControllerError, "--branch must be main"):
                self.make_controller(api, Path(directory), branch="release")
            self.assertEqual(api.branch_calls, [])
            self.assertEqual(api.workflow_run_calls, [])

    def test_main_rejects_workflow_runs_from_privacy_branch(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()  # Its runs are on codex/repository-privacy, not main.
            controller, runner = self.make_controller(api, Path(directory), branch="main")

            with self.assertRaisesRegex(local_ci.ControllerError, "workflow runs for this push"):
                controller.run()

            self.assertEqual(api.branch_calls, ["main"])
            self.assertEqual(api.workflow_run_calls, [("main", HEAD)])
            self.assertEqual(api.generate_calls, [])
            self.assertEqual(runner.started, 0)

    def test_wrong_queued_run_event_or_head_cannot_claim_the_runner(self):
        cases = (
            ("pull_request", HEAD),
            ("push", "b" * 40),
        )
        for event, head in cases:
            with self.subTest(event=event, head=head[:8]), tempfile.TemporaryDirectory() as directory:
                api = FakeApi()
                outside = workflow_run(99, "Untrusted queued run", event=event, head=head)
                api.extra_runs.append(outside)
                api.extra_jobs[99] = [linux_job("untrusted-job")]
                controller, runner = self.make_controller(api, Path(directory))

                with self.assertRaisesRegex(local_ci.ControllerError, "Another queued job"):
                    controller.run()

                self.assertEqual(api.generate_calls, [])
                self.assertEqual(runner.started, 0)

    def test_partial_linux_runner_labels_fail_before_jit(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            api.jobs_by_run[10][0]["labels"] = ["self-hosted", "Linux", "X64"]
            controller, runner = self.make_controller(api, Path(directory))

            with self.assertRaisesRegex(local_ci.ControllerError, "labels do not match"):
                controller.run()

            self.assertEqual(api.generate_calls, [])
            self.assertEqual(runner.started, 0)

    def test_custom_label_only_queued_job_can_claim_the_runner(self):
        with tempfile.TemporaryDirectory() as directory:
            api = FakeApi()
            outside = workflow_run(99, "Untrusted queued run", event="pull_request")
            api.extra_runs.append(outside)
            api.extra_jobs[99] = [linux_job("untrusted-job", labels=["graphlayer-private"])]
            controller, runner = self.make_controller(api, Path(directory))

            with self.assertRaisesRegex(local_ci.ControllerError, "Another queued job"):
                controller.run()

            self.assertEqual(api.generate_calls, [])
            self.assertEqual(runner.started, 0)

    def test_runtime_directory_inside_checkout_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            runner = FakeRunner()

            with self.assertRaisesRegex(local_ci.ControllerError, "outside the Git checkout"):
                local_ci.LocalCIRunnerController(
                    HEAD,
                    api=FakeApi(),
                    runner=runner,
                    runtime_dir=root / "checkout" / "logs",
                    checkout=root / "checkout",
                )

    def test_runtime_directory_inside_another_worktree_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            worktree = root / "other-worktree"
            worktree.mkdir()
            (worktree / ".git").write_text("gitdir: ../gitdir\n", encoding="utf-8")
            runtime = worktree / "controller-logs"
            runner = FakeRunner()

            with self.assertRaisesRegex(local_ci.ControllerError, "every Git worktree"):
                local_ci.LocalCIRunnerController(
                    HEAD,
                    api=FakeApi(),
                    runner=runner,
                    runtime_dir=runtime,
                    checkout=root / "checkout",
                )

    def test_runtime_directory_inside_bare_git_repository_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bare = root / "bare.git"
            bare.mkdir()
            (bare / "HEAD").write_text("ref: refs/heads/main\n", encoding="utf-8")
            (bare / "objects").mkdir()
            (bare / "refs").mkdir()
            runner = FakeRunner()

            with self.assertRaisesRegex(local_ci.ControllerError, "bare Git repository"):
                local_ci.LocalCIRunnerController(
                    HEAD,
                    api=FakeApi(),
                    runner=runner,
                    runtime_dir=bare / "runner-logs",
                    checkout=root / "checkout",
                )


if __name__ == "__main__":
    unittest.main()
