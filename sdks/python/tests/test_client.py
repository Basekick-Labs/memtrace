"""Tests for the synchronous Memtrace client."""

from __future__ import annotations

import httpx
import pytest

from memtrace import Memtrace
from memtrace.exceptions import (
    AuthenticationError,
    ConflictError,
    MemtraceError,
    NoArcInstanceError,
    NotFoundError,
)
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
def client(mock_api):
    with Memtrace(BASE_URL, API_KEY) as c:
        yield c


# --- Convenience methods ---


class TestRemember:
    def test_remember(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=MEMORY_JSON)
        )
        mem = client.remember("agent_1", "User prefers dark mode")
        assert mem.content == "User prefers dark mode"
        assert mem.memory_type == "episodic"

    def test_remember_sends_correct_body(self, client, mock_api):
        route = mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=MEMORY_JSON)
        )
        client.remember("agent_1", "hello")
        body = route.calls[0].request.content
        import json

        data = json.loads(body)
        assert data["agent_id"] == "agent_1"
        assert data["memory_type"] == "episodic"
        assert data["event_type"] == "general"


class TestRecall:
    def test_recall(self, client, mock_api):
        mock_api.get("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = client.recall("agent_1")
        assert result.count == 1
        assert result.memories[0].content == "User prefers dark mode"


class TestDecide:
    def test_decide(self, client, mock_api):
        decision_json = {**MEMORY_JSON, "memory_type": "decision", "event_type": "decision"}
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=decision_json)
        )
        mem = client.decide("agent_1", "Use PostgreSQL", "Better JSON support")
        assert mem.memory_type == "decision"


# --- Memory CRUD ---


class TestAddMemory:
    def test_add_memory(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(200, json=MEMORY_JSON)
        )
        req = AddMemoryRequest(
            agent_id="agent_1",
            memory_type="episodic",
            event_type="general",
            content="hello",
        )
        mem = client.add_memory(req)
        assert mem.agent_id == "agent_1"


class TestAddMemories:
    def test_add_memories_batch(self, client, mock_api):
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
        result = client.add_memories(reqs)
        assert len(result) == 2


class TestListMemories:
    def test_list_memories(self, client, mock_api):
        mock_api.get("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = client.list_memories(ListOptions(agent_id="agent_1", limit=10))
        assert result.count == 1

    def test_list_memories_sends_params(self, client, mock_api):
        route = mock_api.get("/api/v1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [], "count": 0, "has_more": False}
            )
        )
        client.list_memories(ListOptions(agent_id="agent_1", since="2h", limit=50))
        url = str(route.calls[0].request.url)
        assert "agent_id=agent_1" in url
        assert "since=2h" in url
        assert "limit=50" in url


class TestSearchMemories:
    def test_search(self, client, mock_api):
        mock_api.post("/api/v1/search").mock(
            return_value=httpx.Response(
                200, json={"results": [MEMORY_JSON], "count": 1, "query_time_ms": 5}
            )
        )
        result = client.search_memories(
            SearchQuery(agent_id="agent_1", content_contains="dark")
        )
        assert result.count == 1
        assert result.query_time_ms == 5


# --- Agents ---


class TestRegisterAgent:
    def test_register(self, client, mock_api):
        mock_api.post("/api/v1/agents").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        agent = client.register_agent(RegisterAgentRequest(name="test-agent"))
        assert agent.name == "test-agent"


