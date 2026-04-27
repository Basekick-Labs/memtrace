<p align="center">
  <img src="./assets/memtrace_logo.png" alt="Memtrace — Memory layer for AI agents" width="800" />
</p>

# Memtrace

**Multi-tenant memory layer for production AI agents — backed by [Arc](https://github.com/Basekick-Labs/arc) time-series DB.** Works with ChatGPT, Claude, Gemini, DeepSeek, Llama — any LLM.

No embeddings. No vector DB. Just fast, structured, temporal memory that any LLM can consume as plain text context.

## Is Memtrace for me?

Memtrace is **server-side and multi-tenant**, built for teams running fleets of AI agents in production:

- **Many agents, one memory pool** — call centers, SDR teams, multi-agent pipelines that need shared org-scoped memory
- **Many tenants, one deployment** — SaaS teams routing each customer org to its own Arc instance, with per-org API keys encrypted at rest
- **Long-running agents** — autonomous workers that run for hours or days and need durable, time-windowed recall
- **Time-series queries** — "what happened in the last 2 hours?" is a first-class operation, not a vector-similarity hack

Memtrace is **not** a per-developer local memory store for your IDE. If you want a single-binary tool that lives in your laptop's `.memtrace/` and gives Claude Code memory across chat sessions, that's a different product category — different deployment model, different threat model. Memtrace is the server you'd point those products at if you wanted to share memory across an organization, not their replacement.

## Why Memtrace?

AI agents need memory to be useful. They need to remember what they did, what worked, what failed, and what decisions they made. Most memory solutions force you into vector databases and embeddings — adding latency, complexity, and cost.

Memtrace takes a different approach: **operational, temporal memory** built on a time-series database. Every action is temporal. Every query is time-windowed. The feedback loop — Memory, Decision, Action, Log, Repeat — works naturally with time-series data.

## How It Works

Memtrace stores memories as time-series events in [Arc](https://github.com/Basekick-Labs/arc), a high-performance time-series database. Each memory has a type (`episodic`, `decision`, `entity`, `session`), tags, importance score, and metadata. Queries are time-windowed by default — "what happened in the last 2 hours?" is a first-class operation.

A single Memtrace deployment can serve many organizations, each routed to its own Arc instance with its own API key — encrypted at rest, selected automatically by the caller's API key. See the [Architecture](./architecture.md#multi-tenancy) doc for the multi-tenant data model.

The **session context** endpoint is the killer feature: it queries memories for a session, groups them by type, and returns LLM-ready markdown that you inject directly into any prompt. No parsing, no transformation — just paste it into your system prompt.

Read the full [Architecture](./architecture.md) doc for details on the data model, deduplication, write batching, and shared memory.

## Documentation

- [Architecture](./architecture.md) — How Memtrace works under the hood
- [API Reference](./api.md) — Complete REST API documentation
- [Configuration](./configuration.md) — All config options, environment variables, and deployment
- [MCP Server](./mcp.md) — Model Context Protocol server for Claude Code, Cursor, and more
- [OpenAPI Spec](./openapi.yaml) — OpenAPI 3.0 specification for all REST endpoints

## Use Cases

### Autonomous Agents
An AI agent that runs for hours or days — browsing the web, writing code, managing infrastructure. It needs to remember what it already tried, what failed, and what decisions it made so it doesn't repeat mistakes or contradict itself.

**Example:** A coding agent that refactors a large codebase across multiple sessions, remembering which files it already changed, which tests broke, and what strategies worked.

### Customer Support
AI support agents handling conversations across channels. Each agent remembers the full customer history — previous tickets, resolutions, preferences — without re-reading everything from scratch. Multiple agents share context about the same customer in real time.

**Example:** A call center with 50 AI agents sharing a memory pool. When a customer calls back, any agent instantly knows what happened last time.

### Research & Analysis
AI agents that crawl, summarize, and analyze data over time. They need to track what they've already read, what patterns they've found, and what conclusions they've drawn — building knowledge incrementally instead of starting from zero.

**Example:** A market research agent that monitors competitor pricing daily, remembering trends and flagging anomalies against its own historical observations.

### DevOps & Monitoring
AI agents that watch infrastructure, respond to alerts, and take remediation actions. They need to remember what they already investigated, which runbooks they executed, and what the outcomes were — especially during incident response.

**Example:** An on-call agent that correlates a 3 AM alert with a similar incident it handled last Tuesday, remembers the fix that worked, and applies it automatically.

### Content & Social Media
AI agents that create, schedule, and manage content across platforms. They remember what topics performed well, what's already been posted, and what the audience engaged with — avoiding repetition and learning from results.

**Example:** A social media agent that posted about Go generics yesterday and decides to cover a different topic today based on engagement memory.

### Multi-Agent Collaboration
Teams of specialized agents working on the same goal — one researches, one writes, one reviews, one publishes. They share a memory space so each agent can see what the others have done and make decisions accordingly.

**Example:** A content pipeline where a research agent stores findings, a writing agent reads them to draft articles, and an editor agent reviews against the shared decision log.

### Sales & Outreach
AI agents that manage prospect pipelines, personalize outreach, and track interactions over time. They remember every touchpoint, what messaging resonated, and when to follow up.

**Example:** An SDR agent that remembers a prospect mentioned a conference last month, and uses that context to personalize the follow-up email.

### Data Processing Pipelines
Long-running ETL or data enrichment agents that process millions of records in batches. They need to track what's been processed, what failed, and where to resume — with deduplication built in.

**Example:** A data enrichment agent that processes 100K company records over 3 days, remembering which ones are done, which APIs timed out, and which need retry.

## How clients connect

A Memtrace client points at exactly two things: the **deployment URL** and an **API key**. That's it.

```python
from memtrace import Memtrace

client = Memtrace(
    base_url="https://memtrace.example.com",   # one per Memtrace deployment
    api_key="mtk_..."                            # one per organization
)

client.remember(agent_id="my_agent", content="...")
```

Clients never name an organization or an Arc instance. The API key carries the org identity opaquely — Memtrace resolves it server-side and routes the request to that org's Arc instance, with that org's database and that org's API key. Operators provision orgs and Arc bindings on the server with `memtrace org` and `memtrace key`; clients only see the resulting `mtk_...` string.

This is the same shape as Stripe, OpenAI, AWS — **the API key is the tenant credential.**

### One client, multiple orgs

A single backend that needs to write on behalf of multiple Memtrace organizations holds one API key per org and routes between them in its own code. The SDK is unchanged.

```python
class TenantClients:
    def __init__(self):
        self._clients = {}

    def for_org(self, org_id: str) -> Memtrace:
        if org_id not in self._clients:
            api_key = secrets.get(f"memtrace_key_{org_id}")
            self._clients[org_id] = Memtrace("https://memtrace.example.com", api_key)
        return self._clients[org_id]

tenants.for_org("org_acme").remember(...)
tenants.for_org("org_voya").remember(...)
```

### What the client cannot do

- **Override the Arc routing.** A client cannot say "this request goes to a different Arc." If a process needs to talk to two Arcs, it holds two API keys (above).
- **Pick which org to write to.** The org is implied by the API key, never by a request parameter or header. This keeps the security boundary tight: a compromised client can only affect its own org.

### Operator workflow for a new tenant

```bash
# On the Memtrace server (admin CLI)
memtrace org create acme                      # → org_a1b2c3d4...
memtrace org add-arc org_a1b2c3d4... \
    --url https://arc-acme.example.com \
    --api-key <arc-key> \
    --database acme_memory
memtrace key create --org org_a1b2c3d4... --name acme-prod
# → mtk_xxx... (give this to the Acme team)
```

The Acme team then uses `Memtrace("https://memtrace.example.com", "mtk_xxx...")` — and every read and write lands in `arc-acme.example.com / acme_memory` automatically.

## Quick Start

### 1. Prerequisites

- Go 1.25+
- A running [Arc](https://github.com/Basekick-Labs/arc) instance (one or more — Memtrace is multi-tenant and can route different orgs to different Arc instances)

### 2. Install

```bash
git clone https://github.com/Basekick-Labs/memtrace.git
cd memtrace
make build
```

### 3. Generate a master key

Memtrace encrypts each org's Arc API key at rest using AES-256-GCM. Generate the master key once and put it in your secret manager — losing it makes encrypted secrets unrecoverable.

```bash
export MEMTRACE_MASTER_KEY=$(./memtrace keygen master)
```

The same value must be available to both `memtrace serve` and the `memtrace` admin CLI.

### 4. Run the server

The default `memtrace.toml` works as-is.

```bash
./memtrace serve   # or just `./memtrace`
```

On first run with auth enabled, Memtrace prints your admin API key for the bootstrap org. **Save it — it's shown only once.**

```
FIRST RUN: Save your admin API key (shown only once)
API Key: mtk_...
```

### 5. Configure an org and its Arc instance

If this is a fresh install, the bootstrap org (`org_default`) has no Arc instance yet — bind one:

```bash
./memtrace org add-arc org_default \
    --url http://localhost:8000 \
    --api-key <arc-api-key> \
    --database memory
```

To run multiple tenants on the same Memtrace deployment, create another org and point it at a different Arc:

```bash
./memtrace org create acme
# Organization created
#   id:   org_a1b2c3d4...
./memtrace org add-arc org_a1b2c3d4... \
    --url https://arc.acme.example.com \
    --api-key <arc-api-key> \
    --database acme_memory
./memtrace key create --org org_a1b2c3d4... --name acme-prod
```

Each API key is bound to one org, and every authenticated request is routed to that org's Arc instance.

### 6. Use it

```bash
# Store a memory
curl -X POST http://localhost:9100/api/v1/memories \
  -H "x-api-key: mtk_..." \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "my_agent",
    "content": "Crawled https://example.com — found 3 product pages",
    "memory_type": "episodic",
    "event_type": "page_crawled",
    "tags": ["crawling", "products"],
    "importance": 0.7
  }'

# Recall recent memories
curl "http://localhost:9100/api/v1/memories?agent_id=my_agent&since=2h" \
  -H "x-api-key: mtk_..."

# Get LLM-ready session context
curl -X POST http://localhost:9100/api/v1/sessions/sess_abc/context \
  -H "x-api-key: mtk_..." \
  -H "Content-Type: application/json" \
  -d '{"since": "4h", "include_types": ["episodic", "decision"]}'
```

> **Upgrading from a single-Arc deployment?** Memtrace auto-migrates a legacy `[arc]` block in `memtrace.toml` into the new schema on first startup. See the [Configuration guide](./configuration.md#auto-migration-from-the-legacy-arc-block).

## MCP Server (Claude Code, Cursor, etc.)

Memtrace ships an MCP server for integration with Claude Code, Claude Desktop, Cursor, Windsurf, Cline, Zed, and other MCP-compatible tools.

```bash
make build-mcp
```

Configure in Claude Code (`.mcp.json`):

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

7 tools are available: `memtrace_remember`, `memtrace_recall`, `memtrace_search`, `memtrace_decide`, `memtrace_session_create`, `memtrace_session_context`, `memtrace_agent_register`. See the [MCP docs](./mcp.md) for full details.

## Python SDK

```bash
pip install memtrace-sdk
```

```python
from memtrace import Memtrace

client = Memtrace("http://localhost:9100", "mtk_...")

# Quick add
client.remember("my_agent", "Posted tweet about Go generics")

# Recall recent
memories = client.recall("my_agent", since="48h")

# Log a decision
client.decide("my_agent", "post_to_twitter", "feed had interesting content")

# Full API
client.add_memory(AddMemoryRequest(...))
client.list_memories(ListOptions(...))
client.search_memories(SearchQuery(...))
client.create_session(CreateSessionRequest(...))
client.get_session_context(session_id, ContextOptions(...))
```

Async support: `from memtrace import AsyncMemtrace`. See the [Python SDK README](../sdks/python/README.md) for full docs.

## TypeScript SDK

```bash
npm install @memtrace/sdk
```

```typescript
import { Memtrace } from '@memtrace/sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_...')

// Quick add
await client.remember('my_agent', 'Posted tweet about Go generics')

// Recall recent
const memories = await client.recall('my_agent', '48h')

// Log a decision
await client.decide('my_agent', 'post_to_twitter', 'feed had interesting content')

// Full API
await client.addMemory({ ... })
await client.listMemories({ ... })
await client.searchMemories({ ... })
await client.createSession({ ... })
await client.getSessionContext(sessionId, { ... })
```

Zero runtime dependencies, native `fetch` (Node.js 18+). See the [TypeScript SDK README](../sdks/typescript/README.md) for full docs.

## Go SDK

```go
import "github.com/Basekick-Labs/memtrace/pkg/sdk"

client := sdk.New("http://localhost:9100", "mtk_...")

// Quick add
client.Remember(ctx, "my_agent", "Posted tweet about Go generics")

// Recall recent
memories, _ := client.Recall(ctx, "my_agent", "48h")

// Log a decision
client.Decide(ctx, "my_agent", "post_to_twitter", "feed had interesting content")

// Full API
client.AddMemory(ctx, &sdk.AddMemoryRequest{...})
client.ListMemories(ctx, &sdk.ListOptions{...})
client.SearchMemories(ctx, &sdk.SearchQuery{...})
client.CreateSession(ctx, &sdk.CreateSessionRequest{...})
client.GetSessionContext(ctx, sessionID, &sdk.ContextOptions{...})
```

## Framework Integrations

### OpenAI Agents SDK

```bash
pip install openai-agents-memtrace
```

```python
from agents import Agent, Runner
from memtrace import AsyncMemtrace
from openai_agents_memtrace import create_memtrace_tools, MemtraceSession

client = AsyncMemtrace("http://localhost:9100", "mtk_...")
tools = create_memtrace_tools(client, agent_id="my_agent")
session = await MemtraceSession.create(client, agent_id="my_agent")

agent = Agent(
    name="My Agent",
    instructions="Use memtrace_remember to store info, memtrace_recall to retrieve it.",
    tools=tools,
)

result = await Runner.run(agent, "Hello", session=session)
```

4 tools: `memtrace_remember`, `memtrace_recall`, `memtrace_search`, `memtrace_decide`. See the [OpenAI Agents integration README](../integrations/openai-agents/README.md) for full docs.

## Examples

### Claude API + Memtrace

Complete cookbook showing the core memory loop with the Anthropic Claude API:

```python
from memtrace import Memtrace, ContextOptions

mt = Memtrace("http://localhost:9100", "mtk_...")

# Get LLM-ready context and inject into Claude's system prompt
ctx = mt.get_session_context(session_id, ContextOptions(since="4h"))

response = anthropic.messages.create(
    model="claude-sonnet-4-20250514",
    system=f"You are an agent.\n\n{ctx.context}",
    tools=MEMTRACE_TOOLS,  # remember, recall, search, decide
    messages=[...],
)
```

Two runnable scripts: single-agent memory loop and multi-agent shared memory. See the [Claude cookbook](../examples/claude/) for full examples.

### OpenAI API + Memtrace

The same cookbook pattern using the OpenAI Python SDK and function calling:

```python
from openai import OpenAI
from memtrace import Memtrace, ContextOptions

mt = Memtrace("http://localhost:9100", "mtk_...")

# Get LLM-ready context and inject into the system prompt
ctx = mt.get_session_context(session_id, ContextOptions(since="4h"))

response = openai.chat.completions.create(
    model="gpt-4o",
    messages=[
        {"role": "system", "content": f"You are an agent.\n\n{ctx.context}"},
        ...
    ],
    tools=MEMTRACE_TOOLS,  # remember, recall, search, decide
)
```

Two runnable scripts: single-agent memory loop and multi-agent shared memory. See the [OpenAI cookbook](../examples/openai/) for full examples.

## License

Open source. See [LICENSE](../LICENSE) for details.
