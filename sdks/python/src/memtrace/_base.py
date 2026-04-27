"""Shared client configuration and error handling."""

from __future__ import annotations

import httpx

from .exceptions import (
    AuthenticationError,
    ConflictError,
    MemtraceError,
    NoArcInstanceError,
    NotFoundError,
)

DEFAULT_TIMEOUT = 30.0


def build_headers(api_key: str) -> dict[str, str]:
    """Build default request headers."""
    return {
        "x-api-key": api_key,
        "Content-Type": "application/json",
    }


def handle_error(response: httpx.Response) -> None:
    """Raise an appropriate exception for error responses."""
    if response.status_code < 400:
        return

    # Server returns {"error": "..."} consistently
    message = ""
    try:
        body = response.json()
        message = body.get("error", "")
    except (ValueError, TypeError):
        message = response.text

    if not message:
        message = f"HTTP {response.status_code}"

    if response.status_code == 401:
        raise AuthenticationError(message)
    if response.status_code == 404:
        raise NotFoundError(message)
    if response.status_code == 409:
        raise ConflictError(message)
    if response.status_code == 503 and "arc instance" in message.lower():
        raise NoArcInstanceError(message)

    raise MemtraceError(message, status_code=response.status_code)
