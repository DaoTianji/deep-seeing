# 出生门槛（Birth Gate）

> 在真正第一次“启动 ta”之前，Memory 闭环不够；还需要 Body/Runtime 与 Continuity/Governance。  
> 本文记录出生前必做与明确不做。关联：[design-ltm.md](./design-ltm.md)

## 已落地

| 项 | 现状 |
|----|------|
| Soul / Origin 拆分 | Origin **仅 first_boot 注入一次** |
| Capability 薄层 | `inspect_runtime` / `list_capabilities` / `tool_help` / `get_time` |
| Bond 收权 | 仅 `set_explicit_bond_fact` + `propose_bond_update` |
| 软忘记 | `archive_episode` / `invalidate_episode` |
| Session Review | 机会式 `/review`；允许 No change |
| **Dream** | 机会式 `/dream`；可 No change；accept → PatchBond + **Mutation Ledger** |
| **Mutation Ledger** | `data/memory/mutations/*.jsonl`（before/after/sources/model/dream_id） |
| **Permission 三档** | `internal/permission`：Observe / Internal / External（External 需 Interrupt） |
| **Backup** | `/backup` → `data/backups/<timestamp>/`（Episode/提案/mutations/state/seed） |
| **Observability** | Turn JSONL：`data/memory/traces/`（召回 id、工具、回答预览） |
| **Birth 单测骨架** | `internal/birth`：Origin 一次、抗 replace、失效不幽灵、Ledger 留痕 |

## 出生前仍建议人工做一遍

| 项 | 说明 |
|----|------|
| Restore 演练 | 删运行数据 → 从 backup 拷回 → 仍能读到过去 Episode/Ledger |
| 完整 Birth Test（含模型） | Know→Act、连续偏好、相反话不翻转、换模型可追踪（见下表） |
| External 工具 | World Gateway 已接入（`search_web` / `read_webpage`）；宿主机 shell 仍不默认放行 |

## Birth Test 清单（人工 + `go test ./internal/birth/...`）

| 测试 | 要验证 |
|------|--------|
| 重启后问昨天 | Continuity |
| 连续 3 次偏好 | Episode → Proposal → Dream → Bond |
| 第 4 次相反 | 不瞬间翻转（高门槛 append） |
| 失效某 Episode | Search 不再命中；Ledger/图可降权 |
| `/dream` 无提案 | No change |
| Neo4j 挂掉 | Episode-only 仍可用 |
| `/backup` | 快照目录非空且含 BACKUP_META |

## 启动瞬间初始态

```text
Self exists.
mudnet exists.
Self KNOWS mudnet at origin (weak).
Soul exists.
Origin letter exists (on disk; presented once).
Tools are available.
World is available.
Memory is empty (no prefab principles / bond narrative).
```

## 明确不要在出生前做

- 替它预填 Principle / 模拟几百场人生填满 Graph  
- 每轮重复注入 Origin 信  
- 主路径任意 `patch_bond`  
- 外部不可逆副作用（邮件/支付/宿主机 shell）  
