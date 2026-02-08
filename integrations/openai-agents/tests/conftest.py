"""Shared fixtures for OpenAI Agents Memtrace tests."""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest
from memtrace.models import Memory, MemoryList, SearchResult, Session, SessionContext


MEMORY_FIXTURE = Memory(
    time="2026-02-08T12:00:00Z",
    org_id="org_123",
    agent_id="agent_1",
    memory_type="episodic",
    event_type="general",
    content="User prefers dark mode",
)

DECISION_FIXTURE = Memory(
    time="2026-02-08T12:01:00Z",
    org_id="org_123",
    agent_id="agent_1",
    memory_type="decision",
    event_type="decision",
    content="Use PostgreSQL",
    metadata={"reasoning": "Better JSON support"},
)

SESSION_FIXTURE = Session(
    id="sess_1",
    org_id="org_123",
    agent_id="agent_1",
    status="active",
    created_at="2026-02-08T10:00:00Z",
    updated_at="2026-02-08T10:00:00Z",
)

CLOSED_SESSION_FIXTURE = Session(
    id="sess_1",
    org_id="org_123",
    agent_id="agent_1",
    status="closed",
    created_at="2026-02-08T10:00:00Z",
    updated_at="2026-02-08T13:00:00Z",
    closed_at="2026-02-08T13:00:00Z",
)

CONTEXT_FIXTURE = SessionContext(
    session_id="sess_1",
    context="## Session Context\n\n### Recent Actions (1)\n- [12:00] general: User prefers dark mode\n",
    memory_count=1,
)

EMPTY_CONTEXT_FIXTURE = SessionContext(
    session_id="sess_1",
    context="",
    memory_count=0,
)

MEMORY_LIST_FIXTURE = MemoryList(
    memories=[MEMORY_FIXTURE],
    count=1,
    has_more=False,
)

SEARCH_RESULT_FIXTURE = SearchResult(
    results=[MEMORY_FIXTURE],
    count=1,
    query_time_ms=3,
)


@pytest.fixture()
def mock_client() -> AsyncMock:
    """Provide a mocked AsyncMemtrace client."""
    client = AsyncMock()
    client.add_memory.return_value = MEMORY_FIXTURE
    client.decide.return_value = DECISION_FIXTURE
    client.list_memories.return_value = MEMORY_LIST_FIXTURE
    client.search_memories.return_value = SEARCH_RESULT_FIXTURE
    client.create_session.return_value = SESSION_FIXTURE
    client.get_session_context.return_value = CONTEXT_FIXTURE
    client.close_session.return_value = CLOSED_SESSION_FIXTURE
    return client