class TestGetAgent:
    def test_get(self, client, mock_api):
        mock_api.get("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        agent = client.get_agent("agent_1")
        assert agent.id == "agent_1"


class TestGetAgentStats:
    def test_stats(self, client, mock_api):
        mock_api.get("/api/v1/agents/agent_1/stats").mock(
            return_value=httpx.Response(200, json=AGENT_STATS_JSON)
        )
        stats = client.get_agent_stats("agent_1")
        assert stats.memory_count == 42
        assert stats.memory_types["episodic"] == 30


class TestListAgents:
    def test_list_agents(self, client, mock_api):
        mock_api.get("/api/v1/agents").mock(
            return_value=httpx.Response(200, json={"agents": [AGENT_JSON], "count": 1})
        )
        result = client.list_agents()
        assert result.count == 1
        assert result.agents[0].id == "agent_1"

    def test_list_agents_empty(self, client, mock_api):
        mock_api.get("/api/v1/agents").mock(
            return_value=httpx.Response(200, json={"agents": [], "count": 0})
        )
        result = client.list_agents()
        assert result.count == 0
        assert result.agents == []


class TestGetAgentMemories:
    def test_get_agent_memories(self, client, mock_api):
        mock_api.get("/api/v1/agents/agent_1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = client.get_agent_memories("agent_1")
        assert result.count == 1

    def test_get_agent_memories_with_opts(self, client, mock_api):
        route = mock_api.get("/api/v1/agents/agent_1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [], "count": 0, "has_more": False}
            )
        )
        client.get_agent_memories(
            "agent_1", ListOptions(limit=10, since="2h", order="desc")
        )
        url = str(route.calls[0].request.url)
        assert "limit=10" in url
        assert "since=2h" in url
        assert "order=desc" in url
        # agent_id must not be duplicated in query params
        assert "agent_id=" not in url


class TestDeleteAgent:
    def test_delete(self, client, mock_api):
        mock_api.delete("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(204)
        )
        result = client.delete_agent("agent_1")
        assert result is None


# --- Sessions ---


class TestCreateSession:
    def test_create(self, client, mock_api):
        mock_api.post("/api/v1/sessions").mock(
            return_value=httpx.Response(200, json=SESSION_JSON)
        )
        session = client.create_session(CreateSessionRequest(agent_id="agent_1"))
        assert session.status == "active"


class TestGetSession:
    def test_get(self, client, mock_api):
        mock_api.get("/api/v1/sessions/sess_1").mock(
            return_value=httpx.Response(200, json=SESSION_JSON)
        )
        session = client.get_session("sess_1")
        assert session.id == "sess_1"


class TestGetSessionContext:
    def test_context(self, client, mock_api):
        mock_api.post("/api/v1/sessions/sess_1/context").mock(
            return_value=httpx.Response(200, json=SESSION_CONTEXT_JSON)
        )
        ctx = client.get_session_context("sess_1", ContextOptions(since="24h"))
        assert ctx.memory_count == 1

    def test_context_no_opts(self, client, mock_api):
        mock_api.post("/api/v1/sessions/sess_1/context").mock(
            return_value=httpx.Response(200, json=SESSION_CONTEXT_JSON)
        )
        ctx = client.get_session_context("sess_1")
        assert ctx.session_id == "sess_1"


class TestListSessions:
    def test_list_all(self, client, mock_api):
        mock_api.get("/api/v1/sessions").mock(
            return_value=httpx.Response(
                200, json={"sessions": [SESSION_JSON], "count": 1}
            )
        )
        result = client.list_sessions()
        assert result.count == 1
        assert result.sessions[0].id == "sess_1"

    def test_list_with_agent_id(self, client, mock_api):
        route = mock_api.get("/api/v1/sessions").mock(
            return_value=httpx.Response(
                200, json={"sessions": [], "count": 0}
            )
        )
        client.list_sessions(agent_id="agent_1")
        url = str(route.calls[0].request.url)
        assert "agent_id=agent_1" in url

    def test_list_empty(self, client, mock_api):
        mock_api.get("/api/v1/sessions").mock(
            return_value=httpx.Response(
                200, json={"sessions": [], "count": 0}
            )
        )
        result = client.list_sessions()
        assert result.count == 0
        assert result.sessions == []


class TestGetSessionMemories:
    def test_get_session_memories(self, client, mock_api):
        mock_api.get("/api/v1/sessions/sess_1/memories").mock(
            return_value=httpx.Response(
                200, json={"memories": [MEMORY_JSON], "count": 1, "has_more": False}
            )
        )
        result = client.get_session_memories("sess_1")
        assert result.count == 1
        assert result.memories[0].session_id == "sess_1"


class TestCloseSession:
    def test_close(self, client, mock_api):
        closed = {**SESSION_JSON, "status": "closed", "closed_at": "2026-02-08T13:00:00Z"}
        mock_api.put("/api/v1/sessions/sess_1").mock(
            return_value=httpx.Response(200, json=closed)
        )
        session = client.close_session("sess_1")
        assert session.status == "closed"
        assert session.closed_at is not None


# --- Error handling ---


class TestErrorHandling:
    def test_401_raises_authentication_error(self, client, mock_api):
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(401, json={"error": "Invalid API key"})
        )
        with pytest.raises(AuthenticationError) as exc_info:
            client.get_agent("x")
        assert exc_info.value.status_code == 401
        assert "Invalid API key" in exc_info.value.message

    def test_404_raises_not_found_error(self, client, mock_api):
        mock_api.get("/api/v1/agents/missing").mock(
            return_value=httpx.Response(404, json={"error": "Agent not found"})
        )
        with pytest.raises(NotFoundError):
            client.get_agent("missing")

    def test_409_raises_conflict_error(self, client, mock_api):
        mock_api.post("/api/v1/memories").mock(
            return_value=httpx.Response(409, json={"error": "Duplicate memory"})
        )
        req = AddMemoryRequest(
            agent_id="agent_1",
            memory_type="episodic",
            event_type="general",
            content="dup",
        )
        with pytest.raises(ConflictError):
            client.add_memory(req)

    def test_500_raises_memtrace_error(self, client, mock_api):
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(500, json={"error": "Internal server error"})
        )
        with pytest.raises(MemtraceError) as exc_info:
            client.get_agent("x")
        assert exc_info.value.status_code == 500

    def test_503_no_arc_instance_raises_typed_error(self, client, mock_api):
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(
                503,
                json={"error": "no arc instance configured for this org"},
            )
        )
        with pytest.raises(NoArcInstanceError) as exc_info:
            client.get_agent("x")
        assert exc_info.value.status_code == 503
        # Falls through MemtraceError so generic handlers still catch it.
        assert isinstance(exc_info.value, MemtraceError)

    def test_503_unrelated_falls_back_to_memtrace_error(self, client, mock_api):
        # A 503 without "arc instance" in the message should NOT be misclassified.
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(503, json={"error": "upstream timeout"})
        )
        with pytest.raises(MemtraceError) as exc_info:
            client.get_agent("x")
        assert not isinstance(exc_info.value, NoArcInstanceError)
        assert exc_info.value.status_code == 503

    def test_error_without_json_body(self, client, mock_api):
        mock_api.get("/api/v1/agents/x").mock(
            return_value=httpx.Response(502, text="Bad Gateway")
        )
        with pytest.raises(MemtraceError) as exc_info:
            client.get_agent("x")
        assert "Bad Gateway" in exc_info.value.message


