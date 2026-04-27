# Changelog

All notable changes to `@basekick-labs/memtrace-sdk` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-04-27

### Added
- `NoArcInstanceError` exception, raised on `503` responses when the caller's organization has no Arc instance configured. Memtrace deployments are multi-tenant; an administrator must run `memtrace org add-arc <org_id>` before that organization can read or write memories. Subclass of `MemtraceError`.
- README note about multi-tenant deployments: one Memtrace deployment can serve multiple organizations, each routed to its own Arc instance, selected automatically by the API key.

## [0.1.1] - 2026-04-16

### Fixed
- `homepage` in package metadata now points to `https://basekick.net/memtrace` (was `https://memtrace.ai`)

## [0.1.0] - 2026-04-16

Initial public release.

### Added
- `Memtrace` client with convenience methods: `remember`, `recall`, `decide`
- Memory operations: `addMemory`, `addMemories`, `listMemories`, `searchMemories`
- Agent operations: `registerAgent`, `getAgent`, `getAgentStats`
- Session operations: `createSession`, `getSession`, `listSessions`, `getSessionContext`, `closeSession`
- Typed error hierarchy: `MemtraceError`, `AuthenticationError`, `NotFoundError`, `ConflictError`
- Configurable request timeout via `AbortController` (default 30s)
- Dual ESM + CommonJS builds with full TypeScript declarations
- Zero runtime dependencies; uses native `fetch` (Node.js >= 18)
