import csv
import json
from pathlib import Path
import subprocess
import sys
import tomllib


ROOT = Path(__file__).resolve().parents[1]
COLLECTOR = ROOT / "scripts/windows/collect-release-notices.py"


def make_inputs(base: Path):
    repo = base / "repo"
    (repo / "config/licenses").mkdir(parents=True)
    (repo / "third_party/d3").mkdir(parents=True)
    (repo / "LICENSE").write_text("Apache license fixture\n", encoding="utf-8")
    (repo / "third_party/d3/LICENSE").write_text(
        "Copyright 2010-2023 Mike Bostock\n\nPermission to use, copy, modify\n", encoding="utf-8"
    )
    (repo / "platform/investigation/web").mkdir(parents=True)
    (repo / "platform/investigation/web/d3.min.js").write_text(
        "// https://d3js.org v7.9.0 Copyright 2010-2023 Mike Bostock\n", encoding="utf-8"
    )
    (repo / "config/licenses/license-policy.json").write_text(
        json.dumps({"accepted_spdx": ["MIT", "BSD-3-Clause"]}), encoding="utf-8"
    )
    cargo = base / "cargo.json"
    cargo.write_text(json.dumps({
        "licenses": [{"id": "MIT", "name": "MIT License", "text": "MIT full text\n", "used_by": [
            {"crate": {"name": "fixture-rust", "version": "1.2.3"}, "path": None}
        ]}],
        "crates": [{"package": {"name": "fixture-rust", "version": "1.2.3"}}],
    }), encoding="utf-8")
    go_report = base / "go.csv"
    with go_report.open("w", encoding="utf-8", newline="") as stream:
        csv.writer(stream, lineterminator="\n").writerow(
            ["example.invalid/fixture", "https://example.invalid/LICENSE", "BSD-3-Clause"]
        )
    go_materials = base / "go-materials"
    go_materials.mkdir()
    (go_materials / "example.invalid" / "fixture" / "LICENSE").parent.mkdir(parents=True)
    (go_materials / "example.invalid" / "fixture" / "LICENSE").write_text("Go license material\n", encoding="utf-8")
    go_toolchain = base / "go-toolchain"
    go_toolchain.mkdir()
    (go_toolchain / "LICENSE").write_text("Go toolchain license\n", encoding="utf-8")
    rust = base / "rust-stdlib"
    (rust / "licenses").mkdir(parents=True)
    (rust / "COPYRIGHT-library.html").write_text("<html>Rust library attribution</html>\n", encoding="utf-8")
    (rust / "licenses/MIT.txt").write_text("Rust toolchain MIT text\n", encoding="utf-8")
    return repo, cargo, go_report, go_materials, go_toolchain, rust


def collect(inputs, output: Path):
    repo, cargo, go, materials, go_toolchain, rust = inputs
    return subprocess.run([
        sys.executable, str(COLLECTOR), "--repo-root", str(repo),
        "--cargo-report", str(cargo), "--go-report", str(go),
        "--go-materials", str(materials), "--go-toolchain-notices", str(go_toolchain),
        "--rust-stdlib-notices", str(rust), "--output-directory", str(output),
    ], capture_output=True, text=True)


def test_outputs_are_complete_and_deterministic(tmp_path):
    inputs = make_inputs(tmp_path)
    first, second = tmp_path / "out-a", tmp_path / "out-b"
    assert collect(inputs, first).returncode == 0
    assert collect(inputs, second).returncode == 0
    assert sorted(p.name for p in first.iterdir()) == ["LICENSE.txt", "THIRD-PARTY-NOTICES.txt"]
    assert (first / "LICENSE.txt").read_bytes() == (inputs[0] / "LICENSE").read_bytes()
    assert (first / "THIRD-PARTY-NOTICES.txt").read_bytes() == (second / "THIRD-PARTY-NOTICES.txt").read_bytes()
    body = (first / "THIRD-PARTY-NOTICES.txt").read_text(encoding="utf-8")
    for text in ("fixture-rust 1.2.3", "example.invalid/fixture", "Go license material", "Rust library attribution", "Rust toolchain MIT text"):
        assert text in body


