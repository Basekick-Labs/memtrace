# Memtrace v0.2.0 — Multi-tenant Arc Routing

**First tagged release of the Memtrace server.** Memtrace is now multi-tenant: a single deployment can serve many organizations, each routed to its own Arc instance with per-org API keys encrypted at rest.

## Highlights

### Multi-tenant Arc routing

A new metadata table (`arc_instances`) maps each `org_id` to its Arc connection details (URL, API key, database, measurement). Per-org Arc clients are resolved at request time from an in-memory `arc.Registry`, with no global Arc client. Every authenticated request is routed automatically based on the org that owns the API key.

```
Client App  --[mtk_...]-->│
                Memtrace ─┼──> Arc (org_acme)
                          ├──> Arc (org_default)
                          └──> Arc (org_other)
```

### Encryption at rest

Each org's Arc API key is stored AES-256-GCM-encrypted in `arc_instances.api_key_cipher`. The 32-byte master key comes from `MEMTRACE_MASTER_KEY` (env var, base64-encoded). Tampering with a ciphertext fails decryption loudly at startup. Memtrace refuses to start without a master key.

Generate one with:

```bash
memtrace keygen master
```

### Admin CLI (cobra)

`cmd/memtrace` is now a cobra root command. Default behavior (`memtrace` with no args) is still `serve`, so existing `docker run memtrace` keeps working. New subcommands:

- `memtrace serve` — run the HTTP server
- `memtrace keygen master` — generate a base64 master key
- `memtrace org create | list | delete | add-arc | show-arc | remove-arc`
- `memtrace key create | list | revoke`

### Auto-migration

On first startup, if the legacy flat `[arc]` block is populated and `arc_instances` is empty, Memtrace automatically encrypts the legacy credentials and inserts them as the `org_default` Arc instance. Idempotent on re-run.

### SDK releases

Both SDKs released v0.2.0 alongside this server release with a typed `NoArcInstanceError` (subclass of `MemtraceError`) returned for HTTP 503 responses where the org has no Arc instance configured:

- **Python**: `pip install memtrace-sdk==0.2.0`
- **TypeScript**: `npm install @basekick-labs/memtrace-sdk@0.2.0`
- **Go**: `pkg/sdk` adds `*APIError` with `errors.Is(err, sdk.ErrNoArcInstance)` support

The OpenAI Agents integration pin moved to `memtrace-sdk>=0.2.0`.

### `/health` and `/ready`

Both endpoints now report per-org Arc reachability:

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

`/ready` returns `200` only when every configured Arc instance is reachable, `503` otherwise (with a `reason: "no_instances"` hint if the deployment isn't provisioned yet).

## Breaking changes

The flat `[arc]` block in `memtrace.toml` is gone. Per-org connection details (URL, API key, database, measurement) now live in the metadata DB and are managed via `memtrace org add-arc`. The `[arc]` block is preserved for global timing/batch knobs only:

```toml
[arc]
connect_timeout = 5
query_timeout = 30
write_batch_size = 100
write_flush_interval_ms = 1000
```

Existing deployments are auto-migrated on first boot — see "Auto-migration" above.

## Install

### Docker

```bash
MASTER=$(openssl rand -base64 32)
docker run -d -p 9100:9100 \
  -e MEMTRACE_MASTER_KEY=$MASTER \
  -v memtrace-data:/app/data \
  ghcr.io/basekick-labs/memtrace:0.2.0
```

### Debian / Ubuntu (amd64, arm64)

```bash
wget https://github.com/Basekick-Labs/memtrace/releases/download/v0.2.0/memtrace_0.2.0_amd64.deb
sudo dpkg -i memtrace_0.2.0_amd64.deb

# 1. Set MEMTRACE_MASTER_KEY in /etc/memtrace/environment
sudo memtrace keygen master | sudo tee -a /etc/memtrace/environment
# (then edit the file so the key is on a MEMTRACE_MASTER_KEY=... line)

# 2. Start
sudo systemctl enable --now memtrace

# 3. Provision an org and Arc instance
sudo -u memtrace -E memtrace org create acme
sudo -u memtrace -E memtrace org add-arc <org_id> \
  --url https://arc.example.com \
  --api-key <arc-key> \
  --database memory
sudo -u memtrace -E memtrace key create --org <org_id> --name acme-prod
```

### RHEL / Fedora / Rocky (x86_64, aarch64)

```bash
wget https://github.com/Basekick-Labs/memtrace/releases/download/v0.2.0/memtrace-0.2.0-1.x86_64.rpm
sudo rpm -i memtrace-0.2.0-1.x86_64.rpm
# (then follow the same provisioning steps as the Debian path)
```

### macOS (Apple Silicon, Intel)

```bash
# Apple Silicon
curl -L -o memtrace.tar.gz \
  https://github.com/Basekick-Labs/memtrace/releases/download/v0.2.0/memtrace-darwin-arm64.tar.gz
tar -xzf memtrace.tar.gz

# Intel
curl -L -o memtrace.tar.gz \
  https://github.com/Basekick-Labs/memtrace/releases/download/v0.2.0/memtrace-darwin-amd64.tar.gz
tar -xzf memtrace.tar.gz
```

The macOS binaries are not signed or notarized in v0.2.0. On first run, macOS will prompt; right-click → Open to allow.

## Download artifacts

| Platform | Architecture | Package |
|---|---|---|
| Docker | amd64 + arm64 | `ghcr.io/basekick-labs/memtrace:0.2.0` |
| Debian / Ubuntu | amd64 | `memtrace_0.2.0_amd64.deb` |
| Debian / Ubuntu | arm64 | `memtrace_0.2.0_arm64.deb` |
| RHEL / Fedora | x86_64 | `memtrace-0.2.0-1.x86_64.rpm` |
| RHEL / Fedora | aarch64 | `memtrace-0.2.0-1.aarch64.rpm` |
| Linux | amd64 | `memtrace-linux-amd64.tar.gz` |
| Linux | arm64 | `memtrace-linux-arm64.tar.gz` |
| macOS | amd64 | `memtrace-darwin-amd64.tar.gz` |
| macOS | arm64 | `memtrace-darwin-arm64.tar.gz` |

Each artifact ships with a `.sha256` checksum file. Verify with `sha256sum -c memtrace-linux-amd64.tar.gz.sha256` (or `shasum -a 256 -c` on macOS).

## Documentation

- [README](https://github.com/Basekick-Labs/memtrace/blob/main/docs/README.md)
- [Architecture](https://github.com/Basekick-Labs/memtrace/blob/main/docs/architecture.md) — multi-tenancy, encryption, registry
- [Configuration](https://github.com/Basekick-Labs/memtrace/blob/main/docs/configuration.md) — admin CLI, master key, auto-migration
- [API](https://github.com/Basekick-Labs/memtrace/blob/main/docs/api.md) — `/health`, `/ready`, REST endpoints
- [MCP Server](https://github.com/Basekick-Labs/memtrace/blob/main/docs/mcp.md)
