"""MemtraceSession — OpenAI Agents SDK session backed by Memtrace."""

from __future__ import annotations

import asyncio
from typing import Any

from agents.memory.session import SessionABC
from memtrace import (
    AddMemoryRequest,
    AsyncMemtrace,
    ContextOptions,
    CreateSessionRequest,
)


class MemtraceSession(SessionABC):
    """OpenAI Agents SDK session backed by Memtrace.

    Stores conversation history items in-memory (required by the Agents SDK)
    while also persisting significant events as Memtrace memories for
    cross-session and cross-agent recall.

    Use the async ``create()`` factory to instantiate::

        session = await MemtraceSession.create(client, agent_id="agent_1")
        result = await Runner.run(agent, "Hello", session=session)
    """

    def __init__(
        self,
        client: AsyncMemtrace,
        agent_id: str,
        memtrace_session_id: str,
        *,
        persist_user_messages: bool = True,
        persist_assistant_messages: bool = False,
    ) -> None:
        self._client = client
        self._agent_id = agent_id
        self._session_id = memtrace_session_id
        self._items: list[dict[str, Any]] = []
        self._lock = asyncio.Lock()
        self._persist_user = persist_user_messages
        self._persist_assistant = persist_assistant_messages

    @classmethod
    async def create(
        cls,
        client: AsyncMemtrace,
        agent_id: str,
        *,
        metadata: dict[str, Any] | None = None,
        inject_context: bool = True,
        context_since: str | None = None,
        context_max_tokens: int | None = None,
        persist_user_messages: bool = True,
        persist_assistant_messages: bool = False,
    ) -> MemtraceSession:
        """Create a new MemtraceSession with a server-side Memtrace session.

        Args:
            client: AsyncMemtrace client.
            agent_id: Agent ID that owns this session.
            metadata: Optional session metadata.
            inject_context: If True, injects prior memory context as the first
                conversation item. Gives the agent cross-session continuity.
            context_since: Time window for prior context (e.g. "24h").
            context_max_tokens: Max token budget for injected context.
            persist_user_messages: Persist user messages as Memtrace memories.
            persist_assistant_messages: Persist assistant messages as memories.
        """
        session = await client.create_session(
            CreateSessionRequest(agent_id=agent_id, metadata=metadata)
        )

        instance = cls(
            client,
            agent_id,
            session.id,
            persist_user_messages=persist_user_messages,
            persist_assistant_messages=persist_assistant_messages,
        )

        if inject_context:
            opts = ContextOptions(
                since=context_since,
                max_tokens=context_max_tokens,
            )
            ctx = await client.get_session_context(session.id, opts)
            if ctx.context and ctx.memory_count > 0:
                instance._items.append(
                    {
                        "role": "user",
                        "content": (
                            "[System: Prior memory context for this agent]\n\n" + ctx.context
                        ),
                    }
                )

        return instance

    @property
    def session_id(self) -> str:
        """Unique session identifier (Memtrace session ID)."""
        return self._session_id

    async def get_items(self, limit: int | None = None) -> list[dict[str, Any]]:
        """Retrieve conversation history items."""
        async with self._lock:
            if limit is not None:
                return list(self._items[-limit:])
            return list(self._items)

    async def add_items(self, items: list[dict[str, Any]]) -> None:
        """Store conversation items and persist significant ones as memories."""
        if not items:
            return
        async with self._lock:
            self._items.extend(items)

        for item in items:
            await self._maybe_persist(item)

    async def pop_item(self) -> dict[str, Any] | None:
        """Remove and return the most recent item."""
        async with self._lock:
            if self._items:
                return self._items.pop()
            return None

    async def clear_session(self) -> None:
        """Clear all conversation items."""
        async with self._lock:
            self._items.clear()

    async def close(self) -> None:
        """Close the Memtrace session on the server."""
        await self._client.close_session(self._session_id)

    async def _maybe_persist(self, item: dict[str, Any]) -> None:
        """Persist a conversation item as a Memtrace memory if configured.

        Errors are silently ignored to avoid breaking the agent loop when
        the Memtrace backend is unavailable.
        """
        role = item.get("role")
        content = item.get("content")
        if not isinstance(content, str) or not content:
            return

        try:
            if role == "user" and self._persist_user:
                await self._client.add_memory(
                    AddMemoryRequest(
                        agent_id=self._agent_id,
                        session_id=self._session_id,
                        memory_type="session",
                        event_type="user_message",
                        content=content,
                    )
                )
            elif role == "assistant" and self._persist_assistant:
                await self._client.add_memory(
                    AddMemoryRequest(
                        agent_id=self._agent_id,
                        session_id=self._session_id,
                        memory_type="session",
                        event_type="assistant_message",
                        content=content,
                    )
                )
        except Exception:
            pass
