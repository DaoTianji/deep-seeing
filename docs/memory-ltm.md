# 长期记忆（LTM）— 实现现状

> 状态：**Phase 1–4、P5–P8、T1、T2、T3 已落地**；T4 工程与 108/108 行为验收已完成，等待人工浏览器回放后生产启用，默认 off（见 [roadmap-v0.9.md](./roadmap-v0.9.md)）
> 目标架构：[design-ltm.md](./design-ltm.md) · 认知共识：[memory-cognition.md](./memory-cognition.md) · Roadmap：[roadmap-p5-p8.md](./roadmap-p5-p8.md) · v0.9：[roadmap-v0.9.md](./roadmap-v0.9.md) · 契约：[p5.0-contracts.md](./p5.0-contracts.md) · Workspace：[workspace.md](./workspace.md) · Agency：[agency.md](./agency.md) · World：[world.md](./world.md) · 出生门槛：[birth-gate.md](./birth-gate.md)
> 关联：[memory-stm.md](./memory-stm.md) · [`seed/SOUL.md`](../seed/SOUL.md) · [`seed/origin/`](../seed/origin/) · [`internal/graph`](../internal/graph/)

## 1. 当前实现（Phase 1 + 2 + 3）

### 1.1 Soul 与 Origin Context

- **Soul**：每次启动注入；**Origin Introduction**：仅 first_boot（标记于 `data/memory/state/origin_presented/`）；信本身永久在 `seed/origin/`
- 加载：`internal/soul`、`internal/origin`；Assembler：`Soul` + `Body/Capabilities`（薄提示）+（可选）`Origin Introduction` + Memory
- Capability：`inspect_runtime` / `list_capabilities` / `tool_help` / `get_time`（不把全工具表塞进 System Prompt）
- CLI scope：`UserID=mudnet`，`PersonID=user:mudnet`

### 1.2 Episode 存储

```text
data/memory/episodes/          # 或 LTM_EPISODE_DIR
  index.md
  by_id/ep_<uuid>.md
  .migrated_from_topics        # 从 topics/ 迁过则存在
```

- 实现：`EpisodeStore`（`internal/memory/episode_store.go`）
- 空库时自动迁移 `data/memory/topics/*.md` → Episode（带 `legacy_key`）

### 1.3 Neo4j Context Graph（Phase 2）

- 包：`internal/graph`；配置 `NEO4J_*`；可选 `LTM_GRAPH=0` 强制关闭
- 凭证齐全则启动：`EnsureSchema` + `EnsureOriginSeed`（仅 `KNOWS.role_at_origin` + 空 `BOND`）
- 节点：`Self` / `Person` / `Episode`，以及隔离支路 `Role` / `RoleInstance` / `RoleWorldline` / `Source` / `RoleEpisode`（正文仍在文件）
- 边：`KNOWS`、`BOND`、`ABOUT`、`CALLS`
- 失败降级：日志后继续 Episode-only（与 Redis STM 同思路）

### 1.4 写入 / 读取

| 路径 | 现状 |
|------|------|
| 工具 | Episode CRUD；Bond/Self 提案；Self/Workspace/Intent/World；Capability 套件 |
| `set_explicit_bond_fact` | 仅 `call_name` / `basics_fact`；**无**任意 `patch_bond` |
| `archive_episode` / `invalidate_episode` | 软忘记；默认召回跳过 |
| Session Review | exit/`/review`/Room idle；非 Legacy 只产生 ReflectionSeed 或 No change，checkpoint 去重 |
| Dream | 证据型 `/dream` 与隔离生成式 `/dream-gen`；observe 只提案，agent 才可版本化写入 |
| Mutation Ledger | `data/memory/mutations/*.jsonl`；带证据/版本/Seed/Run，撤销追加补偿记录 |
| Backup | `/backup` → `data/backups/` |
| Observability | `data/memory/traces/*.jsonl` |
| 提案队列 | `data/memory/proposals/open|done` |
| Reflection | `data/memory/reflections/`；Seed、Run、checkpoint；公开轨迹不保存正文 |
| 回合后 Extractor | **默认 Noop** |
| 召回 | `legacy`：Bond → SceneNorm（关键词）→ 开放提案 → Episode；`agent`：完整 Bond + Workspace/Intent 薄快照，Agent 自主调查 SceneNorm、Workspace、Intent、Proposal、Episode |

