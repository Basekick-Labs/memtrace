#!/usr/bin/env python3
"""Single-agent memory loop with OpenAI API and Memtrace.

Demonstrates the core memory pattern:
  1. Register an agent in Memtrace
  2. Create a session
  3. Inject prior memory context into the system prompt
  4. Run an agentic loop with tool use (remember, recall, search, decide)
  5. Persist memories across the conversation
  6. Close the session

Usage:
  export OPENAI_API_KEY="sk-..."
  export MEMTRACE_URL="http://localhost:9100"
  export MEMTRACE_API_KEY="mtk_..."
  python examples/openai/single_agent.py
"""

from __future__ import annotations

import json
import os
import sys

from openai import OpenAI
from memtrace import (
    AddMemoryRequest,
    ConflictError,
    ContextOptions,
    CreateSessionRequest,
    ListOptions,
    Memtrace,
    RegisterAgentRequest,
    SearchQuery,
)

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

OPENAI_API_KEY = os.environ.get("OPENAI_API_KEY", "")
MEMTRACE_URL = os.environ.get("MEMTRACE_URL", "http://localhost:9100")
MEMTRACE_API_KEY = os.environ.get("MEMTRACE_API_KEY", "")
MODEL = "gpt-4o"
AGENT_ID = "openai-cookbook-agent"
MAX_TOOL_ROUNDS = 10

# ---------------------------------------------------------------------------
# Memtrace tool definitions (OpenAI function calling format)
# ---------------------------------------------------------------------------

MEMTRACE_TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "memtrace_remember",
            "description": (
                "Store a memory. Use this to record observations, actions, events, "
                "or any information you should remember later."
            ),
            "parameters": {
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
                    "tags": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Tags for categorization",
                    },
                    "importance": {
                        "type": "number",
                        "description": "Importance score 0.0 to 1.0",
                    },
                },
                "required": ["content"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memtrace_recall",
            "description": (
                "Retrieve recent memories. "
                "Returns memories in reverse chronological order."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "since": {
                        "type": "string",
                        "description": "Time window (e.g. 2h, 24h, 7d). Default: 24h",
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
                },
                "required": [],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memtrace_search",
            "description": (
                "Search memories with structured filters: "
                "by content text, memory types, tags, importance, and time range."
            ),
            "parameters": {
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
                },
                "required": [],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "memtrace_decide",
            "description": (
                "Log a decision with reasoning. "
                "Creates an auditable record of what was decided and why."
            ),
            "parameters": {
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
                },
                "required": ["decision", "reasoning"],
            },
        },
    },
]

# ---------------------------------------------------------------------------
# Tool handler
# ---------------------------------------------------------------------------


def handle_tool_call(
    tool_name: str,
    tool_input: dict,
    mt: Memtrace,
    agent_id: str,
    session_id: str,
) -> str:
    """Execute a Memtrace tool call and return the result as a JSON string."""
    try:
        if tool_name == "memtrace_remember":
            mem = mt.add_memory(
                AddMemoryRequest(
                    agent_id=agent_id,
                    session_id=session_id,
                    content=tool_input["content"],
                    memory_type=tool_input.get("memory_type", "episodic"),
                    event_type=tool_input.get("event_type", "general"),
                    tags=tool_input.get("tags"),
                    importance=tool_input.get("importance"),
                )
            )
            return json.dumps(
                {"stored": True, "content": mem.content, "time": str(mem.time)}
            )

        if tool_name == "memtrace_recall":
            result = mt.list_memories(
                ListOptions(
                    agent_id=agent_id,
                    since=tool_input.get("since", "24h"),
                    memory_type=tool_input.get("memory_type"),
                    limit=min(tool_input.get("limit", 50), 200),
                    order="desc",
                )
            )
            memories = [
                {"time": str(m.time), "content": m.content, "type": m.memory_type}
                for m in result.memories
            ]
            return json.dumps({"count": result.count, "memories": memories})

        if tool_name == "memtrace_search":
            result = mt.search_memories(
                SearchQuery(
                    agent_id=agent_id,
                    content_contains=tool_input.get("content_contains"),
                    memory_types=tool_input.get("memory_types"),
                    tags=tool_input.get("tags"),
                    since=tool_input.get("since"),
                    min_importance=tool_input.get("min_importance"),
                    limit=min(tool_input.get("limit", 50), 200),
                    order="desc",
                )
            )
            results = [
                {"time": str(m.time), "content": m.content, "type": m.memory_type}
                for m in result.results
            ]
            return json.dumps({"count": result.count, "results": results})

        if tool_name == "memtrace_decide":
            mem = mt.decide(agent_id, tool_input["decision"], tool_input["reasoning"])
            return json.dumps(
                {"logged": True, "decision": mem.content, "time": str(mem.time)}
            )

        return json.dumps({"error": f"Unknown tool: {tool_name}"})
    except Exception as exc:
        return json.dumps({"error": str(exc)})


