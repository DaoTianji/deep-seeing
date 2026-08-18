# Agency Runtime（P7）

> 状态：已落地骨架。  
> 关联：[roadmap-p5-p8.md](./roadmap-p5-p8.md) · [p5.0-contracts.md](./p5.0-contracts.md)

## 目标

过去的自己可以唤醒未来的自己——有边界、可预算、默认可 noop。

## 存储

```text
data/runtime/runtime.db   # 或 LTM_RUNTIME_DIR
  intents
  wake_jobs
```

- Intent：`one_shot | recurring`；默认 `allow_outbound=false`
- Wake：`session_id = auto:<intent_id>:<attempt>`；不进聊天时间线（本阶段 handler 默认 noop）

## 组件

| 包 | 职责 |
|----|------|
| `internal/intent` | SQLite store + catch-up policy |
| `internal/agency` | Budget / Runner / Scheduler |
| `cmd/deep-seeingd` | Room + Scheduler |

## 工具

- `list_intents` / `read_intent` / `create_intent` / `cancel_intent`

## 默认边界

- 每日自主 wake 预算（默认 8）
- 主动联系人关闭（Budget.AllowOutbound=false）
- 停机后 catch-up：周期 Intent 合并 ticks；一次性过期可 cancel
- Autonomous turn 默认 **noop**（记录 wake receipt）；不强制 LLM 长跑

## 运行

```bash
go run ./cmd/deep-seeingd
# 可选 AGENCY_TICK=30s
```

`go run ./cmd/see` 仍可只开谈话室（不启 Scheduler）；Intent 工具仍可用。
