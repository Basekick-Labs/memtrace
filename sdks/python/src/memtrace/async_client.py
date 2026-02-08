"""Asynchronous Memtrace client."""

from __future__ import annotations

import httpx

from ._base import DEFAULT_TIMEOUT, build_headers, handle_error
from .models import (
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


class AsyncMemtrace:
    """Asynchronous client for the Memtrace API.

    Usage::

        from memtrace import AsyncMemtrace

        async with AsyncMemtrace("http://localhost:9100", "mtk_...") as client:
            await client.remember("agent_1", "User prefers dark mode")
    """

    def __init__(self, base_url: str, api_key: str, *, timeout: float = DEFAULT_TIMEOUT):
        self._client = httpx.AsyncClient(
            base_url=base_url,
            headers=build_headers(api_key),
            timeout=timeout,
        )

    async def close(self) -> None:
        """Close the underlying HTTP client."""
        await self._client.aclose()

    async def __aenter__(self) -> AsyncMemtrace:
        return self

    async def __aexit__(self, *args: object) -> None:
        await self.close()

    # --- Convenience methods ---

    async def remember(self, agent_id: str, content: str) -> Memory:
        """Store an episodic memory (quick add)."""
        return await self.add_memory(AddMemoryRequest(
            agent_id=agent_id,
            memory_type="episodic",
            event_type="general",
            content=content,
        ))

    async def recall(self, agent_id: str, since: str = "24h") -> MemoryList:
        """Retrieve recent memories for an agent."""
        return await self.list_memories(ListOptions(
            agent_id=agent_id,
            since=since,
            order="desc",
            limit=100,
        ))

    async def decide(self, agent_id: str, decision: str, reasoning: str) -> Memory:
        """Log a decision with reasoning."""
        return await self.add_memory(AddMemoryRequest(
            agent_id=agent_id,
            memory_type="decision",
            event_type="decision",
            content=decision,
            metadata={"reasoning": reasoning},
        ))

    # --- Memory CRUD ---

    async def add_memory(self, req: AddMemoryRequest) -> Memory:
        """Store a new memory."""
        resp = await self._client.post(
            "/api/v1/memories", json=req.model_dump(exclude_none=True)
        )
        handle_error(resp)
        return Memory.model_validate(resp.json())

    async def add_memories(self, memories: list[AddMemoryRequest]) -> list[Memory]:
        """Store multiple memories in a batch."""
        payload = {"memories": [m.model_dump(exclude_none=True) for m in memories]}
        resp = await self._client.post("/api/v1/memories", json=payload)
        handle_error(resp)
        data = resp.json()
        return [Memory.model_validate(m) for m in data["memories"]]

    async def list_memories(self, opts: ListOptions) -> MemoryList:
        """List memories with filters."""
        params = opts.model_dump(exclude_none=True)
        resp = await self._client.get("/api/v1/memories", params=params)
        handle_error(resp)
        return MemoryList.model_validate(resp.json())

    async def search_memories(self, query: SearchQuery) -> SearchResult:
        """Search memories with structured filters."""
        resp = await self._client.post(
            "/api/v1/search", json=query.model_dump(exclude_none=True)
        )
        handle_error(resp)
        return SearchResult.model_validate(resp.json())

    # --- Agents ---

    async def register_agent(self, req: RegisterAgentRequest) -> Agent:
        """Register a new agent."""
        resp = await self._client.post(
            "/api/v1/agents", json=req.model_dump(exclude_none=True)
        )
        handle_error(resp)
        return Agent.model_validate(resp.json())

    async def get_agent(self, agent_id: str) -> Agent:
        """Get an agent by ID."""
        resp = await self._client.get(f"/api/v1/agents/{agent_id}")
        handle_error(resp)
        return Agent.model_validate(resp.json())

    async def get_agent_stats(self, agent_id: str) -> AgentStats:
        """Get memory stats for an agent."""
        resp = await self._client.get(f"/api/v1/agents/{agent_id}/stats")
        handle_error(resp)
        return AgentStats.model_validate(resp.json())

    # --- Sessions ---

    async def create_session(self, req: CreateSessionRequest) -> Session:
        """Start a new session."""
        resp = await self._client.post(
            "/api/v1/sessions", json=req.model_dump(exclude_none=True)
        )
        handle_error(resp)
        return Session.model_validate(resp.json())

    async def get_session(self, session_id: str) -> Session:
        """Get a session by ID."""
        resp = await self._client.get(f"/api/v1/sessions/{session_id}")
        handle_error(resp)
        return Session.model_validate(resp.json())

    async def get_session_context(
        self, session_id: str, opts: ContextOptions | None = None
    ) -> SessionContext:
        """Get LLM-formatted session context."""
        body = opts.model_dump(exclude_none=True) if opts else {}
        resp = await self._client.post(
            f"/api/v1/sessions/{session_id}/context", json=body
        )
        handle_error(resp)
        return SessionContext.model_validate(resp.json())

    async def close_session(self, session_id: str) -> Session:
        """Close a session."""
        resp = await self._client.put(
            f"/api/v1/sessions/{session_id}", json={"status": "closed"}
        )
        handle_error(resp)
        return Session.model_validate(resp.json())