### 1.5 与设计的差距

- 无 External 工具 Interrupt UI、Restore 需人工演练  
- 完整 Know→Act Birth Test 仍需带模型跑一遍（清单见 birth-gate）  
- **T1（常模参与对话，已落地）**：`FormatCompactRecall` 优先级注入；Bond `items_json` + `bond_version` 为 Item SoT；`append_bond_boundary` / `propose_bond_update` / `set_explicit_bond_fact`；SceneNorm 文件旁路（`list/read/write_scene_norm`，关键词命中注入）；Strategy 派生缓存（`set_bond_strategy_cache`，绑定 `bond_version`）。旧 `Strategy` 散文非 SoT。
- **T2.1–T2.4（自主召回 + 证据闭环 + 当前任务处境 + 多来源）**：`RECALL_MODE=agent` 时，会话缓存完整 Global Bond，不执行固定 SideQuery；Episode、SceneNorm、Workspace、Intent、Proposal 都通过薄候选 → read → use/dismiss/focus 的公开路径参与回答，固定角色分别为 evidence、guidance、task、plan、hypothesis，Bond 为 baseline。Workspace/Intent 仍自动提供最多 4 张 active 薄卡和会话焦点。统一 Trace 只存来源、ID、查询、读取和采用状态，不存正文或隐藏思维；Room 可实时显示并回放跨来源临时节点。T2.1–T2.3 的 48/48 与 27/27 行为验收已完成；T2.4 的工程、本地验收和 13 类 × 3 次真实 Agent 复验均已完成，最终 39/39 通过。
- **T2.5.1（会话注意）**：Agent 模式在进程内维护中心/支撑/外围 4/8/16 槽位。新中心和支撑必须本轮 read，新外围必须本轮成为 candidate；满槽必须由 Agent 显式指定替换项。成功读取或公开处理会重置闲置计数；快照与调整进入公开 Trace 和 Room 图谱外环，但不保存正文、不写 LTM、不改变来源角色。工程、离线压力套件和真实 Agent 9 类 × 3 次复验均已通过，最终为 48/48 个有效回合。默认仍为 `legacy`，尚未引入 Recall Broker、重排、向量搜索或图扩散。
- **T3（反思与梦境巩固）**：Session Review 先形成可长期等待验证的 ReflectionSeed；证据型 Reflection 自主选择问题、搜索和阅读真实 Episode、声明支持/冲突/过时/不足，再决定保持、延期、确认、修订、替代或形成/解决 Tension。observe 模式只形成预期 Proposal；agent 模式通过来源、当前表达、禁止目标和版本硬门后才写 Bond/Self，并记录完整 Mutation。生成式 Dream 完全隔离，只能创建 generated Seed。补偿撤销保留旧 Ledger 并要求版本未被后续修改。Mind 使用同一 ReflectionRun 展示实时与历史公开轨迹。详见 [t3-reflection.md](./t3-reflection.md)。
- **T4（幕后导演与角色人生剧场，上线门前）**：独立 Actor/Director Runtime、Stage/Backstage Transcript、角色 Episode 命名空间、世界线分叉、可撤销 DirectorAction、来源化角色编译与私人沙箱已落地；Role Library 与 Theater 前端已接入。36×3 真实行为门最终 108/108 通过，隔离 Agent API 演示完成。默认 `ROLE_MODE=off`，人工浏览器完整回放通过前生产最多启用 `observe`。详见 [t4-role-theater.md](./t4-role-theater.md)。

### 1.6 P5.0 / P5 / P6 / P7 / P8（已落地）