# --- Context manager ---


class TestContextManager:
    def test_with_statement(self, mock_api):
        mock_api.get("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        with Memtrace(BASE_URL, API_KEY) as client:
            agent = client.get_agent("agent_1")
            assert agent.id == "agent_1"


# --- Path escaping ---


class TestPathEscaping:
    """Path params with URL-unsafe characters must be percent-encoded."""

    def test_agent_id_with_slash_is_escaped(self, client, mock_api):
        # Register the escaped path explicitly; if the SDK sent an unescaped
        # "agents/../../etc" it would hit a different route (or no route) and
        # respx would raise.
        mock_api.get("/api/v1/agents/a%2Fb%2Fc").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        client.get_agent("a/b/c")

    def test_session_id_with_space_is_escaped(self, client, mock_api):
        mock_api.get("/api/v1/sessions/sess%20with%20space").mock(
            return_value=httpx.Response(200, json=SESSION_JSON)
        )
        client.get_session("sess with space")

    def test_agent_id_with_traversal_is_escaped(self, client, mock_api):
        mock_api.get("/api/v1/agents/..%2Fadmin").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        client.get_agent("../admin")


# --- Headers ---


class TestHeaders:
    def test_api_key_header(self, client, mock_api):
        route = mock_api.get("/api/v1/agents/agent_1").mock(
            return_value=httpx.Response(200, json=AGENT_JSON)
        )
        client.get_agent("agent_1")
        req = route.calls[0].request
        assert req.headers["x-api-key"] == API_KEY
