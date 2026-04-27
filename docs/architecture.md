# Architecture

## Overview

Memtrace is a Go microservice that provides a memory layer for AI agents. A single deployment can serve multiple organizations, each routed to its own [Arc](https://github.com/Basekick-Labs/arc) time-series database instance over HTTP.

```
                          ┌──> Arc (org_acme)
Client App  --[mtk_...]-->│
                Memtrace ─┼──> Arc (org_default)
                          └──> Arc (org_other)
```

- The Memtrace API key (`mtk_...`) identifies the caller's org.
- Memtrace looks up that org's Arc connection details (URL, API key, database, measurement) in its metadata DB and routes the request to the right Arc instance.
- **Writes** go to Arc via `POST /api/v1/write/msgpack` (columnar msgpack format).
- **Queries** go to Arc via `POST /api/v1/query` (SQL over Parquet).
- **Metadata** (orgs, Arc instance bindings, sessions, agents, API keys) lives in a local SQLite database.

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

Orgs, Arc-instance bindings, sessions, agents, and API keys are stored in SQLite:

- **organizations** — Tenant identity (id, name)
- **arc_instances** — Per-org Arc connection (URL, encrypted API key, database, measurement); `UNIQUE(org_id)`
- **api_keys** — bcrypt-hashed `mtk_` keys, each bound to an `org_id`
- **agents** — Registered agents with config (org-scoped)
- **sessions** — Bounded work contexts with lifecycle (org-scoped)

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
- **Organization scope** — All agents in an org see each other's memories (and use the same Arc instance)
- **Session sharing** — Multiple agents can write to the same session
- **Tag-based filtering** — Agents query for memories tagged with relevant topics

This enables use cases like call center agents sharing customer context, or a team of specialized agents collaborating on a complex task.

### Multi-tenancy

Memtrace is multi-tenant at both the data and the transport layer:

- **Auth ↔ org binding.** Every API key (`mtk_...`) is bound to exactly one `org_id`. The auth middleware extracts that `org_id` from the request and pins it into the request context — handlers never accept it from the body or URL.
- **Per-org Arc routing.** An Arc client registry holds one `*arc.Client` per org, built from the `arc_instances` table at startup. On every read or write, the manager resolves `arcRegistry.Get(orgID)` and uses that org's Arc instance, database, and API key. There is no global Arc client.
- **Encryption at rest.** Each org's Arc API key is stored AES-256-GCM-encrypted in `arc_instances.api_key_cipher`. The 32-byte master key comes from the `MEMTRACE_MASTER_KEY` environment variable; it is never written to disk by Memtrace. Tampering with a ciphertext fails decryption loudly at startup.
- **Filtered queries.** Even though each org has its own Arc database, every query Memtrace generates also filters by `org_id` as a defense-in-depth measure, so a misconfiguration cannot leak data between orgs.
- **Admin CLI.** Orgs and their Arc bindings are provisioned with `memtrace org create`, `memtrace org add-arc`, and `memtrace key create`. Commands operate directly on the metadata DB and work whether the server is up or not.

If a request arrives with an API key for an org that has no `arc_instances` row, the API returns `503` with a clear error pointing to `memtrace org add-arc` — no nil-pointer, no wrong-database write.

### Write Batching

Writes are buffered in-memory per Arc client and flushed in batches (configurable size and interval). Each org's batch is independent. This provides high write throughput without overwhelming Arc.

## Project Structure

```
memtrace/
├── cmd/
│   ├── memtrace/          # Server + admin CLI (cobra; `serve`, `keygen`, `org`, `key`)
│   └── mcp/               # MCP server (stdio, CGO_ENABLED=0)
├── internal/
│   ├── api/               # HTTP handlers (Fiber)
│   ├── arc/               # Arc HTTP client + per-org Registry
│   ├── auth/              # API key management
│   ├── config/            # TOML/env config
│   ├── crypto/            # AES-GCM envelope encryption for Arc keys
│   ├── memory/            # Core memory logic
│   ├── session/           # Session management
│   ├── agent/             # Agent registry
│   ├── metadata/          # SQLite metadata store (orgs, arc_instances, keys, agents, sessions)
│   └── sanitize/          # Input validation + SQL escaping
├── pkg/sdk/               # Go SDK (public)
├── sdks/python/           # Python SDK (PyPI: memtrace-sdk)
├── sdks/typescript/       # TypeScript SDK (npm: @memtrace/sdk)
├── integrations/openai-agents/  # OpenAI Agents SDK integration
├── examples/claude/       # Claude API cookbook (single + multi-agent)
├── examples/openai/       # OpenAI API cookbook (single + multi-agent)
├── memtrace.toml          # Default config
├── Dockerfile
└── Makefile
```
