#!/bin/bash
set -e

# Run packaging checks for the demo-data platform.
echo "=== Building package ==="
python -m build --sdist --wheel .

echo "=== Checking distribution ==="
if command -v twine &> /dev/null; then
    twine check dist/*
else
    echo "twine not installed; skipping distribution metadata check."
fi

echo "=== Packaging checks passed ==="
