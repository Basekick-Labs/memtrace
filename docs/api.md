# API Reference

Base URL: `http://localhost:9100`
Auth: `x-api-key: mtk_...` header or `Authorization: Bearer mtk_...`

The API key identifies the caller's organization. Memtrace routes every request to that org's Arc instance. If the org has no Arc instance configured yet, requests return:

```http
503 Service Unavailable
```
```json
{
  "error": "no arc instance configured for this org",
  "hint": "ask an admin to run `memtrace org add-arc <org_id>`"
}
```

## Health

### GET /health

Returns service status and the per-org Arc connection map. No auth required.

```json
{
  "status": "ok",
  "service": "memtrace",
  "uptime": "2h30m15s",
  "arc": {
    "org_default": true,
    "org_acme": true
  }
}
```

The `arc` object reports the last-known health of each org's Arc instance from the background health check.

### GET /ready

Returns `200` only when every configured Arc instance is reachable. No auth required.

```json
{
  "status": "ready",
  "arc": {
    "org_default": true,
    "org_acme": true
  }
}
```

When at least one Arc instance is unreachable, returns `503`:

```json
{
  "status": "not ready",
  "arc": {
    "org_default": true,
    "org_acme": false
  }
}
```

When no Arc instances are configured at all (fresh deployment), returns `503` with a hint:

```json
{
  "status": "not ready",
  "reason": "no_instances",
  "hint": "configure an Arc instance with `memtrace org add-arc`"
}
```

## Memories

### POST /api/v1/memories

Create a memory (single or batch).

**Single:**

```json
{
  "agent_id": "agent_web_crawler",
  "session_id": "sess_abc123",
  "memory_type": "episodic",
  "event_type": "page_crawled",
  "content": "Crawled https://example.com — found 3 product pages",
  "metadata": {"url": "https://example.com", "pages_found": 3},
  "tags": ["crawling", "products"],
  "importance": 0.7,
  "dedup_key": "crawl_example_2026-02-07"
}
```

**Batch:**

```json
{
  "memories": [
    {"agent_id": "agent_1", "content": "First memory", "memory_type": "episodic"},
    {"agent_id": "agent_1", "content": "Second memory", "memory_type": "decision"}
  ]
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `agent_id` | string | yes | — | Agent identifier |
| `content` | string | yes | — | Memory content |
| `memory_type` | string | no | `episodic` | `episodic`, `session`, `decision`, `entity` |
| `event_type` | string | no | `general` | App-defined event type |
| `session_id` | string | no | — | Session scope |
| `metadata` | object | no | — | Arbitrary key-value data |
| `tags` | string[] | no | — | Tags for filtering |
| `importance` | float | no | 0 | 0.0 to 1.0 |
| `dedup_key` | string | no | auto | Deduplication key |
| `parent_id` | string | no | — | Parent memory for threading |

**Response:** `201 Created`

```json
{
  "time": "2026-02-07T20:15:00Z",
  "org_id": "org_default",
  "agent_id": "agent_web_crawler",
  "content": "Crawled https://example.com — found 3 product pages",
  "memory_type": "episodic",
  "event_type": "page_crawled",
  "dedup_key": "a1b2c3d4..."
}
```

### GET /api/v1/memories

List memories with filters.

| Param | Type | Description |
|-------|------|-------------|
| `agent_id` | string | Filter by agent |
| `session_id` | string | Filter by session |
| `memory_type` | string | Filter by type |
| `event_type` | string | Filter by event type |
| `tags` | string | Comma-separated tags (AND) |
| `since` | string | Relative (`2h`, `7d`) or ISO8601 |
| `until` | string | Relative or ISO8601 |
| `limit` | int | Max results (default 100, max 1000) |
| `offset` | int | Pagination offset |
| `order` | string | `asc` or `desc` (default) |

**Response:**

```json
{
  "memories": [...],
  "count": 42,
  "has_more": true
}
```

### GET /api/v1/memories/:id

Get a specific memory by ID.

## Agents

### POST /api/v1/agents

Register a new agent.

```json
{
  "name": "web_crawler",
  "description": "Crawls product pages",
  "config": {"max_depth": 3}
}
```

### GET /api/v1/agents

List all agents in the organization.

### GET /api/v1/agents/:id

Get a specific agent.

### GET /api/v1/agents/:id/stats

Get memory statistics for an agent.

```json
{
  "agent_id": "agent_abc",
  "memory_count": 1523,
  "memories_24h": 47,
  "errors_24h": 2,
  "session_count": 12,
  "active_sessions": 1,
  "memory_types": {
    "episodic": 1200,
    "decision": 300,
    "entity": 23
  }
}
```

### DELETE /api/v1/agents/:id

Delete an agent.

## Sessions

### POST /api/v1/sessions

Create a new session.

```json
{
  "agent_id": "agent_abc",
  "metadata": {"task": "product_research"}
}
```

### GET /api/v1/sessions

List sessions. Optional `agent_id` query param to filter.

### GET /api/v1/sessions/:id

Get a specific session.

### PUT /api/v1/sessions/:id

Update session status or metadata.

```json
{"status": "closed"}
```

### POST /api/v1/sessions/:id/context

Get LLM-ready formatted context for a session.

```json
{
  "since": "4h",
  "include_types": ["episodic", "decision"],
  "max_tokens": 2000
}
```

**Response:**

```json
{
  "session_id": "sess_abc",
  "context": "## Session Context (sess_abc)\n\n### Recent Actions (12)\n...",
  "memory_count": 15
}
```

## Search

### POST /api/v1/search

Search memories with structured filters.

```json
{
  "agent_id": "agent_abc",
  "memory_types": ["episodic", "decision"],
  "event_types": ["page_crawled"],
  "tags": ["products"],
  "content_contains": "example.com",
  "min_importance": 0.5,
  "since": "7d",
  "order": "desc",
  "limit": 50
}
```

**Response:**

```json
{
  "results": [...],
  "count": 23,
  "query_time_ms": 12
}
```
