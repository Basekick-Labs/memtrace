# Memtrace Python SDK

Python client for [Memtrace](https://memtrace.ai) — LLM-agnostic memory layer for AI agents.

## Installation

```bash
pip install memtrace-sdk
```

## Quick Start

```python
from memtrace import Memtrace

client = Memtrace("http://localhost:9100", "mtk_your_api_key")

# Store a memory
client.remember("agent_1", "User prefers dark mode")

# Recall recent memories
memories = client.recall("agent_1", since="24h")
for m in memories.memories:
    print(f"[{m.time}] {m.content}")

# Log a decision
client.decide("agent_1", "Use PostgreSQL", "Better JSON support for metadata")
```

## Async Support

```python
from memtrace import AsyncMemtrace

async with AsyncMemtrace("http://localhost:9100", "mtk_your_api_key") as client:
    await client.remember("agent_1", "User prefers dark mode")
    memories = await client.recall("agent_1")
```

## Full API

### Memory Operations

```python
from memtrace import Memtrace, AddMemoryRequest, ListOptions, SearchQuery

client = Memtrace("http://localhost:9100", "mtk_...")

# Add a single memory with full control
mem = client.add_memory(AddMemoryRequest(
    agent_id="agent_1",
    session_id="sess_1",
    memory_type="episodic",
    event_type="observation",
    content="User clicked the settings button",
    tags=["ui", "navigation"],
    importance=0.7,
))

# Add multiple memories in a batch
memories = client.add_memories([
    AddMemoryRequest(agent_id="agent_1", memory_type="episodic", event_type="general", content="First"),
    AddMemoryRequest(agent_id="agent_1", memory_type="episodic", event_type="general", content="Second"),
])

# List with filters
result = client.list_memories(ListOptions(
    agent_id="agent_1",
    memory_type="decision",
    since="7d",
    limit=50,
    order="desc",
))

# Search with structured query
result = client.search_memories(SearchQuery(
    agent_id="agent_1",
    memory_types=["episodic", "decision"],
    content_contains="dark mode",
    min_importance=0.5,
))
```

### Agent Management

```python
from memtrace import Memtrace, RegisterAgentRequest

client = Memtrace("http://localhost:9100", "mtk_...")

# Register an agent
agent = client.register_agent(RegisterAgentRequest(
    name="my-agent",
    description="Handles customer support",
    config={"model": "gpt-4"},
))

# Get agent details
agent = client.get_agent("agent_1")

# Get agent memory stats
stats = client.get_agent_stats("agent_1")
print(f"Total memories: {stats.memory_count}")
print(f"Active sessions: {stats.active_sessions}")
```

### Session Management

```python
from memtrace import Memtrace, CreateSessionRequest, ContextOptions

client = Memtrace("http://localhost:9100", "mtk_...")

# Create a session
session = client.create_session(CreateSessionRequest(
    agent_id="agent_1",
    metadata={"task": "onboarding"},
))

# Get LLM-formatted context
ctx = client.get_session_context(session.id, ContextOptions(
    since="2h",
    include_types=["episodic", "decision"],
    max_tokens=4000,
))
print(ctx.context)  # Markdown-formatted for LLM consumption

# Close the session
client.close_session(session.id)
```

## Error Handling

```python
from memtrace import Memtrace, MemtraceError, AuthenticationError, NotFoundError, ConflictError

client = Memtrace("http://localhost:9100", "mtk_...")

try:
    agent = client.get_agent("nonexistent")
except NotFoundError:
    print("Agent not found")
except AuthenticationError:
    print("Invalid API key")
except ConflictError:
    print("Duplicate resource")
except MemtraceError as e:
    print(f"API error ({e.status_code}): {e.message}")
```

## Development

```bash
cd sdks/python
pip install -e ".[dev]"
pytest -v
ruff check src/ tests/
```

## License

MIT
