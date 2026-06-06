#!/bin/bash
set -e

# Run packaging checks for the demo-data platform.
# This script validates distribution integrity before release.

echo "=== Step 1: Install build dependencies ==="
pip install --quiet build twine 2>/dev/null || true

echo "=== Step 2: Clean previous builds ==="
rm -rf dist/ build/ *.egg-info

echo "=== Step 3: Build source distribution and wheel ==="
python -m build --sdist --wheel .
echo "Build artifacts:"
ls -lh dist/

echo "=== Step 4: Check distribution metadata ==="
if command -v twine &> /dev/null; then
    twine check dist/*
else
    echo "twine not installed; skipping distribution metadata check."
fi

echo "=== Step 5: Lint package with pycodestyle ==="
if command -v pycodestyle &> /dev/null; then
    pycodestyle --max-line-length=100 platform/ tests/ scripts/smoke_test.py
else
    echo "pycodestyle not installed; skipping lint."
fi

echo "=== Step 6: Run import smoke test ==="
python scripts/smoke_test.py

echo "=== All packaging checks passed ==="
