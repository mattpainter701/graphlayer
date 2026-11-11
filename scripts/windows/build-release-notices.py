#!/usr/bin/env python3
"""Collect locked Windows dependency notices without executing either product binary."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys


class NoticeBuildError(RuntimeError):
    pass


def run(command: list[str], *, cwd: Path, env: dict[str, str], log: Path) -> str:
    result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, timeout=1200)
    log.write_bytes(result.stdout + b"\n" + result.stderr)
    if result.returncode:
        raise NoticeBuildError(f"Notice collection failed; inspect {log.name} in the notice work directory")
    return result.stdout.decode("utf-8", errors="strict")


def regular_file(path: Path) -> None:
    metadata = path.lstat()
    if not stat.S_ISREG(metadata.st_mode) or getattr(metadata, "st_file_attributes", 0) & 0x400:
        raise NoticeBuildError("Notice input must be a regular file")


def copy_notice(source: Path, destination: Path) -> None:
    regular_file(source)
    if source.stat().st_size == 0:
        raise NoticeBuildError("Upstream toolchain notice is empty")
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, destination)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-directory", type=Path, required=True)
    parser.add_argument("--work-directory", type=Path, required=True)
    parser.add_argument("--github-env", type=Path)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    work = args.work_directory.absolute()
    output = args.output_directory.absolute()
    if work.exists() or output.exists() or work == output or work in output.parents or output in work.parents:
        raise NoticeBuildError("Use separate new work and output directories")
    if any(char in str(output) for char in "\r\n"):
        raise NoticeBuildError("Notice output path must fit one environment line")
    versions = json.loads((repo / "config/licenses/tool-versions.json").read_text(encoding="utf-8"))
    if versions != {"cargo_about": "0.9.2", "go_licenses": "v2.0.1"}:
        raise NoticeBuildError("Notice collector tool pins must be explicitly reviewed before changing")
    work.mkdir(parents=True)
    tools = work / "tools"
    bin_dir = tools / "bin"
    env = os.environ.copy()
    # Build the collection tools for the runner host, then inspect the Windows target.
    for name in ("GOOS", "GOARCH", "GRAPHLAYER_VERSION_RESOURCE"):
        env.pop(name, None)
    env.update({"CARGO_TARGET_DIR": str(work / "tool-target"), "CARGO_BUILD_JOBS": "4", "GOBIN": str(bin_dir)})
    run(["cargo", "install", "cargo-about", "--version", versions["cargo_about"], "--locked", "--features", "cli", "--root", str(tools)], cwd=repo, env=env, log=work / "install-cargo-about.log")
    run(["go", "install", "github.com/google/go-licenses/v2@" + versions["go_licenses"]], cwd=repo / "platform", env=env, log=work / "install-go-licenses.log")
    suffix = ".exe" if os.name == "nt" else ""
    about = bin_dir / ("cargo-about" + suffix)
    go_licenses = bin_dir / ("go-licenses" + suffix)
    if run([str(about), "--version"], cwd=repo, env=env, log=work / "cargo-about-version.log").strip() != "cargo-about " + versions["cargo_about"]:
        raise NoticeBuildError("Unexpected cargo-about version")
    build_info = run(["go", "version", "-m", str(go_licenses)], cwd=repo, env=env, log=work / "go-licenses-version.log")
    if not any(line.split()[:3] == ["mod", "github.com/google/go-licenses/v2", versions["go_licenses"]] for line in build_info.splitlines()):
        raise NoticeBuildError("Unexpected go-licenses module version")
    cargo_report = work / "cargo.json"
    cargo_report.write_text(run([str(about), "generate", "--locked", "--fail", "--format", "json", "--manifest-path", "crates/graphlayer-engine/Cargo.toml", "--config", "config/licenses/cargo-about.toml"], cwd=repo, env=env, log=work / "cargo-about.log"), encoding="utf-8")
    target_env = env | {"GOOS": "windows", "GOARCH": "amd64", "CGO_ENABLED": "0"}
    go_report = work / "go.csv"
    own_module = "github.com/mattpainter701/graphlayer/platform"
    go_report.write_text(run([str(go_licenses), "report", "./cmd/graphlayer-investigate", "--ignore", own_module], cwd=repo / "platform", env=target_env, log=work / "go-report.log"), encoding="utf-8")
    materials = work / "go-materials"
    run([str(go_licenses), "save", "./cmd/graphlayer-investigate", "--ignore", own_module, "--save_path", str(materials)], cwd=repo / "platform", env=target_env, log=work / "go-save.log")
    goroot = Path(run(["go", "env", "GOROOT"], cwd=repo, env=env, log=work / "goroot.log").strip())
    go_toolchain = work / "go-toolchain"
    for name in ("LICENSE", "PATENTS"):
        copy_notice(goroot / name, go_toolchain / name)
    vendor = goroot / "src/vendor"
    for source in sorted(vendor.rglob("*")):
        if source.is_file() and source.name.upper().startswith(("LICENSE", "COPYING", "NOTICE", "PATENTS")):
            copy_notice(source, go_toolchain / "vendor" / source.relative_to(vendor))
    boring = goroot / "src/crypto/internal/boring/LICENSE"
    if boring.exists():
        copy_notice(boring, go_toolchain / "crypto-internal-boring/LICENSE")
    sysroot = Path(run(["rustc", "--print", "sysroot"], cwd=repo, env=env, log=work / "rust-sysroot.log").strip())
    stdlib = work / "rust-stdlib"
    copy_notice(sysroot / "share/doc/rust/COPYRIGHT-library.html", stdlib / "COPYRIGHT-library.html")
    license_files = sorted((sysroot / "share/doc/rust/licenses").glob("*.txt"))
    if not license_files:
        raise NoticeBuildError("Rust standard-library license bundle is missing")
    for source in license_files:
        copy_notice(source, stdlib / "licenses" / source.name)
    run([sys.executable, str(repo / "scripts/windows/collect-release-notices.py"), "--repo-root", str(repo), "--cargo-report", str(cargo_report), "--go-report", str(go_report), "--go-materials", str(materials), "--go-toolchain-notices", str(go_toolchain), "--rust-stdlib-notices", str(stdlib), "--output-directory", str(output)], cwd=repo, env=env, log=work / "collect.log")
    if args.github_env:
        with args.github_env.open("a", encoding="utf-8") as stream:
            stream.write(f"GRAPHLAYER_LICENSE_DIRECTORY={output}\n")
    print("Collected locked Windows notices; provider eligibility review remains separate.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (NoticeBuildError, OSError, UnicodeError, subprocess.TimeoutExpired) as error:
        print(f"Notice build failed: {error}", file=sys.stderr)
        raise SystemExit(1)
