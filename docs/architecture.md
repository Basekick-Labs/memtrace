# Architecture

## Overview

Memtrace is a Go microservice that provides a memory layer for AI agents. It connects to a running [Arc](https://github.com/Basekick-Labs/arc) time-series database instance over HTTP.

```
Client App  --[API key]--> Memtrace --[Arc API key]--> Arc
```

- **Writes** go to Arc via `POST /api/v1/write/msgpack` (columnar msgpack format)
- **Queries** go to Arc via `POST /api/v1/query` (SQL over Parquet)
- **Metadata** (sessions, agents, API keys) lives in a local SQLite database

## Data Model

### Arc (time-series data)

All memories are stored in a single `events` measurement with columns for filtering:

| Column | Type | Description |
|--------|------|-------------|
| `time` | TIMESTAMP | Auto-set, nanosecond precision |
| `org_id` | VARCHAR | Organization/tenant ID |
| `agent_id` | VARCHAR | Agent that created this memory |
| `session_id` | VARCHAR | Session scope (empty if unscoped) |
| `memory_type` | VARCHAR | `episodic` / `session` / `decision` / `entity` |
| `event_type` | VARCHAR | App-defined (e.g., `page_crawled`, `error`) |
| `content` | VARCHAR | Primary content text |
| `metadata_json` | VARCHAR | JSON-encoded arbitrary key-value data |
| `tags_csv` | VARCHAR | Comma-separated tags |
| `dedup_key` | VARCHAR | Deduplication key (SHA256) |
| `importance` | DOUBLE | 0.0-1.0 score |
| `parent_id` | VARCHAR | Link to parent memory (threading) |

### SQLite (metadata)

Sessions, agents, organizations, and API keys are stored in SQLite:

- **organizations** — Multi-tenant support
- **agents** — Registered agents with config
- **sessions** — Bounded work contexts with lifecycle
- **api_keys** — bcrypt-hashed keys with `mtk_` prefix

## Memory Types

| Type | Use Case |
|------|----------|
| `episodic` | Actions taken, events observed, things that happened |
| `session` | Session-scoped context and state |
| `decision` | Decisions with reasoning (audit trail) |
| `entity` | Facts about entities (people, tools, systems) |

## Key Features

### Deduplication

Memories are deduplicated using a SHA256 key derived from `agent_id + event_type + content[:200]`. Before writing, Memtrace checks Arc for an existing memory with the same key within a configurable time window (default: 24h). This prevents agents from logging duplicate actions.

### Session Context

The killer feature. `POST /api/v1/sessions/:id/context` queries Arc for session memories and returns LLM-ready markdown, grouped by type:

```markdown
## Session Context (sess_abc123)

### Recent Actions (12)
- [2026-02-07T20:15:00Z] page_crawled: Crawled https://example.com — found 3 pro...
- [2026-02-07T20:14:30Z] api_call: Called OpenAI API for summarization...

### Decisions Made (3)
- [2026-02-07T20:14:00Z] decision: Skip pagination — only 2 pages deep...

### Errors (1)
- [2026-02-07T20:13:00Z] error: Rate limited by target API, backing off...
```

This formatted context can be injected directly into any LLM prompt — ChatGPT, Claude, Gemini, or any other model.

### Shared Memory

Multiple agents can share memories through:
- **Organization scope** — All agents in an org see each other's memories
- **Session sharing** — Multiple agents can write to the same session
- **Tag-based filtering** — Agents query for memories tagged with relevant topics

This enables use cases like call center agents sharing customer context, or a team of specialized agents collaborating on a complex task.

### Write Batching

Writes are buffered in-memory and flushed to Arc in batches (configurable size and interval). This provides high write throughput without overwhelming Arc.

## Project Structure

```
memtrace/
├── cmd/
│   ├── memtrace/          # Main server entry point
│   └── mcp/               # MCP server (stdio, CGO_ENABLED=0)
├── internal/
│   ├── api/               # HTTP handlers (Fiber)
│   ├── arc/               # Arc HTTP client
│   ├── auth/              # API key management
│   ├── config/            # TOML/env config
│   ├── memory/            # Core memory logic
│   ├── session/           # Session management
│   ├── agent/             # Agent registry
│   ├── metadata/          # SQLite metadata store
│   └── sanitize/          # Input validation + SQL escaping
├── pkg/sdk/               # Go SDK (public)
├── sdks/python/           # Python SDK (PyPI: memtrace-sdk)
├── sdks/typescript/       # TypeScript SDK (npm: @memtrace/sdk)
├── integrations/openai-agents/  # OpenAI Agents SDK integration
├── memtrace.toml          # Default config
├── Dockerfile
└── Makefile
```
