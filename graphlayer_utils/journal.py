"""Tiny logging shim used by the legacy Python connector modules."""

from __future__ import annotations

import logging
from typing import Any


_LOGGER = logging.getLogger("graphlayer.connector")


def _log(level: str, message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    getattr(_LOGGER, level)(message, *args, extra=extra, **kwargs)


def debug(message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    _log("debug", message, *args, extra=extra, **kwargs)


def info(message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    _log("info", message, *args, extra=extra, **kwargs)


def warning(message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    _log("warning", message, *args, extra=extra, **kwargs)


def error(message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    _log("error", message, *args, extra=extra, **kwargs)


def exception(message: str, *args: Any, extra: dict | None = None, **kwargs: Any) -> None:
    _log("exception", message, *args, extra=extra, **kwargs)
