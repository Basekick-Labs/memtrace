# Configuration

Memtrace is configured via a TOML file and/or environment variables.

## Config File

Memtrace looks for `memtrace.toml` in these locations (in order):
1. Current directory (`./memtrace.toml`)
2. `/etc/memtrace/memtrace.toml`
3. `$HOME/.memtrace/memtrace.toml`

## Environment Variables

All config values can be overridden with environment variables using the `MEMTRACE_` prefix:

```bash
MEMTRACE_SERVER_PORT=9100
MEMTRACE_ARC_URL=http://localhost:8000
MEMTRACE_ARC_API_KEY=your_arc_key
MEMTRACE_AUTH_ENABLED=true
MEMTRACE_LOG_LEVEL=debug
```

## Reference

### [server]

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `host` | `MEMTRACE_SERVER_HOST` | `0.0.0.0` | Bind address |
| `port` | `MEMTRACE_SERVER_PORT` | `9100` | Listen port |
| `read_timeout` | `MEMTRACE_SERVER_READ_TIMEOUT` | `30` | Read timeout (seconds) |
| `write_timeout` | `MEMTRACE_SERVER_WRITE_TIMEOUT` | `30` | Write timeout (seconds) |
| `shutdown_timeout` | `MEMTRACE_SERVER_SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout (seconds) |

### [log]

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `level` | `MEMTRACE_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `format` | `MEMTRACE_LOG_FORMAT` | `console` | Log format (`console` or `json`) |

### [arc]

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `url` | `MEMTRACE_ARC_URL` | `http://localhost:8000` | Arc instance URL |
| `api_key` | `MEMTRACE_ARC_API_KEY` | — | Arc API key |
| `database` | `MEMTRACE_ARC_DATABASE` | `memory` | Arc database name |
| `measurement` | `MEMTRACE_ARC_MEASUREMENT` | `events` | Arc measurement name |
| `connect_timeout` | `MEMTRACE_ARC_CONNECT_TIMEOUT` | `5` | Connection timeout (seconds) |
| `query_timeout` | `MEMTRACE_ARC_QUERY_TIMEOUT` | `30` | Query timeout (seconds) |
| `write_batch_size` | `MEMTRACE_ARC_WRITE_BATCH_SIZE` | `100` | Records per write batch |
| `write_flush_interval_ms` | `MEMTRACE_ARC_WRITE_FLUSH_INTERVAL_MS` | `1000` | Flush interval (milliseconds) |

### [auth]

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `enabled` | `MEMTRACE_AUTH_ENABLED` | `true` | Enable API key authentication |
| `db_path` | `MEMTRACE_AUTH_DB_PATH` | `./data/memtrace.db` | SQLite database path |

### [dedup]

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `enabled` | `MEMTRACE_DEDUP_ENABLED` | `true` | Enable deduplication |
| `window_hours` | `MEMTRACE_DEDUP_WINDOW_HOURS` | `24` | Dedup time window (hours) |

## Example Configuration

```toml
# Memtrace Configuration
# https://memtrace.ai

[server]
host = "0.0.0.0"
port = 9100

[log]
level = "info"
format = "json"    # Use "json" in production

[arc]
url = "http://localhost:8000"
api_key = "your_arc_api_key"
database = "memory"
measurement = "events"
write_batch_size = 100
write_flush_interval_ms = 1000

[auth]
enabled = true
db_path = "./data/memtrace.db"

[dedup]
enabled = true
window_hours = 24
```

## Docker

```bash
docker build -t memtrace .
docker run -p 9100:9100 \
  -e MEMTRACE_ARC_URL=http://host.docker.internal:8000 \
  -v ./data:/app/data \
  memtrace
```
