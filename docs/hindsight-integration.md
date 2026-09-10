# Hindsight retrieval and SiliconFlow integration

## Scope

Ordinary Agent Episode recall now supports `MEMORY_RETRIEVAL_BACKEND=legacy|bm25|hindsight`.
Default remains `legacy`. Invalid configuration falls back with a log. `RECALL_MODE=legacy`
continues using the original SideQuery and search behavior. Actor, Corpus, role-worldline
memories, reflection, Soul and Bond are **not migrated** by this change. The Director's
ordinary Episode tools share the new backend; its dedicated role-memory tools stay separate.

`bm25` uses factual Episode files, including files older than the bounded recent index.
It does not call any model. Latin words and overlapping Han bigrams are indexed;
this is a baseline tokenizer, not a claim of optimal Chinese segmentation.

`hindsight` calls an explicit REST client. Use a tenant-derived bank, not an
Agent-supplied bank name. A result is usable only when document ID, metadata ID,
current source revision, active status and ordinary scope all match locally.
Remote fact text is not passed off as the source. Candidate previews are bounded
original excerpts; read/adopt remain separate actions. A successful empty result
stays empty; an unavailable service falls back to BM25 and records this in Trace.
There is no recent-Episode fallback.

Hindsight recall HTTP requests have a 60-second timeout (increased from 15 seconds
after the six-case pilot). Retain keeps its separate 180-second timeout. A shorter
caller deadline or cancellation still takes precedence. This changes the wait
boundary, not the number of retries; the historical pilot results remain unchanged.

## State and cost boundary

`MEMORY_INDEX_MODE=off` is the code default. `auto` enables daemon reconciliation:
two documents per minute-long batch, at most 50 attempts and 256 KiB of source text
per UTC day, maximum 64 KiB per document. Files remain the durable work source.
The attempt journal (`LTM_RUNTIME_DIR/episode-index-sync.json`) is atomically written
before remote I/O. Failed or uncertain writes are not automatically retried, including
after restart. Inspect the upstream document before manually authorizing another try.
Successful retain token usage is saved; failed-call billing and embedding/reranking
costs may not be returned and must not be assumed to be zero.

No automatic Hindsight consolidation or reflect is enabled. T3 keeps its authority.
Before enabling Hindsight in production, populate and verify the scoped bank.
An empty Hindsight index is not an upgraded production memory system.

Sources remain in the existing files. After editing a source, old indexed revisions
are rejected until explicitly reindexed. After archive/deletion they cannot be used.
The reconciliation worker also deletes previously indexed documents whose local
source is deleted, archived or no longer allowed. A failed deletion is marked
`delete_uncertain`; local access stays denied while an operator investigates.
No source document is deleted by reconciliation. AppleDouble `._` files are ignored;
missing legacy timestamps use stable file metadata instead of changing on every read.

## Private PostgreSQL

Run `deploy/scripts/provision-memory-postgres.py` as root on the target server.
It reuses the installed `pgvector/pgvector:0.8.5-pg16` image by immutable image ID,
creates only `deep-seeing-memory-pg` / `deep-seeing-memory-pgdata`, and binds
`127.0.0.1:5457`. Existing PostgreSQL, Redis and Neo4j are not modified.
The container has a 768 MiB limit, 1 CPU limit and bounded logs.
Root-only credentials are in `/etc/deep-seeing/memory/`; never copy them to Git.
The application role `hindsight` has no superuser/create-role/create-database rights.

Status: `docker ps --filter name=deep-seeing-memory-pg`

Dedicated backup, without stopping other services:

```sh
systemctl start deep-seeing-memory-backup
systemctl status deep-seeing-memory-backup.timer
```

Archives are root-only under `/var/backups/deep-seeing/hindsight/`, retained for
14 days and validated with `pg_restore --list`. Existing backup jobs are unchanged.
Never delete the dedicated volume to roll back application code.

## Models and isolated service

Requested generator: `deepseek-ai/DeepSeek-V4-Pro`.
Embedding: `Qwen/Qwen3-Embedding-8B` at **1536 dimensions**;
reranker: `Qwen/Qwen3-Reranker-8B`.
Endpoint: `https://api.siliconflow.cn/v1`.
Confirm model availability with the account's model list before changing production.
Do not change only the base URL while retaining the ops-ai API key.
Use an explicitly chosen embedding dimension compatible with the deployed vector
index, fixed across ingest and query; changing it requires a new index generation.

