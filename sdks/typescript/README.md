# Memtrace TypeScript SDK

TypeScript/Node.js client for [Memtrace](https://memtrace.ai) — LLM-agnostic memory layer for AI agents.

## Installation

```bash
npm install @basekick-labs/memtrace-sdk
```

## Quick Start

```typescript
import { Memtrace } from '@basekick-labs/memtrace-sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_your_api_key')

// Store a memory
await client.remember('agent_1', 'User prefers dark mode')

// Recall recent memories
const memories = await client.recall('agent_1', '24h')
for (const m of memories.memories) {
  console.log(`[${m.time}] ${m.content}`)
}

// Log a decision
await client.decide('agent_1', 'Use PostgreSQL', 'Better JSON support for metadata')
```

## Full API

### Memory Operations

```typescript
import { Memtrace } from '@basekick-labs/memtrace-sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_...')

// Add a single memory with full control
const mem = await client.addMemory({
  agent_id: 'agent_1',
  session_id: 'sess_1',
  memory_type: 'episodic',
  event_type: 'observation',
  content: 'User clicked the settings button',
  tags: ['ui', 'navigation'],
  importance: 0.7,
})

// Add multiple memories in a batch
const memories = await client.addMemories([
  { agent_id: 'agent_1', memory_type: 'episodic', event_type: 'general', content: 'First' },
  { agent_id: 'agent_1', memory_type: 'episodic', event_type: 'general', content: 'Second' },
])

// List with filters
const result = await client.listMemories({
  agent_id: 'agent_1',
  memory_type: 'decision',
  since: '7d',
  limit: 50,
  order: 'desc',
})

// Search with structured query
const searchResult = await client.searchMemories({
  agent_id: 'agent_1',
  memory_types: ['episodic', 'decision'],
  content_contains: 'dark mode',
  min_importance: 0.5,
})
```

### Agent Management

```typescript
import { Memtrace } from '@basekick-labs/memtrace-sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_...')

// Register an agent
const agent = await client.registerAgent({
  name: 'my-agent',
  description: 'Handles customer support',
  config: { model: 'gpt-4' },
})

// Get agent details
const fetched = await client.getAgent('agent_1')

// Get agent memory stats
const stats = await client.getAgentStats('agent_1')
console.log(`Total memories: ${stats.memory_count}`)
console.log(`Active sessions: ${stats.active_sessions}`)
```

### Session Management

```typescript
import { Memtrace } from '@basekick-labs/memtrace-sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_...')

// Create a session
const session = await client.createSession({
  agent_id: 'agent_1',
  metadata: { task: 'onboarding' },
})

// Get LLM-formatted context
const ctx = await client.getSessionContext(session.id, {
  since: '2h',
  include_types: ['episodic', 'decision'],
  max_tokens: 4000,
})
console.log(ctx.context) // Markdown-formatted for LLM consumption

// List sessions (optionally filtered by agent)
const all = await client.listSessions()
const forAgent = await client.listSessions('agent_1')

// Close the session
await client.closeSession(session.id)
```

## Error Handling

```typescript
import { Memtrace, MemtraceError, AuthenticationError, NotFoundError, ConflictError } from '@basekick-labs/memtrace-sdk'

const client = new Memtrace('http://localhost:9100', 'mtk_...')

try {
  const agent = await client.getAgent('nonexistent')
} catch (e) {
  if (e instanceof NotFoundError) {
    console.log('Agent not found')
  } else if (e instanceof AuthenticationError) {
    console.log('Invalid API key')
  } else if (e instanceof ConflictError) {
    console.log('Duplicate resource')
  } else if (e instanceof MemtraceError) {
    console.log(`API error (${e.statusCode}): ${e.message}`)
  }
}
```

## Configuration

```typescript
const client = new Memtrace('http://localhost:9100', 'mtk_...', {
  timeout: 10_000, // Request timeout in ms (default: 30000)
})
```

## Requirements

- Node.js >= 18 (uses native `fetch`)
- Zero runtime dependencies

## Development

```bash
cd sdks/typescript
npm install
npm run build
npm test
```

## License

MIT
