"""GraphLayer platform package.

This package intentionally shares its top-level name with the stdlib
``platform`` module because the existing connector tests import
``platform.connectors``. To keep the standard library behavior available for
third-party tools like ``pytest``/``uuid``, we proxy the stdlib module's public
API into this package namespace.
"""

from __future__ import annotations

import importlib.util
import sysconfig
from pathlib import Path


def _load_stdlib_platform():
    stdlib_path = Path(sysconfig.get_path("stdlib")) / "platform.py"
    spec = importlib.util.spec_from_file_location("_stdlib_platform", stdlib_path)
    if spec is None or spec.loader is None:  # pragma: no cover - environment specific
        raise ImportError(f"unable to load stdlib platform module from {stdlib_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


_stdlib_platform = _load_stdlib_platform()
for _name in dir(_stdlib_platform):
    if _name.startswith("_"):
        continue
    globals()[_name] = getattr(_stdlib_platform, _name)

__all__ = [name for name in dir(_stdlib_platform) if not name.startswith("_")]

