"""Pydantic v2 models for the Memtrace API."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field


# --- Memory ---


class Memory(BaseModel):
    """A stored memory event."""

    time: datetime
    org_id: str
    agent_id: str
    session_id: str | None = None
    memory_type: Literal["episodic", "session", "decision", "entity"]
    event_type: str
    content: str
    metadata: dict[str, Any] | None = None
    tags: list[str] | None = None
    dedup_key: str | None = None
    importance: float | None = None
    parent_id: str | None = None


class AddMemoryRequest(BaseModel):
    """Request body for adding a single memory."""

    agent_id: str
    session_id: str | None = None
    memory_type: Literal["episodic", "session", "decision", "entity"]
    event_type: str
    content: str
    metadata: dict[str, Any] | None = None
    tags: list[str] | None = None
    dedup_key: str | None = None
    importance: float | None = Field(default=None, ge=0, le=1)
    parent_id: str | None = None


class ListOptions(BaseModel):
    """Query parameters for listing memories."""

    agent_id: str | None = None
    session_id: str | None = None
    memory_type: str | None = None
    event_type: str | None = None
    tags: str | None = None  # comma-separated (query param)
    since: str | None = None  # relative ("2h", "7d") or ISO 8601
    until: str | None = None
    limit: int | None = None
    offset: int | None = None
    order: Literal["asc", "desc"] | None = None


class MemoryList(BaseModel):
    """Response for listing memories."""

    memories: list[Memory]
    count: int
    has_more: bool


class SearchQuery(BaseModel):
    """Request body for searching memories."""

    agent_id: str | None = None
    session_id: str | None = None
    memory_types: list[str] | None = None
    event_types: list[str] | None = None
    tags: list[str] | None = None  # JSON body list
    content_contains: str | None = None
    since: str | None = None
    until: str | None = None
    min_importance: float | None = None
    limit: int | None = None
    order: Literal["asc", "desc"] | None = None


class SearchResult(BaseModel):
    """Response for searching memories."""

    results: list[Memory]
    count: int
    query_time_ms: int


# --- Agents ---


class Agent(BaseModel):
    """A registered agent."""

    id: str
    org_id: str
    name: str
    description: str | None = None
    config: dict[str, Any] | None = None
    created_at: datetime
    last_active_at: datetime | None = None


class RegisterAgentRequest(BaseModel):
    """Request body for registering an agent."""

    name: str
    description: str | None = None
    config: dict[str, Any] | None = None


class AgentStats(BaseModel):
    """Memory statistics for an agent."""

    agent_id: str
    memory_count: int
    memories_24h: int
    errors_24h: int
    session_count: int
    active_sessions: int
    last_active_at: datetime | None = None
    memory_types: dict[str, int]


# --- Sessions ---


class Session(BaseModel):
    """A bounded context for agent work."""

    id: str
    org_id: str
    agent_id: str
    status: str
    metadata: dict[str, Any] | None = None
    created_at: datetime
    updated_at: datetime
    closed_at: datetime | None = None


class CreateSessionRequest(BaseModel):
    """Request body for creating a session."""

    agent_id: str
    metadata: dict[str, Any] | None = None


class ContextOptions(BaseModel):
    """Options for session context retrieval."""

    max_tokens: int | None = None
    include_types: list[str] | None = None
    since: str | None = None


class SessionContext(BaseModel):
    """LLM-ready formatted context for a session."""

    session_id: str
    context: str
    memory_count: int