Store the new credential as `SILICONFLOW_API_KEY` in ignored `.env.local` for secure
transfer. Do not print it. The user authorized small live probes and requires advance
notice before large-scale evaluations. No bulk LongMemEval run is part of this slice.
DeepSeek reasoning is enabled (`high`) for the existing Agent/ChatClient paths and
tested through an actual Eino streamed tool round trip. Public traces do not expose
reasoning content. Hindsight fact extraction uses non-thinking Pro with an 8192-token
output cap, concurrency 1 and one configured attempt, rather than the upstream 64000
output-token default. Embedding and reranking use dedicated remote models, not local ML.

The registry image repeatedly timed out on duplicated 320 MB dependency layers.
The deployment therefore uses the official `hindsight-api-slim==0.9.2` Python package
in `/opt/deep-seeing/hindsight-0.9.2-locked`, independent of the system environment.
`deploy/hindsight-0.9.2-constraints.txt` derives version constraints from the release's
official `uv.lock`; it avoids silently upgrading major SDK versions during installation.
The service uses the separate `deep-seeing-memory` account, 1536 MiB memory cap,
150% CPU cap, loopback `127.0.0.1:8889`, root-only configuration, and journald.
It has no access to the app's private runtime directory and no Tailscale/public route.

```sh
systemctl status deep-seeing-hindsight
journalctl -u deep-seeing-hindsight --since today
# Preview only; does not invoke models:
/opt/deep-seeing/ops/deep-seeing-memory-index -episodes /var/lib/deep-seeing/data/memory/episodes
```

Manual backfill requires `-apply -limit N -journal PATH` and `HINDSIGHT_URL`.
Do not run it concurrently with the daemon's automatic indexing owner.
`cmd/smoke-memory -run` creates only temporary synthetic sources and a separate bank,
checks semantic retrieval plus Agent search/read/use, then deletes its remote documents.
`scripts/smoke-siliconflow.py` runs four bounded provider probes with fictional input.

Application releases record the binary SHA-256, source base, previous link and protected
configuration backup. `activate-memory-release.py` reverts binary and configuration
if the expected model does not appear in `/api/runtime`. It does not overwrite memory.
For a later manual rollback, restore both the recorded old symlink and configuration,
restart Deep-Seeing, and leave the new database volume intact. A backend-only rollback
can set `MEMORY_RETRIEVAL_BACKEND=bm25` and `MEMORY_INDEX_MODE=off`.

## Upstream review

Pinned upstream release: Hindsight v0.9.2.
MIT main license. This is not a blanket legal/security warranty for all transitive
dependencies or hosted provider terms. Preserve upstream license notices when distributing.
No optional `pg_search`/AGPL extension is installed by the PostgreSQL script.

References:
- https://hindsight.vectorize.io/developer/api/recall
- https://hindsight.vectorize.io/developer/api/retain
- https://api-docs.siliconflow.cn/docs/api/embeddings-post
- https://api-docs.siliconflow.cn/docs/api/rerank-post

## Deployment acceptance (2026-09-09)

The private server now uses SiliconFlow DeepSeek V4 Pro and Hindsight following
explicit approval for existing and future ordinary Episode processing. Eight eligible
documents (1931 source bytes) were indexed without changing source-file hashes.
Bounded automatic reconciliation is enabled in production only; code defaults stay
legacy/off. Production recall and one Room streaming turn passed. Controlled restarts
preserved STM, Mutation, Reflection, Turn data and the index journal without reindexing.

`deploy/scripts/verify-memory-release.py` defaults to read-only HTTP checks.
`--recall` adds one billable production query; `--chat` adds one marked deployment
message to the real Room; `--restart` restarts the two application services, not any
database container. Do not run these flags in an unattended monitoring loop.
Private HTTPS, HTML, JS/CSS and runtime APIs were verified. Browser visual QA could
not run because of a local automation environment error and is not claimed as passed.

These checks do not establish a benchmark score. Large evaluations require advance
notice; no bulk LongMemEval run was performed during deployment.

## Offline tests

`go test ./internal/memory ./internal/tools ./internal/observe ./internal/app`

Tests cover durable attempts, bounded synchronization, withdrawn-source cleanup,
stable legacy timestamps, macOS sidecars, source/revision validation, cross-person and role exclusion, old files,
Chinese lexical retrieval, empty results, fallback, redirects, sanitized errors,
candidate previews and the read/evidence trace boundary. They do not prove live
provider quality or simulate a completed migration.
