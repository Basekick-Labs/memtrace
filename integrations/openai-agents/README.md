# OpenAI Agents + Memtrace

Memory tools and session store for the [OpenAI Agents SDK](https://github.com/openai/openai-agents-python), powered by [Memtrace](https://memtrace.ai).

## Installation

```bash
pip install openai-agents-memtrace
```

## Quick Start

```python
from agents import Agent, Runner
from memtrace import AsyncMemtrace
from openai_agents_memtrace import create_memtrace_tools, MemtraceSession

async def main():
    # 1. Create Memtrace client
    client = AsyncMemtrace("http://localhost:9100", "mtk_your_api_key")

    # 2. Create memory tools bound to an agent
    tools = create_memtrace_tools(client, agent_id="support_agent")

    # 3. Create a session with prior memory context
    session = await MemtraceSession.create(client, agent_id="support_agent")

    # 4. Create an agent with memory tools
    agent = Agent(
        name="Support Agent",
        instructions=(
            "You are a helpful support agent. "
            "Use memtrace_remember to store important information and "
            "memtrace_recall to check what you've seen before."
        ),
        tools=tools,
    )

    # 5. Run the agent
    result = await Runner.run(agent, "I need help with my account", session=session)
    print(result.final_output)

    # 6. Clean up
    await session.close()
    await client.close()
```

## Tools

`create_memtrace_tools(client, agent_id)` returns 4 tools:

| Tool | Description |
|------|-------------|
| `memtrace_remember` | Store a memory (actions, observations, events) |
| `memtrace_recall` | Retrieve recent memories (reverse chronological) |
| `memtrace_search` | Search memories by content, tags, types, importance |
| `memtrace_decide` | Log a decision with reasoning (audit trail) |

All tools use the configured `agent_id` by default. Pass `agent_id` as a parameter to any tool for cross-agent shared memory.

```python
# Bind tools to agent + optional session
tools = create_memtrace_tools(client, agent_id="my_agent", session_id="sess_1")
```

## Session

`MemtraceSession` implements `SessionABC` from the OpenAI Agents SDK. It stores conversation history in-memory while persisting significant events as Memtrace memories.

```python
# Create a session (calls Memtrace API)
session = await MemtraceSession.create(
    client,
    agent_id="my_agent",
    metadata={"task": "onboarding"},       # session metadata
    inject_context=True,                    # inject prior memories (default: True)
    context_since="24h",                    # prior context time window
    context_max_tokens=4000,                # max tokens for injected context
    persist_user_messages=True,             # store user messages as memories (default)
    persist_assistant_messages=False,        # store assistant messages (default: False)
)

# Use with Runner
result = await Runner.run(agent, "Hello", session=session)

# Close when done
await session.close()
```

### Cross-Session Memory

When `inject_context=True` (default), the session fetches prior memory context from Memtrace and injects it as the first conversation item. This gives the agent continuity across sessions without manual history management.

## Multi-tenant Deployments

A Memtrace deployment can serve multiple organizations, each routed to its own Arc instance. The integration is unchanged — you still pass an `AsyncMemtrace(base_url, api_key)` client. Memtrace looks up the organization that owns your API key and forwards reads and writes to that org's Arc instance automatically. Provision each organization with the `memtrace org` and `memtrace key` admin CLI on the server.

## Error Handling

Memtrace SDK exceptions propagate from tool invocations:

```python
from memtrace import (
    MemtraceError,
    AuthenticationError,
    NotFoundError,
    NoArcInstanceError,
)

try:
    result = await Runner.run(agent, "Hello", session=session)
except AuthenticationError:
    print("Invalid Memtrace API key")
except NoArcInstanceError:
    # The Memtrace org bound to this API key has no Arc instance configured.
    # Ask an admin to run `memtrace org add-arc <org_id>`.
    print("Memtrace is not provisioned for this org yet")
except MemtraceError as e:
    print(f"Memtrace error ({e.status_code}): {e.message}")
```

## Development

```bash
cd integrations/openai-agents
pip install -e ".[dev]"
pytest -v
ruff check src/ tests/
```

## License

MIT