# ---------------------------------------------------------------------------
# Agentic loop
# ---------------------------------------------------------------------------


def run_agent_loop(
    client: OpenAI,
    mt: Memtrace,
    system_prompt: str,
    messages: list[dict],
    agent_id: str,
    session_id: str,
) -> str:
    """Run the OpenAI tool-use loop until the model stops calling tools.

    Returns the final assistant text response.
    """
    for _round in range(MAX_TOOL_ROUNDS):
        response = client.chat.completions.create(
            model=MODEL,
            messages=[{"role": "system", "content": system_prompt}] + messages,
            tools=MEMTRACE_TOOLS,
        )

        choice = response.choices[0]
        message = choice.message

        # Append assistant response to conversation history
        messages.append(message.to_dict())

        # If no tool calls, return the text response
        if choice.finish_reason != "tool_calls":
            return message.content or ""

        # Process tool calls
        for tool_call in message.tool_calls:
            func = tool_call.function
            tool_input = json.loads(func.arguments)
            print(f"  [tool] {func.name}({json.dumps(tool_input)[:80]})")
            result = handle_tool_call(
                func.name, tool_input, mt, agent_id, session_id
            )
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": result,
                }
            )

    return "[Max tool rounds reached]"


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------


def main() -> None:
    if not OPENAI_API_KEY:
        print("Error: OPENAI_API_KEY environment variable is not set.")
        sys.exit(1)
    if not MEMTRACE_API_KEY:
        print("Error: MEMTRACE_API_KEY environment variable is not set.")
        sys.exit(1)

    client = OpenAI(api_key=OPENAI_API_KEY)
    mt = Memtrace(MEMTRACE_URL, MEMTRACE_API_KEY)

    try:
        # Step 1: Register agent (idempotent)
        print("1. Registering agent...")
        try:
            agent = mt.register_agent(
                RegisterAgentRequest(
                    name=AGENT_ID,
                    description="OpenAI cookbook demo agent",
                    config={"model": MODEL},
                )
            )
        except ConflictError:
            agent = mt.get_agent(AGENT_ID)
        print(f"   Agent: {agent.id}")

        # Step 2: Create session
        print("2. Creating session...")
        session = mt.create_session(
            CreateSessionRequest(
                agent_id=agent.id,
                metadata={"cookbook": "openai-single-agent"},
            )
        )
        print(f"   Session: {session.id}")

        # Step 3: Load prior memory context
        print("3. Loading prior memory context...")
        ctx = mt.get_session_context(session.id, ContextOptions(since="4h"))
        context_block = ""
        if ctx.context and ctx.memory_count > 0:
            context_block = f"\n\n## Prior Memory\n{ctx.context}"
            print(f"   Loaded {ctx.memory_count} prior memories")
        else:
            print("   No prior memories (first run)")

        # Step 4: Build system prompt
        system_prompt = (
            "You are an AI assistant with persistent memory. "
            "You can store observations and decisions using your memory tools.\n\n"
            "Guidelines:\n"
            "- Use memtrace_remember to store important facts the user tells you.\n"
            "- Use memtrace_recall to check what you already know before answering.\n"
            "- Use memtrace_decide when you make a significant recommendation.\n"
            "- Always recall existing memories before answering questions about "
            "the user."
            f"{context_block}"
        )

        # Step 5: Run conversation
        print("\n4. Starting conversation...\n")
        messages: list[dict] = []
        prompts = [
            "My name is Alex and I work at Acme Corp. I prefer Python over JavaScript.",
            "What do you remember about me?",
            "Based on what you know about me, recommend a tech stack for my next project.",
        ]

        for user_msg in prompts:
            print(f"User: {user_msg}")
            messages.append({"role": "user", "content": user_msg})
            response = run_agent_loop(
                client, mt, system_prompt, messages, agent.id, session.id
            )
            print(f"\nAssistant: {response}\n")
            print("-" * 60)

        # Step 6: Close session
        print("\n5. Closing session...")
        mt.close_session(session.id)
        print("   Done!")

    finally:
        mt.close()


if __name__ == "__main__":
    main()
