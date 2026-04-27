# Configuration

Memtrace splits configuration in two:

- **Global server settings** (HTTP port, log format, timeouts, dedup) live in `memtrace.toml` or `MEMTRACE_*` environment variables.
- **Per-org Arc connection details** (URL, API key, database, measurement) live in the metadata SQLite database and are managed via the `memtrace org` CLI. Memtrace is multi-tenant — each org points at its own Arc instance.

A required `MEMTRACE_MASTER_KEY` environment variable is used to encrypt per-org Arc API keys at rest.

## Master key

Memtrace uses AES-256-GCM envelope encryption for the per-org Arc API keys it stores. The master key must be set on every host that runs `memtrace serve` or any `memtrace` admin subcommand.

```bash
# Generate once
export MEMTRACE_MASTER_KEY=$(memtrace keygen master)
```

The value is a base64-encoded 32-byte key. **Losing it makes encrypted secrets unrecoverable** — store it in your secret manager (AWS Secrets Manager, GCP Secret Manager, Vault, 1Password, etc.) and inject it at runtime. Don't commit it to source control.

The server fails to start if `MEMTRACE_MASTER_KEY` is missing, malformed, or the wrong length.

## Config file

Memtrace looks for `memtrace.toml` in these locations (in order):

1. Current directory (`./memtrace.toml`)
2. `/etc/memtrace/memtrace.toml`
3. `$HOME/.memtrace/memtrace.toml`

## Environment variables

All TOML keys can be overridden with environment variables using the `MEMTRACE_` prefix and `_` between sections:

```bash
MEMTRACE_SERVER_PORT=9100
MEMTRACE_AUTH_ENABLED=true
MEMTRACE_LOG_LEVEL=debug
MEMTRACE_MASTER_KEY=...    # required; not from TOML
```

## Reference

### `[server]`

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `host` | `MEMTRACE_SERVER_HOST` | `0.0.0.0` | Bind address |
| `port` | `MEMTRACE_SERVER_PORT` | `9100` | Listen port |
| `read_timeout` | `MEMTRACE_SERVER_READ_TIMEOUT` | `30` | Read timeout (seconds) |
| `write_timeout` | `MEMTRACE_SERVER_WRITE_TIMEOUT` | `30` | Write timeout (seconds) |
| `shutdown_timeout` | `MEMTRACE_SERVER_SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout (seconds) |

### `[log]`

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `level` | `MEMTRACE_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `format` | `MEMTRACE_LOG_FORMAT` | `console` | Log format (`console` or `json`) |

### `[arc]`

These are the global Arc client knobs shared by every per-org instance. Per-org `url`, `api_key`, `database`, and `measurement` are stored in the metadata DB — see "Managing organizations and Arc instances" below.

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `connect_timeout` | `MEMTRACE_ARC_CONNECT_TIMEOUT` | `5` | Connection timeout (seconds) |
| `query_timeout` | `MEMTRACE_ARC_QUERY_TIMEOUT` | `30` | Query timeout (seconds) |
| `write_batch_size` | `MEMTRACE_ARC_WRITE_BATCH_SIZE` | `100` | Records per write batch |
| `write_flush_interval_ms` | `MEMTRACE_ARC_WRITE_FLUSH_INTERVAL_MS` | `1000` | Flush interval (milliseconds) |

### `[auth]`

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `enabled` | `MEMTRACE_AUTH_ENABLED` | `true` | Enable API key authentication |
| `db_path` | `MEMTRACE_AUTH_DB_PATH` | `./data/memtrace.db` | SQLite database path (also stores org/Arc-instance metadata) |

### `[dedup]`

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `enabled` | `MEMTRACE_DEDUP_ENABLED` | `true` | Enable deduplication |
| `window_hours` | `MEMTRACE_DEDUP_WINDOW_HOURS` | `24` | Dedup time window (hours) |

### Master key (env-only)

| Env | Required | Description |
|-----|----------|-------------|
| `MEMTRACE_MASTER_KEY` | yes | Base64-encoded 32-byte key for envelope encryption of per-org Arc API keys. Generate with `memtrace keygen master`. |

## Example configuration

```toml
[server]
host = "0.0.0.0"
port = 9100

[log]
level = "info"
format = "json"    # use "json" in production

[arc]
connect_timeout = 5
query_timeout = 30
write_batch_size = 100
write_flush_interval_ms = 1000

[auth]
enabled = true
db_path = "./data/memtrace.db"

[dedup]
enabled = true
window_hours = 24
```

## Managing organizations and Arc instances

Per-org Arc connection details are managed with the bundled CLI. Every admin command needs the metadata DB path (read from `memtrace.toml`) and `MEMTRACE_MASTER_KEY` set.

### Create an organization

```bash
memtrace org create acme
# Organization created
#   id:   org_a1b2c3d4...
#   name: acme
```

### Bind an Arc instance to the org

```bash
memtrace org add-arc org_a1b2c3d4... \
    --url https://arc.acme.example.com \
    --api-key <arc-api-key> \
    --database acme_memory \
    --measurement events       # default: events
```

The Arc API key is encrypted with `MEMTRACE_MASTER_KEY` before being stored.

### Inspect / replace / remove

```bash
memtrace org list
memtrace org show-arc <org_id>      # API key is masked
memtrace org remove-arc <org_id>
memtrace org delete <org_id>        # also cascades to its arc instance, agents, sessions, keys
```

### Issue API keys for the org

```bash
memtrace key create --org <org_id> --name acme-prod
# API key created (shown only once — save it now):
# mtk_...

memtrace key list --org <org_id>
memtrace key revoke <key_id>
```

A request authenticated with that key automatically routes reads and writes to the org's Arc instance. If the org has no Arc instance configured, the API returns `503` with a hint to run `memtrace org add-arc`.

## Auto-migration from the legacy `[arc]` block

Older Memtrace deployments declared the Arc URL/API key/database/measurement directly in `memtrace.toml`. On first startup, if the legacy `[arc]` block has a `url` set and the new `arc_instances` table is empty, Memtrace automatically:

1. Ensures `org_default` exists in the `organizations` table.
2. Encrypts the legacy `api_key`.
3. Inserts an `arc_instances` row (`id=arc_default`, `org_id=org_default`).
4. Logs:

```
WRN legacy [arc] config detected — migrating to DB url=...
INF migration complete; remove [arc] url/api_key/database/measurement from memtrace.toml on next deploy
```

The migration is idempotent — once `arc_instances` is populated, the legacy fields are ignored and the migration code does nothing on subsequent boots. Remove the deprecated fields from your TOML when convenient.

## Docker

```bash
docker build -t memtrace .

# Generate a master key once and store it in your secret manager
MASTER=$(docker run --rm memtrace memtrace keygen master)

# Run the server
docker run -p 9100:9100 \
  -e MEMTRACE_MASTER_KEY="$MASTER" \
  -v ./data:/app/data \
  memtrace
```

After the server is up, provision orgs and Arc instances by running the admin CLI inside the container or against the same data volume:

```bash
docker exec -e MEMTRACE_MASTER_KEY="$MASTER" -it memtrace \
  memtrace org create acme
```
