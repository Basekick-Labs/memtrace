"""Tests for Memtrace memory tools."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock

import pytest
from agents import FunctionTool

from openai_agents_memtrace import create_memtrace_tools


# --- Factory ---


def test_create_returns_four_tools(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    assert len(tools) == 4
    assert all(isinstance(t, FunctionTool) for t in tools)


def test_create_tool_names(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    names = [t.name for t in tools]
    assert names == ["memtrace_remember", "memtrace_recall", "memtrace_search", "memtrace_decide"]


# --- Remember ---


@pytest.mark.asyncio
async def test_remember_calls_add_memory(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    remember = tools[0]

    await remember.on_invoke_tool(None, json.dumps({"content": "User likes cats"}))

    mock_client.add_memory.assert_called_once()
    req = mock_client.add_memory.call_args[0][0]
    assert req.agent_id == "agent_1"
    assert req.content == "User likes cats"
    assert req.memory_type == "episodic"
    assert req.event_type == "general"


@pytest.mark.asyncio
async def test_remember_uses_custom_params(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    remember = tools[0]

    await remember.on_invoke_tool(
        None,
        json.dumps({
            "content": "hello",
            "memory_type": "entity",
            "event_type": "observation",
            "tags": ["test"],
            "importance": 0.9,
        }),
    )

    req = mock_client.add_memory.call_args[0][0]
    assert req.memory_type == "entity"
    assert req.event_type == "observation"
    assert req.tags == ["test"]
    assert req.importance == 0.9


@pytest.mark.asyncio
async def test_remember_allows_agent_id_override(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    remember = tools[0]

    await remember.on_invoke_tool(
        None,
        json.dumps({"content": "shared info", "agent_id": "other_agent"}),
    )

    req = mock_client.add_memory.call_args[0][0]
    assert req.agent_id == "other_agent"


@pytest.mark.asyncio
async def test_remember_uses_default_session_id(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1", session_id="sess_1")
    remember = tools[0]

    await remember.on_invoke_tool(None, json.dumps({"content": "hello"}))

    req = mock_client.add_memory.call_args[0][0]
    assert req.session_id == "sess_1"


@pytest.mark.asyncio
async def test_remember_returns_json(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    remember = tools[0]

    result = await remember.on_invoke_tool(None, json.dumps({"content": "hello"}))

    parsed = json.loads(result)
    assert parsed["content"] == "User prefers dark mode"
    assert parsed["memory_type"] == "episodic"


# --- Recall ---


@pytest.mark.asyncio
async def test_recall_calls_list_memories(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    recall = tools[1]

    await recall.on_invoke_tool(None, json.dumps({}))

    mock_client.list_memories.assert_called_once()
    opts = mock_client.list_memories.call_args[0][0]
    assert opts.agent_id == "agent_1"
    assert opts.since == "24h"
    assert opts.limit == 50
    assert opts.order == "desc"


@pytest.mark.asyncio
async def test_recall_custom_params(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    recall = tools[1]

    await recall.on_invoke_tool(
        None,
        json.dumps({"since": "2h", "memory_type": "decision", "limit": 10}),
    )

    opts = mock_client.list_memories.call_args[0][0]
    assert opts.since == "2h"
    assert opts.memory_type == "decision"
    assert opts.limit == 10


@pytest.mark.asyncio
async def test_recall_returns_json(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    recall = tools[1]

    result = await recall.on_invoke_tool(None, json.dumps({}))

    parsed = json.loads(result)
    assert parsed["count"] == 1
    assert len(parsed["memories"]) == 1


# --- Search ---


@pytest.mark.asyncio
async def test_search_calls_search_memories(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    search = tools[2]

    await search.on_invoke_tool(
        None,
        json.dumps({"content_contains": "dark mode", "tags": ["preference"]}),
    )

    mock_client.search_memories.assert_called_once()
    query = mock_client.search_memories.call_args[0][0]
    assert query.agent_id == "agent_1"
    assert query.content_contains == "dark mode"
    assert query.tags == ["preference"]


@pytest.mark.asyncio
async def test_search_returns_json(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    search = tools[2]

    result = await search.on_invoke_tool(None, json.dumps({}))

    parsed = json.loads(result)
    assert parsed["count"] == 1
    assert parsed["query_time_ms"] == 3


# --- Decide ---


@pytest.mark.asyncio
async def test_decide_calls_client(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    decide = tools[3]

    await decide.on_invoke_tool(
        None,
        json.dumps({"decision": "Use PostgreSQL", "reasoning": "Better JSON support"}),
    )

    mock_client.decide.assert_called_once_with("agent_1", "Use PostgreSQL", "Better JSON support")


@pytest.mark.asyncio
async def test_decide_allows_agent_id_override(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    decide = tools[3]

    await decide.on_invoke_tool(
        None,
        json.dumps({
            "decision": "Use Redis",
            "reasoning": "Fast caching",
            "agent_id": "other_agent",
        }),
    )

    mock_client.decide.assert_called_once_with("other_agent", "Use Redis", "Fast caching")


@pytest.mark.asyncio
async def test_decide_returns_json(mock_client: AsyncMock) -> None:
    tools = create_memtrace_tools(mock_client, "agent_1")
    decide = tools[3]

    result = await decide.on_invoke_tool(
        None,
        json.dumps({"decision": "x", "reasoning": "y"}),
    )

    parsed = json.loads(result)
    assert parsed["memory_type"] == "decision"
