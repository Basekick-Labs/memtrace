"""Memtrace SDK exceptions."""


class MemtraceError(Exception):
    """Base exception for Memtrace API errors."""

    def __init__(self, message: str, status_code: int = 0):
        self.message = message
        self.status_code = status_code
        super().__init__(message)


class AuthenticationError(MemtraceError):
    """Raised on 401 Unauthorized responses."""

    def __init__(self, message: str = "Invalid or missing API key"):
        super().__init__(message, status_code=401)


class NotFoundError(MemtraceError):
    """Raised on 404 Not Found responses."""

    def __init__(self, message: str = "Resource not found"):
        super().__init__(message, status_code=404)


class ConflictError(MemtraceError):
    """Raised on 409 Conflict responses (e.g. duplicate memory)."""

    def __init__(self, message: str = "Conflict"):
        super().__init__(message, status_code=409)


class NoArcInstanceError(MemtraceError):
    """Raised on 503 responses when the caller's organization has no Arc
    instance configured. The Memtrace deployment is multi-tenant; an
    administrator must run ``memtrace org add-arc <org_id>`` before this
    organization's API keys can read or write memories.
    """

    def __init__(self, message: str = "no arc instance configured for this org"):
        super().__init__(message, status_code=503)
