"""Tests for MemtraceSession."""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from openai_agents_memtrace import MemtraceSession
from tests.conftest import EMPTY_CONTEXT_FIXTURE


# --- Creation ---


@pytest.mark.asyncio
async def test_create_calls_create_session(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1")

    mock_client.create_session.assert_called_once()
    req = mock_client.create_session.call_args[0][0]
    assert req.agent_id == "agent_1"
    assert session.session_id == "sess_1"


@pytest.mark.asyncio
async def test_create_with_metadata(mock_client: AsyncMock) -> None:
    await MemtraceSession.create(mock_client, "agent_1", metadata={"task": "onboarding"})

    req = mock_client.create_session.call_args[0][0]
    assert req.metadata == {"task": "onboarding"}


@pytest.mark.asyncio
async def test_create_injects_context(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1")

    items = await session.get_items()
    assert len(items) == 1
    assert items[0]["role"] == "user"
    assert "Prior memory context" in items[0]["content"]
    assert "Session Context" in items[0]["content"]


@pytest.mark.asyncio
async def test_create_no_context_when_empty(mock_client: AsyncMock) -> None:
    mock_client.get_session_context.return_value = EMPTY_CONTEXT_FIXTURE

    session = await MemtraceSession.create(mock_client, "agent_1")

    items = await session.get_items()
    assert len(items) == 0


@pytest.mark.asyncio
async def test_create_inject_context_false(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    mock_client.get_session_context.assert_not_called()
    items = await session.get_items()
    assert len(items) == 0


@pytest.mark.asyncio
async def test_create_passes_context_options(mock_client: AsyncMock) -> None:
    await MemtraceSession.create(
        mock_client,
        "agent_1",
        context_since="2h",
        context_max_tokens=4000,
    )

    opts = mock_client.get_session_context.call_args[0][1]
    assert opts.since == "2h"
    assert opts.max_tokens == 4000


# --- get_items ---


@pytest.mark.asyncio
async def test_get_items_returns_all(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)
    await session.add_items([
        {"role": "user", "content": "hello"},
        {"role": "assistant", "content": "hi"},
    ])

    items = await session.get_items()
    assert len(items) == 2


@pytest.mark.asyncio
async def test_get_items_with_limit(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)
    await session.add_items([
        {"role": "user", "content": "first"},
        {"role": "user", "content": "second"},
        {"role": "user", "content": "third"},
    ])

    items = await session.get_items(limit=2)
    assert len(items) == 2
    assert items[0]["content"] == "second"
    assert items[1]["content"] == "third"


# --- add_items ---


@pytest.mark.asyncio
async def test_add_items_persists_user_messages(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.add_items([{"role": "user", "content": "I need help"}])

    mock_client.add_memory.assert_called_once()
    req = mock_client.add_memory.call_args[0][0]
    assert req.agent_id == "agent_1"
    assert req.session_id == "sess_1"
    assert req.memory_type == "session"
    assert req.event_type == "user_message"
    assert req.content == "I need help"


@pytest.mark.asyncio
async def test_add_items_skips_assistant_by_default(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.add_items([{"role": "assistant", "content": "Sure, how can I help?"}])

    mock_client.add_memory.assert_not_called()


@pytest.mark.asyncio
async def test_add_items_persists_assistant_when_enabled(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(
        mock_client, "agent_1", inject_context=False, persist_assistant_messages=True
    )

    await session.add_items([{"role": "assistant", "content": "Here is your answer"}])

    mock_client.add_memory.assert_called_once()
    req = mock_client.add_memory.call_args[0][0]
    assert req.event_type == "assistant_message"


@pytest.mark.asyncio
async def test_add_items_skips_non_string_content(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.add_items([{"role": "user", "content": [{"type": "text", "text": "hi"}]}])

    mock_client.add_memory.assert_not_called()


@pytest.mark.asyncio
async def test_add_items_skips_empty_content(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.add_items([{"role": "user", "content": ""}])

    mock_client.add_memory.assert_not_called()


@pytest.mark.asyncio
async def test_add_empty_list_is_noop(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.add_items([])

    items = await session.get_items()
    assert len(items) == 0


# --- pop_item ---


@pytest.mark.asyncio
async def test_pop_item_returns_last(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)
    await session.add_items([
        {"role": "user", "content": "first"},
        {"role": "user", "content": "second"},
    ])

    item = await session.pop_item()
    assert item is not None
    assert item["content"] == "second"

    items = await session.get_items()
    assert len(items) == 1


@pytest.mark.asyncio
async def test_pop_item_returns_none_when_empty(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    item = await session.pop_item()
    assert item is None


# --- clear_session ---


@pytest.mark.asyncio
async def test_clear_session(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)
    await session.add_items([{"role": "user", "content": "hello"}])

    await session.clear_session()

    items = await session.get_items()
    assert len(items) == 0


# --- close ---


@pytest.mark.asyncio
async def test_close_calls_api(mock_client: AsyncMock) -> None:
    session = await MemtraceSession.create(mock_client, "agent_1", inject_context=False)

    await session.close()

    mock_client.close_session.assert_called_once_with("sess_1")
