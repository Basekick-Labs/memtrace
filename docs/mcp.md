# MCP Server

Memtrace ships an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server that exposes memory tools over stdio. This enables integration with any MCP-compatible client — Claude Code, Claude Desktop, Cursor, Windsurf, Cline, Zed, VS Code Copilot, Gemini CLI, and more.

The MCP server is a thin adapter: it receives tool calls over stdio and forwards them to a running Memtrace instance via the [Go SDK](../pkg/sdk/).

## Prerequisites

- A running Memtrace instance
- A Memtrace API key (`mtk_...`)

## Build

```bash
make build-mcp
```

This produces a `memtrace-mcp` binary. The MCP server is built with `CGO_ENABLED=0` — no C dependencies, easy to cross-compile and distribute.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MEMTRACE_URL` | `http://localhost:9100` | Memtrace instance URL |
| `MEMTRACE_API_KEY` | (required) | Memtrace API key |

## Integration

### Claude Code

Add to your Claude Code MCP settings (`~/.claude/settings.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "memtrace": {
      "command": "/path/to/memtrace-mcp",
      "env": {
        "MEMTRACE_URL": "http://localhost:9100",
        "MEMTRACE_API_KEY": "mtk_..."
      }
    }
  }
}
```

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "memtrace": {
      "command": "/path/to/memtrace-mcp",
      "env": {
        "MEMTRACE_URL": "http://localhost:9100",
        "MEMTRACE_API_KEY": "mtk_..."
      }
    }
  }
}
```

### Cursor / Windsurf / Other MCP Clients

The pattern is the same — point the MCP client at the `memtrace-mcp` binary with the required env vars. Consult your client's documentation for the exact config file location.

## Tools

The MCP server exposes 7 tools, consolidated for LLM ergonomics (fewer tools = better tool selection by the model).

### memtrace_remember

Store a memory. Use this to record actions, observations, events, or any information the agent should remember later.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `agent_id` | string | yes | — | Agent ID |
| `content` | string | yes | — | Memory content text |
| `memory_type` | string | no | `episodic` | `episodic`, `decision`, `entity`, `session` |
| `event_type` | string | no | `general` | Event type (e.g. `page_crawled`, `error`, `api_call`) |
| `session_id` | string | no | — | Session ID to scope this memory to |
| `tags` | string[] | no | — | Tags for categorization |
| `importance` | float | no | 0 | Importance score 0.0 to 1.0 |
| `metadata` | object | no | — | Arbitrary key-value metadata |

### memtrace_recall

Retrieve recent memories for an agent. Returns memories in reverse chronological order.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `agent_id` | string | yes | — | Agent ID |
| `since` | string | no | `24h` | Time window (e.g. `2h`, `24h`, `7d`) |
| `session_id` | string | no | — | Filter by session ID |
| `memory_type` | string | no | — | Filter by memory type |
| `limit` | int | no | 50 | Max results |

### memtrace_search

Search memories with structured filters: by content text, memory types, tags, importance, and time range.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `agent_id` | string | no | — | Filter by agent ID |
| `content_contains` | string | no | — | Search text within memory content |
| `memory_types` | string[] | no | — | Filter by memory types |
| `tags` | string[] | no | — | Filter by tags |
| `since` | string | no | — | Time window (e.g. `2h`, `24h`) |
| `min_importance` | float | no | — | Minimum importance score 0.0 to 1.0 |
| `limit` | int | no | 50 | Max results |

### memtrace_decide

Log a decision with reasoning. Creates an auditable record of what was decided and why.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `agent_id` | string | yes | — | Agent ID |
| `decision` | string | yes | — | The decision that was made |
| `reasoning` | string | yes | — | Why this decision was made |

### memtrace_session_create

Start a new session — a bounded context for a unit of work. Memories can be scoped to sessions.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `agent_id` | string | yes | — | Agent ID that owns this session |
| `metadata` | object | no | — | Session metadata (e.g. goal, context) |

### memtrace_session_context

Get LLM-ready session context as formatted markdown. Returns all memories for a session grouped by type, ready to inject into a prompt.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `session_id` | string | yes | — | Session ID |
| `since` | string | no | all | Time window (e.g. `4h`) |
| `include_types` | string[] | no | all | Memory types to include |
| `max_tokens` | int | no | — | Approximate max token budget for context |

### memtrace_agent_register

Register a new agent. Required before storing memories for that agent.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | yes | — | Agent name (used as the agent ID) |
| `description` | string | no | — | What this agent does |

## Example Workflow

Once configured, an LLM using Memtrace tools might:

1. **Register** itself: `memtrace_agent_register(name: "code-reviewer")`
2. **Create a session**: `memtrace_session_create(agent_id: "code-reviewer", metadata: {goal: "review PR #42"})`
3. **Remember** as it works: `memtrace_remember(agent_id: "code-reviewer", session_id: "sess_...", content: "Found SQL injection in auth.go:45")`
4. **Decide** on approach: `memtrace_decide(agent_id: "code-reviewer", decision: "Flag as critical", reasoning: "SQL injection is OWASP top 10")`
5. **Recall** on next session: `memtrace_recall(agent_id: "code-reviewer", since: "7d")` — remembers what it found before
6. **Get context**: `memtrace_session_context(session_id: "sess_...")` — inject full session history into prompt
