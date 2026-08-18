# Deep-Seeing P5–P8 Roadmap

> 目标：在 P1–P4 长期记忆之上，补齐自我观察、持续思考、自主运行、认识世界、运行透明度。  
> 原则：**提供能力，不替 ta 编排人生。**  
> 契约细则：[p5.0-contracts.md](./p5.0-contracts.md)

## 顺序

```text
P5.0 基础契约
  ↓
P5  自我记忆工作台（inspect / trace / propose）
  ↓
P6  Workspace
  ↓
P7  Agency Runtime（Intent / Scheduler / Daemon）
  ↓
P8  World Gateway
```

Runtime Transparency 从 P5.0 起贯穿。

## P5.0（本阶段）

- SelfArtifact 文件 + Graph 指针
- Episode `experience_mode`
- Proposal `kind` + 分策略 ReviewPolicy
- ExecutionQueue
- Intent catch-up / Maintenance dirty / P8 SSRF 契约桩

验收见 [p5.0-contracts.md](./p5.0-contracts.md) 与实现测试。

## P5（已落地）

- `inspect_self` / `trace_self_belief` / `list_self_tensions` / `propose_self_update`
- Dream 按 kind 采纳 self → SelfArtifact + Mutation Ledger（`self_artifact` / `self_proposal`）
- Catalog / Permission / `inspect_runtime` stores 含 `self_store`

## P6（已落地）

- WorkspaceStore：`data/memory/workspace/{questions,writings,research,projects}/`
- revision history + `link_workspace_episode`
- 工具：`list_workspace` / `read_workspace` / `write_workspace` / `link_workspace_episode`
- 详见 [workspace.md](./workspace.md)

## P7（已落地）

- `data/runtime/runtime.db`：Intent + wake_jobs
- Scheduler + Autonomous Turn（默认 noop）+ 日预算；主动联系默认关闭
- 工具：`list_intents` / `read_intent` / `create_intent` / `cancel_intent`
- `cmd/deep-seeingd`：谈话室 + Scheduler
- 详见 [agency.md](./agency.md)

## P8

- **已落地**：`search_web` / `read_webpage` / `list_sources` / `read_source`；`SourceStore` + `SafeClient`（遵守 P5.0 §8）
- 不自动结晶 Principle；正文 `UNTRUSTED_EXTERNAL_CONTENT` fence
- 详见 [world.md](./world.md)

## 明确不做

- 24h LLM 常驻生成、定时强制反思、直接改 Soul、无限自主消息、外部不可逆操作默认放行