def test_unknown_cargo_license_fails_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    report = json.loads(inputs[1].read_text(encoding="utf-8"))
    report["licenses"][0]["id"] = "GPL-3.0-only"
    inputs[1].write_text(json.dumps(report), encoding="utf-8")
    result = collect(inputs, tmp_path / "out")
    assert result.returncode != 0
    assert "Unreviewed Rust dependency license" in result.stderr
    assert not (tmp_path / "out").exists()


def test_missing_cargo_attribution_fails_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    report = json.loads(inputs[1].read_text(encoding="utf-8"))
    report["licenses"][0]["used_by"] = []
    inputs[1].write_text(json.dumps(report), encoding="utf-8")
    result = collect(inputs, tmp_path / "out")
    assert result.returncode != 0
    assert "no dependency attribution" in result.stderr


def test_unknown_go_license_and_source_sharing_material_fail_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    inputs[2].write_text("fixture,https://example.invalid/LICENSE,Unknown\n", encoding="utf-8")
    result = collect(inputs, tmp_path / "out-unknown")
    assert result.returncode != 0
    assert "Unreviewed or incomplete Go dependency license" in result.stderr
    inputs = make_inputs(tmp_path / "second")
    (inputs[3] / "source.go").write_text("package fixture\n", encoding="utf-8")
    result = collect(inputs, tmp_path / "out-source")
    assert result.returncode != 0
    assert "Unexpected file in notice materials" in result.stderr


def test_unmapped_go_dependency_or_saved_notice_fails_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    with inputs[2].open("a", encoding="utf-8") as stream:
        stream.write("example.invalid/missing,https://example.invalid/LICENSE,MIT\n")
    result = collect(inputs, tmp_path / "out-uncovered")
    assert result.returncode != 0
    assert "Go dependency notice coverage mismatch" in result.stderr

    inputs = make_inputs(tmp_path / "second")
    (inputs[3] / "extra" / "LICENSE").parent.mkdir(parents=True)
    (inputs[3] / "extra" / "LICENSE").write_text("unmapped", encoding="utf-8")
    result = collect(inputs, tmp_path / "out-unmapped")
    assert result.returncode != 0
    assert "Go dependency notice coverage mismatch" in result.stderr


def test_tampered_or_missing_stdlib_text_fails_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    (inputs[5] / "COPYRIGHT-library.html").unlink()
    result = collect(inputs, tmp_path / "out")
    assert result.returncode != 0
    assert "Rust standard-library notice directory has unexpected files" in result.stderr


def test_tampered_d3_notice_and_policy_drift_fail_closed(tmp_path):
    inputs = make_inputs(tmp_path)
    (inputs[0] / "third_party/d3/LICENSE").write_text("Modified text\n", encoding="utf-8")
    result = collect(inputs, tmp_path / "out-d3")
    assert result.returncode != 0
    assert "D3 ISC license text is missing or unexpected" in result.stderr

    policy = json.loads((ROOT / "config/licenses/license-policy.json").read_text(encoding="utf-8"))
    cargo = tomllib.loads((ROOT / "config/licenses/cargo-about.toml").read_text(encoding="utf-8"))
    assert set(policy["accepted_spdx"]) == set(cargo["accepted"])


def test_existing_output_directory_is_never_overwritten(tmp_path):
    inputs = make_inputs(tmp_path)
    output = tmp_path / "out"
    output.mkdir()
    marker = output / "keep"
    marker.write_text("unchanged", encoding="utf-8")
    result = collect(inputs, output)
    assert result.returncode != 0
    assert marker.read_text(encoding="utf-8") == "unchanged"


def test_reparse_notice_directory_is_rejected_when_supported(tmp_path):
    inputs = make_inputs(tmp_path)
    linked = tmp_path / "linked-go-materials"
    try:
        linked.symlink_to(inputs[3], target_is_directory=True)
    except (OSError, NotImplementedError):
        import pytest
        pytest.skip("directory symlinks are not available on this host")
    inputs = (*inputs[:3], linked, *inputs[4:])
    result = collect(inputs, tmp_path / "out")
    assert result.returncode != 0
    assert "directory is missing or unsafe" in result.stderr
