# Python SDK — Issue #4

**Date:** 2026-02-08
**Branch:** `feat/python-sdk`
**Status:** Complete

## What Was Built

Full Python SDK at `sdks/python/` mirroring the Go SDK (`pkg/sdk/`) 1:1.

### Package Structure
- `src/memtrace/` — 7 modules
  - `client.py` — Sync client (`Memtrace`) with 14 methods
  - `async_client.py` — Async client (`AsyncMemtrace`) with 14 methods
  - `models.py` — 13 Pydantic v2 models
  - `exceptions.py` — 4 exception classes (base + 401/404/409)
  - `_base.py` — Shared headers, error handling
  - `_version.py` — Version 0.1.0
  - `__init__.py` — Public API re-exports
- `tests/` — 61 tests across 4 files using pytest + respx
- `pyproject.toml` — Hatchling build, Python 3.10+
- `README.md` — Quickstart + full API examples

### Design Decisions
- **Pydantic v2** with `Literal` types (not Enum) for cleaner JSON serialization
- **httpx** for both sync (`httpx.Client`) and async (`httpx.AsyncClient`) with `base_url`
- **`model_dump(exclude_none=True)`** for Go `omitempty` parity
- **respx** for httpx-native test mocking (patches transport layer, not monkeypatch)
- **Context manager** support (`with`/`async with`) for proper client lifecycle
- **`ListOptions.tags`** is `str` (comma-separated query param, matches Go SDK)
- **`SearchQuery.tags`** is `list[str]` (JSON body, matches Go SDK)

### API Surface (14 methods per client)
Convenience: `remember()`, `recall()`, `decide()`
Memory: `add_memory()`, `add_memories()`, `list_memories()`, `search_memories()`
Agents: `register_agent()`, `get_agent()`, `get_agent_stats()`
Sessions: `create_session()`, `get_session()`, `get_session_context()`, `close_session()`

### Error Hierarchy
```
MemtraceError (base, any HTTP ≥400)
├── AuthenticationError (401)
├── NotFoundError (404)
└── ConflictError (409)
```

## Verification
- `pip install -e ".[dev]"` — OK
- `pytest -v` — 61/61 passed
- `ruff check` — 0 errors
- `python -c "from memtrace import Memtrace"` — OK

## What's Next
- Issue #5: TypeScript SDK (same pattern, different language)
- Issues #6–#15: Python framework integrations (LangChain, CrewAI, etc.) — all depend on this SDK
