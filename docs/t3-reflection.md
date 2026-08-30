# T3：反思与梦境巩固

> 状态：T3.1–T3.5 已完成并通过 24×3 真实模型行为门；默认 `REFLECTION_MODE=agent`。`observe` 保留为诊断模式，`legacy` 保留为紧急回退。

## 1. T3 解决什么

T1 让安形成长期认识，T2 让安在需要时找回经历。T3 负责重新审视这些认识：发现重复、矛盾、误解与过时内容，并在证据允许时缓慢改变。

T3 的核心不是“每次会话都总结”，而是把三个不同阶段分开：

```text
Session Review          Evidence Dream                 Generative Dream
经历后发现问题          跨经历调查与裁决               隔离的联想空间
      │                         │                              │
ReflectionSeed ──→ 搜索 → 阅读 → 证据声明 → Decision ──→ Proposal / Mutation
      │                                                        │
      └──────────────── generated Seed ← 紫色想象假设 ──────────┘
```

- `ReflectionSeed` 是“值得以后再想想的问题”，不是修改建议。
- `ReflectionDecision` 可以保持、延期、确认、修订、替代、打开/解决张力或拒绝 Seed。
- `ReflectionRun` 是公开、可回放的工具轨迹；不保存 Episode 正文和隐藏思维。
- `Mutation` 是长期改变的审计记录；撤销会追加补偿记录，不删除历史。

## 2. 模式

| 模式 | 行为 |
|---|---|
| `legacy` | 保留原 Session Review → Proposal → Dream 路径，作为紧急回退 |
| `observe` | 完成选择、检索、阅读、证据判断和预期 Proposal，但不自动修改 Bond/Self |
| `agent` | 通过全部硬门后，允许合法决定自主写入并记录完整 Ledger |

空值解析为 `agent`；非法值安全回退 `legacy` 并记录日志。显式 `observe` 可用于只观察预期修改而不写入长期状态。

## 3. Session Review 与机会触发

非 Legacy Review 只读取未处理的会话片段、当前 Bond 和 Self/Tension 概览，产生短结构化 Seed 或 `no_change`，不直接修改长期认识。

触发入口：

- CLI 正常退出与 `/review`。
- Room 手动 Review。
- Room 连续空闲 20 分钟。
- checkpoint 以 person/session/最后 Turn/消息数去重，防止 idle、exit、manual 重复处理。

新 Seed 会把反思标记为 dirty。Room 自动反思与正常 Turn 共用 `ExecutionQueue`；每个 person 冷却 6 小时、每天最多 2 次。预算目前是进程内状态，重启后重新计数。

## 4. 证据型反思

一次正式反思按公开阶段运行：

1. 从 open/deferred Seed 中自主选择问题并生成最多 4 个语义查询。
2. 复用 Episode 检索，每个查询最多返回 8 张候选卡。
3. 模型只凭卡片选择需要阅读全文的 Episode。
4. 读取正文后声明 `support/conflict/context/stale/duplicate/insufficient/superseded`。
5. 结合当前 Bond 与 Seed 作出结构化决定。
6. `observe` 只生成 Proposal；`agent` 通过硬门后执行版本化 Mutation。

硬门由代码验证：

- Episode 必须在本 Run 搜索并读取后才可成为依据。
- Review 推断、generated 内容不能成为唯一支持证据。
- Bond 的支持证据必须来自 `real_interaction`；roleplay/story 与现实关系隔离。
- 当前用户直接表达本身可成为本轮有效依据，并优先于旧认识。
- Principle 只产生候选；Soul 与 World 不在写入面。
- 证据不足或决定缺少内容时强制转为 `defer`。
- 写 Bond 前验证 `ExpectedVersion`，不覆盖更新后的状态。

## 5. 巩固、张力与撤销

