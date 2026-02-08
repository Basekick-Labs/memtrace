"""Memtrace memory tools for OpenAI Agents."""

from __future__ import annotations

import json
from typing import Any

from agents import FunctionTool
from agents.tool import ToolContext
from memtrace import AddMemoryRequest, AsyncMemtrace, ListOptions, SearchQuery


def create_memtrace_tools(
    client: AsyncMemtrace,
    agent_id: str,
    *,
    session_id: str | None = None,
) -> list[FunctionTool]:
    """Create Memtrace memory tools for an OpenAI Agent.

    Args:
        client: An AsyncMemtrace client instance.
        agent_id: Default agent ID for all memory operations.
        session_id: Optional default session ID to scope memories to.

    Returns:
        A list of FunctionTool instances ready to pass to Agent(tools=[...]).
    """
    return [
        _make_remember_tool(client, agent_id, session_id),
        _make_recall_tool(client, agent_id, session_id),
        _make_search_tool(client, agent_id),
        _make_decide_tool(client, agent_id),
    ]


def _make_remember_tool(
    client: AsyncMemtrace,
    default_agent_id: str,
    default_session_id: str | None,
) -> FunctionTool:
    async def on_invoke(ctx: ToolContext[Any], args_json: str) -> str:
        args = json.loads(args_json)
        mem = await client.add_memory(
            AddMemoryRequest(
                agent_id=args.get("agent_id", default_agent_id),
                content=args["content"],
                memory_type=args.get("memory_type", "episodic"),
                event_type=args.get("event_type", "general"),
                session_id=args.get("session_id", default_session_id),
                tags=args.get("tags"),
                importance=args.get("importance"),
                metadata=args.get("metadata"),
            )
        )
        return json.dumps(mem.model_dump(mode="json"), default=str)

    return FunctionTool(
        name="memtrace_remember",
        description=(
            "Store a memory. Use this to record actions, observations, events, "
            "or any information the agent should remember later."
        ),
        params_json_schema={
            "type": "object",
            "properties": {
                "content": {
                    "type": "string",
                    "description": "Memory content text",
                },
                "memory_type": {
                    "type": "string",
                    "description": "Memory type: episodic (default), decision, entity, session",
                    "enum": ["episodic", "decision", "entity", "session"],
                },
                "event_type": {
                    "type": "string",
                    "description": "Event type (e.g. observation, action, error). Default: general",
                },
                "session_id": {
                    "type": "string",
                    "description": "Session ID to scope this memory to",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Tags for categorization",
                },
                "importance": {
                    "type": "number",
                    "description": "Importance score 0.0 to 1.0",
                },
                "metadata": {
                    "type": "object",
                    "description": "Arbitrary key-value metadata",
                },
                "agent_id": {
                    "type": "string",
                    "description": "Override default agent ID (for cross-agent memory)",
                },
            },
            "required": ["content"],
            "additionalProperties": False,
        },
        on_invoke_tool=on_invoke,
        strict_json_schema=False,
    )


def _make_recall_tool(
    client: AsyncMemtrace,
    default_agent_id: str,
    default_session_id: str | None,
) -> FunctionTool:
    async def on_invoke(ctx: ToolContext[Any], args_json: str) -> str:
        args = json.loads(args_json)
        result = await client.list_memories(
            ListOptions(
                agent_id=args.get("agent_id", default_agent_id),
                since=args.get("since", "24h"),
                session_id=args.get("session_id", default_session_id),
                memory_type=args.get("memory_type"),
                limit=min(args.get("limit", 50), 200),
                order="desc",
            )
        )
        return json.dumps(result.model_dump(mode="json"), default=str)

    return FunctionTool(
        name="memtrace_recall",
        description=(
            "Retrieve recent memories for an agent. "
            "Returns memories in reverse chronological order."
        ),
        params_json_schema={
            "type": "object",
            "properties": {
                "since": {
                    "type": "string",
                    "description": "Time window (e.g. 2h, 24h, 7d). Default: 24h",
                },
                "session_id": {
                    "type": "string",
                    "description": "Filter by session ID",
                },
                "memory_type": {
                    "type": "string",
                    "description": "Filter by memory type",
                    "enum": ["episodic", "decision", "entity", "session"],
                },
                "limit": {
                    "type": "integer",
                    "description": "Max results. Default: 50",
                },
                "agent_id": {
                    "type": "string",
                    "description": "Override default agent ID",
                },
            },
            "required": [],
            "additionalProperties": False,
        },
        on_invoke_tool=on_invoke,
        strict_json_schema=False,
    )


def _make_search_tool(
    client: AsyncMemtrace,
    default_agent_id: str,
) -> FunctionTool:
    async def on_invoke(ctx: ToolContext[Any], args_json: str) -> str:
        args = json.loads(args_json)
        result = await client.search_memories(
            SearchQuery(
                agent_id=args.get("agent_id", default_agent_id),
                content_contains=args.get("content_contains"),
                memory_types=args.get("memory_types"),
                tags=args.get("tags"),
                since=args.get("since"),
                min_importance=args.get("min_importance"),
                limit=min(args.get("limit", 50), 200),
                order="desc",
            )
        )
        return json.dumps(result.model_dump(mode="json"), default=str)

    return FunctionTool(
        name="memtrace_search",
        description=(
            "Search memories with structured filters: "
            "by content text, memory types, tags, importance, and time range."
        ),
        params_json_schema={
            "type": "object",
            "properties": {
                "content_contains": {
                    "type": "string",
                    "description": "Search text within memory content",
                },
                "memory_types": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Filter by memory types (episodic, decision, entity, session)",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Filter by tags",
                },
                "since": {
                    "type": "string",
                    "description": "Time window (e.g. 2h, 24h)",
                },
                "min_importance": {
                    "type": "number",
                    "description": "Minimum importance score 0.0 to 1.0",
                },
                "limit": {
                    "type": "integer",
                    "description": "Max results. Default: 50",
                },
                "agent_id": {
                    "type": "string",
                    "description": "Override default agent ID",
                },
            },
            "required": [],
            "additionalProperties": False,
        },
        on_invoke_tool=on_invoke,
        strict_json_schema=False,
    )


def _make_decide_tool(
    client: AsyncMemtrace,
    default_agent_id: str,
) -> FunctionTool:
    async def on_invoke(ctx: ToolContext[Any], args_json: str) -> str:
        args = json.loads(args_json)
        mem = await client.decide(
            args.get("agent_id", default_agent_id),
            args["decision"],
            args["reasoning"],
        )
        return json.dumps(mem.model_dump(mode="json"), default=str)

    return FunctionTool(
        name="memtrace_decide",
        description=(
            "Log a decision with reasoning. "
            "Creates an auditable record of what was decided and why."
        ),
        params_json_schema={
            "type": "object",
            "properties": {
                "decision": {
                    "type": "string",
                    "description": "The decision that was made",
                },
                "reasoning": {
                    "type": "string",
                    "description": "Why this decision was made",
                },
                "agent_id": {
                    "type": "string",
                    "description": "Override default agent ID",
                },
            },
            "required": ["decision", "reasoning"],
            "additionalProperties": False,
        },
        on_invoke_tool=on_invoke,
        strict_json_schema=False,
    )