- P5.0 契约：experience_mode、Proposal policy、ExecutionQueue、SSRF 桩  
- Self：SelfArtifact + inspect/trace/propose_self_*  
- Workspace：`data/memory/workspace/` + `list/read/write/link_workspace_*`  
- Agency：`data/runtime/runtime.db` + Intent 工具 + `cmd/deep-seeingd`（默认 noop wake）  
- World：`data/memory/sources/` + `search_web` / `read_webpage` / `list_sources` / `read_source`（SSRF + fence + 日预算）


## 2. 与 STM 的分工

| | STM | LTM |
|--|-----|-----|
| 内容 | 近轮对话 | 沉淀后的可引用状态 |
| 生命周期 | 会话 / TTL | 持久，可修订可追溯 |
| 进 prompt | history（可压缩） | Memory recall 段 + 工具 |
| 丢失策略 | 可丢原文 | 丢之前应压缩/结晶，而非静默蒸发 |

回合位置：

```text
STM.Get → LTM SideQuery → Assembler → Compactor(STM视图) → Eino
  → STM.Append → LTM Extractor（及工具读写 LTM）
```

## 3. 现状：存储与类型

### 3.1 文件布局

```text
data/memory/
  MEMORY.md           # 索引，newest first
  topics/<key>.md     # 正文 + YAML front matter
```

- 实现：`MarkdownStore`（`internal/memory/markdown_store.go`）
- 索引上限：`maxIndexLines = 200`，`maxIndexBytes = 25KiB`（超则裁索引行）
- 迁移：空库时导入旁路 `data/ltm.json` → `ltm.json.migrated`

### 3.2 接口与 Category

- `Provider`：`Write` / `Query` / `ListRecent`（`internal/memory/types.go`）
- Category：`user` | `feedback` | `project` | `reference` | `person`
- 记录字段：`id/key/content/metadata/created_at/updated_at`
- 租户：`identity.TenantScope`（CLI：`local` / `deep-seeing`）；Markdown 侧隔离仍偏弱，图化后需显式节点

### 3.3 写入路径（现状）

**A. 工具 `write_memory`**（`internal/tools`）

- 模型在 ReAct 中主动调用 → `Provider.Write`
- 同 key 更新 topic 并刷新索引行

**B. `LLMExtractor.AfterTurn`**（`internal/memory/extractor.go`）

- 触发条件：user+assistant 合计 rune 数 ≥ `MinTurnLen`（默认 **40**）
- 小模型抽取 JSON 数组，最多约 **5** 条；无则 `[]`
- 提示要求：只留稳定事实；排除临时步骤、密钥与隐私
- 写出：直接 `LTM.Write`（**无显式去重/合并策略**，同 key 覆盖）

### 3.4 读取路径（现状）

**A. 旁路 `LLMSideQuery`**（每轮主模型前）

- `limit` 默认 **5**（runtime 传入）
- 小模型读 `MEMORY.md` 选 key → `ReadKeys` → `prompt.FormatMemoryRecall` 注入 `## Memory recall`
- Chat 失败或空选：回退 `ListRecent`

**B. 工具 `read_memory`**

- 关键词 / 全文 `Query`，模型按需拉取

### 3.5 已知局限（现状）

- 扁平 `key → content`，缺「对谁 / 在何事件」槽位 → 称谓等易假冲突
- 写入无去重、无证据链、无信任权重
- 无 Reflection / Dream / 合理化 / Canon
- Neo4j 已鉴权连通，**代码未消费**

## 4. 产品原则：深度看见（已共识，待代码）

### 4.1 分层真相源（级别不同）

三层级别不同，**不是三份互相覆盖的副本**：

| 层 | 管什么 | 变慢/变快 | 权威场景 |
|----|--------|-----------|----------|
| **图（整体）** | 印象、性格常模、相处方式、原则、信任、认同 | **慢变**：十次沟通形成的整体印象，不因单次会话整体翻转 | 「ta 通常怎样」「我一般如何与 ta 说话」 |
| **文档（事件）** | 值得记忆的具体事、可引用叙事；可按用户时间线成卷 | 中速：会话中/结束后追加 | 「某次发生了什么」 |
| **聊天（原话）** | STM/会话日志原文 | 快、可 TTL | 核对语气、争议、审计；默认不进主召回 |