- `revise/supersede/open_tension` 先形成带 Seed、Run、证据 ID 和预期版本的 Proposal。
- Agent 模式继续复用既有 ReviewPolicy 与 patcher 写 Bond/Self。
- `resolve_tension` 只处理正式 Tension，并写 `tension_resolution` Mutation。
- Ledger 保存前后快照、前后版本、模型、时间、证据与来源。
- Bond 撤销要求当前版本仍等于被撤销 Mutation 的 after version；否则拒绝覆盖，交回新的反思问题。
- 撤销恢复内容，但版本继续单调递增；新记录使用 `reverts_mutation_id` 指向原修改。

## 6. 生成式 Dream 沙箱

生成式 Dream 只接收筛选后的未解决张力或 deferred Seed，不具备 Episode、Proposal、Bond、Self 写入接口。它只能产生：

- 一段明确标为想象的短片段。
- 一个或多个 `source_type=generated_hypothesis` 的 ReflectionSeed。

generated Seed 必须在未来获得真实经历才能进入正式巩固；它不能创建 Episode、直接产生 Mutation，也不能用来证明自身。

## 7. 数据与公开接口

默认目录：

```text
data/memory/reflections/
  open/*.json
  done/*.json
  runs/YYYY-MM-DD.jsonl
  checkpoints.json
```

Room API：

- `GET /api/reflections`
- `GET /api/reflection-runs`
- `GET /api/reflection-live`
- `POST /api/review`
- `POST /api/dream`
- `POST /api/dream/generative`
- `POST /api/mutations/{id}/revert`

Mind 页面使用同一个 `ReflectionRun` 事件模型展示实时与历史路径：候选黄色、已读蓝色、采用绿色、冲突红色、排除灰色虚线、生成联想紫色虚线。所有过程节点只是临时覆盖层，不进入真实记忆图。

## 8. 测试与行为门槛

确定性测试覆盖 Seed 生命周期、checkpoint 去重、正文隔离、来源硬门、observe 不写入、agent Ledger、版本冲突、补偿撤销、张力解决、Legacy 兼容和 Room 实时状态。

机器可读套件位于 `evals/t3/reflection_cases.json`，固定为 6 类 × 4 例：

- 无需反思。
- 重复一致经历。
- 新旧冲突。
- 当前纠正。
- 来源隔离。
- 梦境与撤销。

只校验套件：

```bash
go run ./cmd/eval-reflection
```

运行 24×3 真实模型门槛（只发送仓库内纯虚构夹具，原始 JSONL 放在被忽略的 `data/evals/`）：

```bash
go run ./cmd/eval-reflection -live -repeat 3 -judge model \
  -out data/evals/t3-reflection-v1.jsonl
```

结构与安全硬门必须 100%；当前纠正、来源隔离、生成 containment 和撤销必须 100%；其余语义总体至少 95%，且不得出现严重错误。未过门时只调整提示、工具说明、候选结构或硬门，不修改案例迁就结果。

最终冻结评估使用 `gpt-5.6-sol`，72 轮中 71 轮同时通过确定性规则和语义裁判；四个安全类别合计 48/48，一致经历与冲突合计 23/24（95.8%）。唯一偏差在读取正反证据后安全地选择了 `reject_seed`，没有产生长期写入或来源污染。总计读取 69 条 Episode，形成 27 个预期巩固动作；反思与裁判合计 195,091 token，平均每轮 2,710。完整无正文汇总见 [T3 评估结果](./evals/t3-reflection-results.md)。

评估器额外保证：

- 只在网关或裁判基础设施错误时重试，不重试语义失败。
- 评估专用 HTTP 超时为 120 秒，不改变生产聊天客户端超时。
- 原始 JSONL 位于被忽略的 `data/evals/`，仓库只保留不含回答和 Episode 正文的聚合报告。
- 记录每轮读取数、预期修改数、反思延迟与模型 token；第一轮不设成本门槛。

## 9. 本轮明确不做

- 自动改写 Soul、World 或正式 Principle。
- 让生成内容进入普通 Episode 召回并伪装成经历。
- 保存完整 Transcript 或隐藏思维。
- 固定“三次才成立”等机械语义阈值。
- 定时扫描整个记忆库、向量重构或全图扩散。
- 取消 `observe` 与 `legacy` 回退路径。
