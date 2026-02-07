# Arc Memory - Claude Code Instructions

Context:

/Users/nacho/dev/basekick-labs/basekick.net/content/update/arc-agentic-memory.mdx
/Users/nacho/dev/basekick-labs/arc/docs/case-study-autonomous-agent-memory.md

Backend

## Project Overview

Arc is a high-performance time-series database written in Go. It uses Fiber (HTTP), zerolog (logging), DuckDB (queries), SQLite (metadata/auth/audit), and Apache Arrow/Parquet (storage format).

## Architecture

- **Storage path format:** `{database}/{measurement}/{year}/{month}/{day}/{hour}/{filename}.parquet`
- **Shared SQLite DB** at `cfg.Auth.DBPath` — used by auth, audit, tiering metadata, and MQTT
- **2-tier storage system:** hot (local) and cold (S3/Azure)
- **Clustering:** Raft consensus, writer/reader roles, WAL replication
- **Enterprise product definition:** `docs/progress/2026-01-08-arc-enterprise-product-definition.md`

### Enterprise Features (require license gating)

Features that require a license key: tiering, audit logging, writer failover, clustering, RBAC, auto-aggregation.

**License gating pattern:**
1. Add feature constant in `internal/license/license.go` (e.g., `FeatureWriterFailover = "writer_failover"`)
2. Add `CanUseX()` method on both `License` struct and `Client` struct (`internal/license/client.go`)
3. Check at startup in `cmd/arc/main.go` or coordinator before initializing the feature
4. Add runtime re-validation middleware on API routes (see `requireTieringLicense` pattern)

**Auth pattern for API routes:**
- Use `auth.RequireAdmin(authManager)` middleware on admin-only routes
- Always guard with `if h.authManager != nil` (auth can be disabled)
- License middleware goes after auth middleware

## Build & Test

```bash
go build ./cmd/... ./internal/...
go test ./internal/<pkg>/... -v
```

**Known quirk:** `cmd/arc` is in `.gitignore` — use `git add -f cmd/arc/main.go` when staging changes to main.go.

## Conventions

### Code Style
- Structured logging with zerolog: `.Str("key", val).Msg("description")`
- Component loggers: `logger.With().Str("component", "name").Logger()`
- Context with timeout on all API handlers: `context.WithTimeout(c.Context(), 30*time.Second)`
- Test loggers: `zerolog.Nop()`
- Parameterized SQL queries only (never string interpolation)
- Validate inputs at system boundaries (API handlers, file path parsing)

### Git & PRs
- Always create a branch from main.
- Branch naming: `feat/description`, `fix/description`
- Commit format: `feat(scope): description` or `fix(scope): description`
- PR descriptions: Summary bullets + Test plan checklist
- Main branch: `main`

### API Handler Pattern
```go
type XHandler struct {
    manager       *pkg.Manager
    authManager   *auth.AuthManager
    licenseClient *license.Client
    logger        zerolog.Logger
}

func (h *XHandler) RegisterRoutes(app fiber.Router) {
    group := app.Group("/api/v1/x")
    if h.authManager != nil {
        group.Use(auth.RequireAdmin(h.authManager))
    }
    group.Use(h.requireXLicense)
    // ... routes
}
```

## Planning & Review Process

### Plan Documentation
When exiting plan mode to begin implementation, ALWAYS save the implementation plan first as a markdown file in `docs/progress/` with the date and phase name in the filename. For example: `2024-11-24-phase1-foundation.md`. These files should be tracked and uploaded to Github (use `git add -f` since docs/ may be gitignored).

### Pre-Plan Validation
Before finalizing a plan, use another agent to validate your findings. If there is consensus, ask for authorization from the user to move forward.

### Post-Implementation Review
As last step, use multiple agents in parallel to review:

**For enterprise features:**
- Is this implementation secure?
- Is this implementation behind a license key?
- Does this implementation introduce security issues?
- Does this implementation introduce redundant code, multiple passes, etc.?
- Is the code elegant? Would it be aligned with what a senior engineer would do?
- Update the release notes, and the enterprise-product-definition file. 

**For non-enterprise features:**
- Is this implementation secure?
- Does this implementation introduce security issues?
- Does this implementation introduce redundant code, multiple passes, etc.?
- Is the code elegant? Would it be aligned with what a senior engineer would do?
- Update the release notes. 

## Common Pitfalls

- Path traversal: always validate database/measurement names extracted from file paths (no `..`, no `/\`)
- SQLite file permissions: should be `0600` (contains auth tokens, audit logs)
- `sync.RWMutex`: two concurrent `RLock`s don't deadlock (safe for read-only getters)
- Fiber middleware ordering matters: auth before license before handlers
- When wiring new features in `cmd/arc/main.go`, check that `licenseClient` and `authManager` may be nil