原则句：

> **整体性的事以图为主；细节以文档和聊天记录为主。**  
> **先有事，后有树；树在复盘里长，不在每一句话里长。**

召回默认顺序：

```text
1. 展开与当前 Person 相关的图枝（常模 + 现行结论 + tag）
2. 信息不够 → 按 tag/id 读 Episode 文档
3. 仍不够或有争议 → 读对应 session 聊天记录
```

工具面相应三分：图读写、文档读写、聊天读取。

文档粒度：**不要「一会话一主文档」**；倾向「一用户一条时间线（过长按时间拆卷）+ 多条 Episode 引用」，图节点用 `session_id` / `episode_id` tag 指回证据。

### 4.2 常模（Norm）与对人的觉察

深度看见的核心产出之一是 **对某人的常模认知**，挂在 `Self–Person`（Bond）上，例如：

- 基础信息与稳定偏好  
- 性格与沟通风格（怎么说得进去、什么碰不得）  
- 平时关心/关注什么  
- 可接受与不可接受的边界  
- 「平时状态」基线（语气、节奏、能量的典型范围）

有了常模，才能回答：

- **平常**该如何与这个人交流  
- 当前表现是否落在常模内  

### 4.3 波动时的双假设（觉察的可贵之处）

当本会话/本事件相对常模出现明显偏离时，不立刻改写整体印象，而是显式进入判别：

| 假设 | 含义 | 典型动作 |
|------|------|----------|
| **H1：我的认知有偏** | Self 对 ta 的常模不准或过时 | 积累证据 → 会话复盘/Dream **缓慢修订** Bond（需多次或强证据） |
| **H2：对方状态异常** | ta 今天/这段时间不在常态 | 记 Episode + 「状态波动」标记；**常模不动**；沟通策略临时调整（更谨慎、更询问） |

一次沟通不足以推翻十次形成的整体印象——这与「图管整体、抗单次冲击」一致。  
觉察的体验来自：被当成一个有连续性的人对待，同时异常时被「看见」而不是被标签一次性改写。

同时维护：

- **Self 认知**：我是谁、我的原则、我与人相处的方法  
- **对 XX 的认知**：常模 Bond + 指向具体事件的证据链  

### 4.4 编码 → 复盘 → 巩固（类人过程）

| 阶段 | 时机 | 产出 |
|------|------|------|
| 编码 | 回合中/后轻量 | 「值得记的事」→ 文档/Episode 节点 |
| 会话复盘 | STM 结束 / idle / 手动 | 本会话子树更新；常模仅提案或微调；打 session/person tag |
| 巩固 Dream | 低频跨会话 | 合并、降噪、常模正式修订、原则结晶 |

## 5. 规划：图与本体论（在原则之下）

### 5.1 核心命题

1. **物质（节点）**：人、事、物、活动……可指称物  
2. **联系（边）**：Self 与之交互才产生意义  
3. **常模子图**：Bond 上慢变的结构化印象（深度看见的主界面之一）  
4. **证据外置**：细节正文在文档，原话在聊天；图持有指针与结论  

### 5.2 建议节点 / 边（演进，随原则调整）

| 版本 | 焦点 | 内容 |
|------|------|------|
| **v1** | 分层可跑通 | `Self` `Person` `Episode`；Bond 常模字段（可先粗）；Episode↔文档；会话复盘写枝；召回树→文档 |
| **v2** | 觉察闭环 | 常模 vs 波动标记；H1/H2 提案；信任 / `SELF_IDENTITY` |
| **v3** | 巩固 | `Principle` `Method`；Dream 慢更新常模 |
| **v4** | 张力 | `Tension` / 合理化（仅不可解冲突） |
| **v5** | Canon | 小说道路等只读层（可选） |

配置（已预留）：`NEO4J_URI` / `NEO4J_USER` / `NEO4J_PASSWORD` / `NEO4J_DATABASE`。

