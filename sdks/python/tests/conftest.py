"""Shared test fixtures for Memtrace SDK tests."""

from __future__ import annotations

import pytest
import respx

BASE_URL = "http://localhost:9100"
API_KEY = "mtk_test_key_12345"


# --- Reusable JSON response fixtures ---

MEMORY_JSON = {
    "time": "2026-02-08T12:00:00Z",
    "org_id": "org_123",
    "agent_id": "agent_1",
    "session_id": "sess_1",
    "memory_type": "episodic",
    "event_type": "general",
    "content": "User prefers dark mode",
    "metadata": {"source": "settings"},
    "tags": ["preference"],
    "dedup_key": "abc123",
    "importance": 0.8,
    "parent_id": None,
}

AGENT_JSON = {
    "id": "agent_1",
    "org_id": "org_123",
    "name": "test-agent",
    "description": "A test agent",
    "config": {"model": "gpt-4"},
    "created_at": "2026-02-08T10:00:00Z",
    "last_active_at": "2026-02-08T12:00:00Z",
}

AGENT_STATS_JSON = {
    "agent_id": "agent_1",
    "memory_count": 42,
    "memories_24h": 10,
    "errors_24h": 1,
    "session_count": 5,
    "active_sessions": 2,
    "last_active_at": "2026-02-08T12:00:00Z",
    "memory_types": {"episodic": 30, "decision": 12},
}

SESSION_JSON = {
    "id": "sess_1",
    "org_id": "org_123",
    "agent_id": "agent_1",
    "status": "active",
    "metadata": {"task": "onboarding"},
    "created_at": "2026-02-08T10:00:00Z",
    "updated_at": "2026-02-08T12:00:00Z",
    "closed_at": None,
}

SESSION_CONTEXT_JSON = {
    "session_id": "sess_1",
    "context": "## Session Context (sess_1)\n\n### Recent Actions (1)\n- [12:00] general: User prefers dark mode\n",
    "memory_count": 1,
}


@pytest.fixture()
def mock_api():
    """Provide a respx mock router scoped to the base URL."""
    with respx.mock(base_url=BASE_URL) as router:
        yield router
