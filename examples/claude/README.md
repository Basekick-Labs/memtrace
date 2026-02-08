# Claude API + Memtrace Cookbook

Complete examples showing how to use [Memtrace](https://github.com/Basekick-Labs/memtrace) with the [Anthropic Claude API](https://docs.anthropic.com/) for agent memory.

## Prerequisites

- Python 3.10+
- A running [Memtrace](https://github.com/Basekick-Labs/memtrace) instance (with [Arc](https://github.com/Basekick-Labs/arc) backend)
- An [Anthropic API key](https://console.anthropic.com/)

## Setup

```bash
pip install -r examples/claude/requirements.txt
```

Set environment variables:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
export MEMTRACE_URL="http://localhost:9100"    # default
export MEMTRACE_API_KEY="mtk_..."
```

## Examples

### Single Agent (`single_agent.py`)

Demonstrates the core memory loop with a single Claude-powered agent:

1. **Register** an agent in Memtrace
2. **Create** a session
3. **Inject** prior memory context into Claude's system prompt
4. **Converse** with tool use — the agent stores facts, recalls them, and makes decisions
5. **Close** the session

```bash
python examples/claude/single_agent.py
```

The agent receives 3 user messages that exercise the full memory cycle: storing a fact, recalling it, and making a recommendation based on stored context.

### Multi-Agent (`multi_agent.py`)

Demonstrates two agents sharing a Memtrace memory space:

1. **Researcher** investigates a topic and stores findings with tags and importance scores
2. **Summarizer** reads the researcher's memories and produces a structured report

Both agents use the same Memtrace session, so the summarizer can see everything the researcher stored.

```bash
python examples/claude/multi_agent.py
```

This pattern enables agent pipelines, handoffs, and collaborative workflows where specialized agents build on each other's work.

## Memory Loop Pattern

Both examples follow the same core pattern:

```
┌─────────────────────────────────────────┐
│  1. Get session context from Memtrace   │
│     (LLM-ready markdown)                │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  2. Inject context into Claude's        │
│     system prompt                       │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  3. Claude acts — calls memory tools    │
│     (remember, recall, search, decide)  │
└──────────────────┬──────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────┐
│  4. Results persisted to Memtrace       │
│     → available in next session         │
└─────────────────────────────────────────┘
```

```python
# The pattern in code:
ctx = memtrace.get_session_context(session_id, ContextOptions(since="4h"))

response = anthropic.messages.create(
    model="claude-sonnet-4-20250514",
    system=f"You are an agent.\n\n{ctx.context}",
    tools=MEMTRACE_TOOLS,
    messages=[...],
)

# Handle tool_use blocks → execute Memtrace SDK calls → return tool_result
# Loop until stop_reason == "end_turn"
```

## Tools

Both examples provide 4 Memtrace tools to Claude:

| Tool | Purpose |
|------|---------|
| `memtrace_remember` | Store a memory (observation, action, event) |
| `memtrace_recall` | Retrieve recent memories (reverse chronological) |
| `memtrace_search` | Search with filters (content, tags, types, importance) |
| `memtrace_decide` | Log a decision with reasoning (audit trail) |

These match the tools available in the [MCP server](../../docs/mcp.md) and the [OpenAI Agents integration](../../integrations/openai-agents/).
