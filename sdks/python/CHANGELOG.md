# Changelog

All notable changes to `memtrace-sdk` (Python) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-04-27

### Added
- `NoArcInstanceError` exception, raised on `503` responses when the caller's organization has no Arc instance configured. Memtrace deployments are multi-tenant; an administrator must run `memtrace org add-arc <org_id>` before that organization can read or write memories. Subclass of `MemtraceError`.
- README note about multi-tenant deployments: one Memtrace deployment can serve multiple organizations, each routed to its own Arc instance, selected automatically by the API key.

## [0.1.1] - 2026-04-16

### Security
- Percent-encode path parameters (`agent_id`, `session_id`) with `urllib.parse.quote` to prevent URL structure tampering when IDs contain `/`, spaces, or other unsafe characters. Affects `get_agent`, `get_agent_stats`, `get_session`, `get_session_context`, `close_session` (sync and async).

### Added
- `list_agents()` / `list_agents` async — `GET /api/v1/agents`
- `get_agent_memories(agent_id, opts)` (sync + async) — `GET /api/v1/agents/{id}/memories`
- `get_session_memories(session_id, opts)` (sync + async) — `GET /api/v1/sessions/{id}/memories`
- `delete_agent(agent_id)` (sync + async) — `DELETE /api/v1/agents/{id}`
- `AgentList` response model, exported from the package

### Changed
- `homepage` in package metadata now points to `https://basekick.net/memtrace` (was `https://memtrace.ai`)
- Added `Issues` and `Changelog` project URLs
- Added `keywords` to package metadata

## [0.1.0] - 2026-02-08

Initial release.

### Added
- Sync `Memtrace` and async `AsyncMemtrace` clients (full feature parity)
- Convenience methods: `remember`, `recall`, `decide`
- Memory operations: `add_memory`, `add_memories`, `list_memories`, `search_memories`
- Agent operations: `register_agent`, `get_agent`, `get_agent_stats`
- Session operations: `create_session`, `get_session`, `list_sessions`, `get_session_context`, `close_session`
- Typed error hierarchy: `MemtraceError`, `AuthenticationError`, `NotFoundError`, `ConflictError`
- Pydantic v2 models for all request/response types
- Context manager support (sync and async)
- Python 3.10+ required; `httpx` and `pydantic` runtime dependencies
