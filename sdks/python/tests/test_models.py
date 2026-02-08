"""Tests for Pydantic models."""

from __future__ import annotations

import pytest
from pydantic import ValidationError

from memtrace.models import (
    AddMemoryRequest,
    Agent,
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
)


class TestMemory:
    def test_full_memory(self):
        m = Memory(
            time="2026-02-08T12:00:00Z",
            org_id="org_1",
            agent_id="agent_1",
            session_id="sess_1",
            memory_type="episodic",
            event_type="general",
            content="hello",
            metadata={"key": "val"},
            tags=["a", "b"],
            dedup_key="abc",
            importance=0.5,
            parent_id="parent_1",
        )
        assert m.memory_type == "episodic"
        assert m.importance == 0.5

    def test_minimal_memory(self):
        m = Memory(
            time="2026-02-08T12:00:00Z",
            org_id="org_1",
            agent_id="agent_1",
            memory_type="decision",
            event_type="decision",
            content="chose option A",
        )
        assert m.session_id is None
        assert m.tags is None

    def test_invalid_memory_type(self):
        with pytest.raises(ValidationError):
            Memory(
                time="2026-02-08T12:00:00Z",
                org_id="org_1",
                agent_id="agent_1",
                memory_type="invalid_type",
                event_type="general",
                content="hello",
            )


class TestAddMemoryRequest:
    def test_exclude_none(self):
        req = AddMemoryRequest(
            agent_id="agent_1",
            memory_type="episodic",
            event_type="general",
            content="hello",
        )
        dumped = req.model_dump(exclude_none=True)
        assert "session_id" not in dumped
        assert "tags" not in dumped
        assert "dedup_key" not in dumped

    def test_importance_validation(self):
        with pytest.raises(ValidationError):
            AddMemoryRequest(
                agent_id="agent_1",
                memory_type="episodic",
                event_type="general",
                content="hello",
                importance=1.5,
            )

    def test_importance_valid(self):
        req = AddMemoryRequest(
            agent_id="agent_1",
            memory_type="episodic",
            event_type="general",
            content="hello",
            importance=0.0,
        )
        assert req.importance == 0.0


class TestListOptions:
    def test_exclude_none(self):
        opts = ListOptions(agent_id="agent_1", limit=50)
        dumped = opts.model_dump(exclude_none=True)
        assert dumped == {"agent_id": "agent_1", "limit": 50}

    def test_order_validation(self):
        opts = ListOptions(order="desc")
        assert opts.order == "desc"

        with pytest.raises(ValidationError):
            ListOptions(order="random")


class TestSearchQuery:
    def test_tags_is_list(self):
        q = SearchQuery(tags=["a", "b"])
        assert q.tags == ["a", "b"]

    def test_full_query(self):
        q = SearchQuery(
            agent_id="agent_1",
            memory_types=["episodic", "decision"],
            content_contains="dark mode",
            min_importance=0.5,
            limit=10,
            order="asc",
        )
        dumped = q.model_dump(exclude_none=True)
        assert dumped["memory_types"] == ["episodic", "decision"]
        assert dumped["min_importance"] == 0.5


class TestResponseModels:
    def test_memory_list(self):
        ml = MemoryList(memories=[], count=0, has_more=False)
        assert ml.count == 0

    def test_search_result(self):
        sr = SearchResult(results=[], count=0, query_time_ms=5)
        assert sr.query_time_ms == 5


class TestAgentModels:
    def test_agent(self):
        a = Agent(
            id="agent_1",
            org_id="org_1",
            name="test",
            created_at="2026-02-08T12:00:00Z",
        )
        assert a.description is None
        assert a.config is None

    def test_register_agent_request(self):
        req = RegisterAgentRequest(name="my-agent", description="does things")
        dumped = req.model_dump(exclude_none=True)
        assert dumped == {"name": "my-agent", "description": "does things"}

    def test_agent_stats(self):
        stats = AgentStats(
            agent_id="agent_1",
            memory_count=10,
            memories_24h=3,
            errors_24h=0,
            session_count=2,
            active_sessions=1,
            memory_types={"episodic": 8, "decision": 2},
        )
        assert stats.memory_types["episodic"] == 8


class TestSessionModels:
    def test_session(self):
        s = Session(
            id="sess_1",
            org_id="org_1",
            agent_id="agent_1",
            status="active",
            created_at="2026-02-08T10:00:00Z",
            updated_at="2026-02-08T12:00:00Z",
        )
        assert s.closed_at is None

    def test_create_session_request(self):
        req = CreateSessionRequest(agent_id="agent_1")
        dumped = req.model_dump(exclude_none=True)
        assert dumped == {"agent_id": "agent_1"}

    def test_context_options(self):
        opts = ContextOptions(max_tokens=4000, include_types=["episodic", "decision"])
        dumped = opts.model_dump(exclude_none=True)
        assert "since" not in dumped
        assert dumped["include_types"] == ["episodic", "decision"]

    def test_session_context(self):
        sc = SessionContext(session_id="sess_1", context="## Context\nHello", memory_count=1)
        assert sc.memory_count == 1
