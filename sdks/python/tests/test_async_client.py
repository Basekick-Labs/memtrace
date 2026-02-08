"""Tests for the asynchronous Memtrace client."""

from __future__ import annotations

import httpx
import pytest

from memtrace import AsyncMemtrace
from memtrace.exceptions import AuthenticationError, NotFoundError
from memtrace.models import (
    AddMemoryRequest,
    ContextOptions,
    CreateSessionRequest,
    ListOptions,
    RegisterAgentRequest,
    SearchQuery,
)

from .conftest import (
    AGENT_JSON,
    AGENT_STATS_JSON,
    API_KEY,
    BASE_URL,
    MEMORY_JSON,
    SESSION_CONTEXT_JSON,
    SESSION_JSON,
)


@pytest.fixture()
async def client(mock_api):
    async with AsyncMemtrace(BASE_URL, API_KEY) as c:
        yield c


# --- Convenience methods ---


class TestRemember:
    async def test_remember(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=MEMORY_JSON)
        )
        mem = await client.remember("agent_1", "User prefers dark mode")
        assert mem.content == "User prefers dark mode"
        assert mem.memory_type == "episodic"


class TestRecall:
    async def test_recall(self, client, mock_api):
        mock_api.get("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = await client.recall("agent_1")
        assert result.count == 1


class TestDecide:
    async def test_decide(self, client, mock_api):
        decision_json = {**MEMORY_JSON, "memory_type": "decision", "event_type": "decision"}
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=decision_json)
        )
        mem = await client.decide("agent_1", "Use PostgreSQL", "Better JSON support")
        assert mem.memory_type == "decision"


# --- Memory CRUD ---


class TestAddMemory:
    async def test_add_memory(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=MEMORY_JSON)
        )
        req = AddMemoryRequest(
            agent_id="agent_1",
            memory_type="episodic",
            event_type="general",
            content="hello",
        )
        mem = await client.add_memory(req)
        assert mem.agent_id == "agent_1"


class TestAddMemories:
    async def test_batch(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON, MEMORY_JSON], "count": 2}
            )
        )
        reqs = [
            AddMemoryRequest(
                agent_id="agent_1",
                memory_type="episodic",
                event_type="general",
                content=f"memory {i}",
            )
            for i in range(2)
        ]
        result = await client.add_memories(reqs)
        assert len(result) == 2


class TestListMemories:
    async def test_list(self, client, mock_api):
        mock_api.get("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = await client.list_memories(ListOptions(agent_id="agent_1"))
        assert result.count == 1


class TestSearchMemories:
    async def test_search(self, client, mock_api):
        mock_api.post("/api/v1/search").mock(
            return_value=httpx.Response(
                200, json={"results": [MEMORY_JSON], "count": 1, "query_time_ms": 3}
            )
        )
        result = await client.search_memories(SearchQuery(content_contains="dark"))
        assert result.query_time_ms == 3


# --- Agents ---


class TestRegisterAgent:
    async def test_register(self, client, mock_api):
        mock_api.post("/api/v1/agents").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        agent = await client.register_agent(RegisterAgentRequest(name="test-agent"))
        assert agent.name == "test-agent"


class TestGetAgent:
    async def test_get(self, client, mock_api):
        mock_api.get("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        agent = await client.get_agent("agent_1")
        assert agent.id == "agent_1"


class TestGetAgentStats:
    async def test_stats(self, client, mock_api):
        mock_api.get("/api/v1/agents/agent_1/stats").mock(
            return_value=httpx.Response(200, json=AGENT_STATS_JSON)
        )
        stats = await client.get_agent_stats("agent_1")
        assert stats.memory_count == 42


# --- Sessions ---


class TestCreateSession:
    async def test_create(self, client, mock_api):
        mock_api.post("/api/v1/sessions").mock(
            return_value=httpx.Response(200, json=SESSION_JSON)
        )
        session = await client.create_session(CreateSessionRequest(agent_id="agent_1"))
        assert session.status == "active"


class TestGetSession:
    async def test_get(self, client, mock_api):
        mock_api.get("/api/v1/sessions/sess_1").mock(
            return_value=httpx.Response(200, json=SESSION_JSON)
        )
        session = await client.get_session("sess_1")
        assert session.id == "sess_1"


class TestGetSessionContext:
    async def test_context(self, client, mock_api):
        mock_api.post("/api/v1/sessions/sess_1/context").mock(
            return_value=httpx.Response(200, json=SESSION_CONTEXT_JSON)
        )
        ctx = await client.get_session_context("sess_1", ContextOptions(since="24h"))
        assert ctx.memory_count == 1

    async def test_context_no_opts(self, client, mock_api):
        mock_api.post("/api/v1/sessions/sess_1/context").mock(
            return_value=httpx.Response(200, json=SESSION_CONTEXT_JSON)
        )
        ctx = await client.get_session_context("sess_1")
        assert ctx.session_id == "sess_1"


class TestListSessions:
    async def test_list_all(self, client, mock_api):
        mock_api.get("/api/v1/sessions").mock(
            return_value=httpx.Response(
                200, json={"sessions": [SESSION_JSON], "count": 1}
            )
        )
        result = await client.list_sessions()
        assert result.count == 1
        assert result.sessions[0].id == "sess_1"

    async def test_list_with_agent_id(self, client, mock_api):
        route = mock_api.get("/api/v1/sessions").mock(
            return_value=httpx.Response(
                200, json={"sessions": [], "count": 0}
            )
        )
        await client.list_sessions(agent_id="agent_1")
        url = str(route.calls[0].request.url)
        assert "agent_id=agent_1" in url


class TestCloseSession:
    async def test_close(self, client, mock_api):
        closed = {**SESSION_JSON, "status": "closed", "closed_at": "2026-02-08T13:00:00Z"}
        mock_api.put("/api/v1/sessions/sess_1").mock(
            return_value=httpx.Response(200, json=closed)
        )
        session = await client.close_session("sess_1")
        assert session.status == "closed"


# --- Error handling ---


class TestErrorHandling:
    async def test_401(self, client, mock_api):
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(401, json={"error": "Invalid API key"})
        )
        with pytest.raises(AuthenticationError):
            await client.get_agent("x")

    async def test_404(self, client, mock_api):
        mock_api.get("/api/v1/agents/missing").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )
        with pytest.raises(NotFoundError):
            await client.get_agent("missing")


# --- Context manager ---


class TestAsyncContextManager:
    async def test_async_with(self, mock_api):
        mock_api.get("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        async with AsyncMemtrace(BASE_URL, API_KEY) as client:
            agent = await client.get_agent("agent_1")
            assert agent.id == "agent_1"
