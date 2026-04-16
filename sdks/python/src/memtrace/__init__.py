"""Memtrace — LLM-agnostic memory layer for AI agents."""

from ._version import __version__
from .async_client import AsyncMemtrace
from .client import Memtrace
from .exceptions import AuthenticationError, ConflictError, MemtraceError, NotFoundError
from .models import (
    AddMemoryRequest,
    Agent,
    AgentList,
    AgentStats,
    ContextOptions,
    CreateSessionRequest,
    ListOptions,
    Memory,
    MemoryList,
    RegisterAgentRequest,
    SearchQuery,
    SearchResult,
    Session,
    SessionContext,
    SessionList,
)

__all__ = [
    "__version__",
    # Clients
    "Memtrace",
    "AsyncMemtrace",
    # Exceptions
    "MemtraceError",
    "AuthenticationError",
    "NotFoundError",
    "ConflictError",
    # Models
    "AddMemoryRequest",
    "Agent",
    "AgentList",
    "AgentStats",
    "ContextOptions",
    "CreateSessionRequest",
    "ListOptions",
    "Memory",
    "MemoryList",
    "RegisterAgentRequest",
    "SearchQuery",
    "SearchResult",
    "Session",
    "SessionContext",
    "SessionList",
]