Neo4j 落地前须先把 §4 的权威边界写进 schema（哪些属性属慢变 Bond，哪些只能写在 Episode）。

### 5.3 多挂载

同一经历可挂：自我感受、关系常模、具体事件——primary + `also_link`，避免三份打架散文。

## 6. 规划：写入与读取

### 6.1 写入

| 通道 | 行为 |
|------|------|
| 在线 Extractor | 只做轻量「值得记的事」→ 文档/Episode；**默认不改常模** |
| 工具 | 分清写图（结论/常模提案）vs 写文档（事件） |
| 会话复盘 | 聊天 + 本会话文档 → 更新子树与 tag；常模变更要更高门槛 |
| Dream | 跨会话采纳常模修订、去重、结晶 |

硬规则倾向：

- 称谓/评价/秘密/承诺带 Person 或 Event 槽  
- **单次会话不得整体覆盖 Bond 常模**（除非显式高确信修订流程）  

### 6.2 读取

1. 入口：当前 Person 的常模枝 + 近期波动标记  
2. 不够 → Episode 文档  
3. 再不够 → session 聊天  
4. 注入：常模（如何相处）+ 必要时的事件细节；避免每次倾倒原话  

### 6.3 冲突、信任、自信

- 假冲突：补情境槽（如称谓对 A/B 不同）  
- 真冲突：修订边，保留证据  
- 信任 / 自我认同：见既有讨论；与常模修订共用「慢变」纪律  

### 6.4 合理化

仅当多条高确信结构顶死、无法同时保全时生成叙事补丁；非常模日常更新手段。

## 7. 已实现：Reflection、会话复盘与双层 Dream

| | 会话复盘 Session Review | 证据型 Reflection | 生成式 Dream |
|--|-------------------------|-------------------|--------------|
| 时机 | exit / manual / idle 20m | dirty + 6h 冷却 / manual | 持续张力 / manual |
| 范围 | 尚未 Review 的当前会话片段 | 被选 Seed + T2 Episode 证据 | 筛选后的张力摘要 |
| 长期写入 | 只能 Seed / 状态观察 | observe=Proposal；agent=合法 Mutation | 只能 generated Seed |
| 真实性 | 不保存完整 Transcript | 必须搜索、读取、声明真实证据 | 想象不得成为证据或 Episode |
| No change | 允许 | 允许并持久化 Run | 允许 |

## 8. T3 上线顺序

1. 已用 `REFLECTION_MODE=observe` 验证决策与预期修改。
2. 已完成仓库内 24 个纯虚构案例 × 3 次与浏览器实时/历史回放。
3. 安全类别 48/48，一致经历与冲突 23/24（95.8%）；产品默认已切到 `agent`。

T3 第一轮仍不自动写 Principle、World 或 Soul；不引入全库定时扫描、向量重构或图扩散。

## 9. 最佳实践与治理

### 9.1 去重与实体对齐

- Person/Event 对齐；写入前查邻域  
- 稳定 id 与显示名分离  

### 9.2 证据与修订

- 常模边带 `confidence`、`support_count`、`last_confirmed_at`  
- 修订可追溯；Episode/聊天可回指  
- **画像导出 ≠ 第二真相源**（可由 Bond 生成人读视图）  

### 9.3 预算与遗忘

- 旁路先常模后细节；聊天按需  
- 冷 Episode 可降权但仍可经 tag 找回  

### 9.4 隐私与披露

- 社交层判断说不说；多 Person 默认按 Bond 隔离  

### 9.5 评测（深度看见相关）

- 单次会话是否错误翻转告模（应接近 0）  
- 波动时是否给出 H1/H2 而非静默改写  
- 召回是否多数轮次只靠树+少量文档即可相处  

## 10. 变更时请更新本文

- §4 产品原则（分层权威、常模、双假设）  
- 图/文档/聊天的写入门槛与召回顺序  
- 会话复盘 / Dream 对常模的权限  
- 存储实现与迁移  
- 从规划挪到现状的已落地项
