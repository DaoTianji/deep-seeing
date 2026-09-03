# Private server deployment

Deep-Seeing is deployed as one Linux binary behind private Tailscale Serve.
Redis and Neo4j remain independently managed services.

## Fixed layout

```text
/opt/deep-seeing/releases/<commit>/deep-seeing
/opt/deep-seeing/current -> releases/<commit>
/opt/deep-seeing/seed/
/etc/deep-seeing/deep-seeing.env
/var/lib/deep-seeing/data/
/var/backups/deep-seeing/
```

The application service runs as the unprivileged `deep-seeing` user and binds
only to `127.0.0.1:3319`. Tailscale Serve terminates private HTTPS and proxies
to that loopback listener. Do not use Tailscale Funnel for this deployment.

## Required environment

The root-owned environment file must define at least:

```text
OPENAI_API_KEY=
OPENAI_BASE_URL=
OPENAI_MODEL=
NEO4J_URI=bolt://127.0.0.1:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=
NEO4J_DATABASE=neo4j
REDIS_ADDR=127.0.0.1:26739
REDIS_PASSWORD=
REDIS_DB=0
ROOM_ADDR=127.0.0.1:3319
LTM_GRAPH=1
RECALL_MODE=agent
REFLECTION_MODE=agent
ROLE_MODE=off
ROLE_INIT_MODE=off
ROLE_SEARCH_PROVIDER=bing
# Set to the Tailscale login emails allowed to open the private room.
TAILSCALE_ALLOWED_USERS=
```

`brave` remains the preferred authenticated provider when
`BRAVE_SEARCH_API_KEY` is configured. `bing` is the practical no-key fallback;
`duckduckgo` uses the limited Instant Answer endpoint and is unsuitable for
broad source discovery.

生产启用允许列表后，应用只信任 Tailscale Serve 在 loopback 代理请求上注入的 `Tailscale-User-Login`。不要把 Room 直接监听到公网地址，也不要把同名请求头当作普通反向代理认证。

Persistent store paths are absolute and live below `/var/lib/deep-seeing`.
The systemd unit supplies `GOMEMLIMIT=512MiB`.

## Operations

```bash
systemctl status deep-seeing
systemctl restart deep-seeing
journalctl -u deep-seeing -f
systemctl start deep-seeing-backup
systemctl list-timers deep-seeing-backup.timer
tailscale serve status
```

## Release and rollback

Each release is immutable. Upload the binary into a new commit-named directory,
verify its SHA-256 checksum, then atomically replace `/opt/deep-seeing/current`
and restart the service.

To roll back code, repoint `current` to the previous release and restart. If a
data rollback is also required, stop the service, verify the selected backup,
extract it into a temporary directory, replace the data directory, restore
ownership, and start the service. Never overwrite live data without retaining
the pre-rollback snapshot.

## Backup boundary

The timer briefly stops Deep-Seeing so the file stores and SQLite database are
captured consistently. It retains 14 daily archives. It does not back up Neo4j,
Redis, PostgreSQL, or their Docker volumes; those require an infrastructure-level
backup policy.

## Security boundary

Deep-Seeing itself must not be exposed through the public Nginx listener or a
public firewall port. Existing database port exposure is audited separately so
that firewall changes do not interrupt unknown callers.
