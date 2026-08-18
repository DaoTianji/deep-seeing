# Deep-Seeing 长期记忆系统设计

> 版本：v0.1（设计共识稿）  
> 状态：Phase 1–4 已实现（见 [memory-ltm.md](./memory-ltm.md)）；Birth Gate 见 [birth-gate.md](./birth-gate.md)  
> 关联：[memory-stm.md](./memory-stm.md) · 认知共识：[memory-cognition.md](./memory-cognition.md)  
> 非目标：导购 CRM 式「用户画像变现」；本设计服务 **深度看见**（主体觉察人与己）

---

## 0. 一句话

长期记忆不是一份用户档案，也不是程序强制抽取的备忘录，而是 **种子在灵魂本能之上自主生长出的、可演化、可追溯、分层权威的情境模型**：整体认知在图上慢变；具体事件在文档；原话在聊天；**记住什么主要由主体用工具自行决定**（因对「自己」有价值）；召回呈上已有记忆供使用，而非代替主体决定写什么。

**Soul** 只保留主体性等成长条件，不含具体的人。**mudnet** 作为 Origin Record 永久保存，但 **Origin Introduction 仅 first_boot 呈现一次**，之后靠 Bond/Episode，不每轮重复注入。图初始化仅弱事实 `KNOWS` + `role_at_origin`，不预置 `TRUSTS=high` /「导师」。Bond 由 Episode 生长；主 Agent 不可任意 `patch_bond`。

出生前横向要求见 [birth-gate.md](./birth-gate.md)。

---

## 1. 问题与动机

### 1.1 要解决什么

Agent 若只有扁平笔记或向量碎片，会出现：

- 称谓/偏好等 **假冲突**（缺「对谁成立」）
- **一次会话推翻长期印象**
- 记得「标签」却说不清 **从哪来、是否过时、是否推断**
- 召回相似文本，却不知道 **平常该如何与此人相处**

深度看见需要：

1. **自我认知**：我是谁、原则、如何与人相处  
2. **对人常模**：ta 是什么样的人、关心什么、边界在哪、平时状态如何  
3. **事件精度**：某次发生了什么、原话如何  
4. **波动觉察**：异常时，是我看错了，还是 ta 今天不对  

### 1.2 与主流方案的关系

| 路线 | 强项 | 不足（相对本产品） |
|------|------|-------------------|
| 文档记忆 | 可读、可策展 | 结构弱、难慢变常模 |
| 向量记忆 | 语义相似召回 | 多跳/时间/证据弱；易拆不清「整体 vs 细节」 |
| 纯图记忆 | 关系、多跳 | 过度结构化易拆碎完整语义（业界反例） |
| User Context Graph（导购/CRM） | 人-商品-意图闭环 | 主语是「服务用户」，非 Self 觉察 |

本设计采用：**Graph（整体）+ Episodic 文档（事件）+ Raw 聊天（原话）**；向量为可选增强，**不替代**前三层。  
技术曲线可参考 Temporal Graph / Memory Writer / Hybrid Retrieval；产品主语是 **Self + Bond**，不是 User Profile Service。

### 1.3 设计口号

> **先有事，后有树；树在复盘里长。**  
> **整体以图为主；细节以文档和聊天为主。**  
> **十次形成的印象，不被一次沟通整体改写。**

---

## 2. 设计原则

1. **分层权威**：不同层回答不同问题；禁止用单次 Episode 静默覆盖 Bond 常模。  
2. **可追溯**：凡进入图的结论，尽量带来源 Episode / session、时间、置信度、陈述类型。  
3. **区分认知等级**：`user_said` / `behavior` / `model_inferred` / `model_decided` 不得混写成「事实」。  
4. **时间有效**：关系可有 `valid_from` / `valid_to`；改变是新区间，不是无痕覆盖。  
5. **Write 即策略**：不是每条消息入长期记忆；编码 / 复盘 / Dream 三套门槛。  
6. **Hybrid 召回**：结构优先，完整语义回原文；承认「关系对了但答案不全」的风险。  
7. **Know → Act**：系统成功的标准是相处与决策更好，不是字段填满。  
8. **主体性**：Self 有脊梁（认同、原则）；社交披露由处境判断，非法内容仍可硬拦。

---

## 3. 总体架构

```text
                    ┌─────────────────────────────┐
                    │     Raw Event Store (STM)   │
                    │  会话聊天记录 · 可 TTL/冷存   │
                    └──────────────┬──────────────┘
                                   │ 回合结束轻量编码
                                   ▼
                    ┌─────────────────────────────┐
                    │   Episodic Store（文档层）    │
                    │  值得记忆的事 · 用户时间线卷   │
                    └──────────────┬──────────────┘
                                   │ 会话复盘 / Dream
                                   ▼
                    ┌─────────────────────────────┐
                    │  Context Graph（Neo4j）      │
                    │  Self · Person · Bond常模    │
                    │  Episode指针 · 原则 · 信任   │
                    └──────────────┬──────────────┘
                                   │
          SideQuery / Tools ◄──────┤
                                   │
                                   ▼
                         Agent Planning / Action
                                   │
                                   ▼
                              Feedback 回写
```

### 3.1 三层职责

| 层 | 存储建议 | 管什么 | 变慢程度 | 权威问题 |
|----|----------|--------|----------|----------|
| **L0 Raw** | Redis STM + 可选冷日志 | 原话、工具轨迹 | 快、可过期 | 「当时到底怎么说的」 |
| **L1 Episodic** | 文件系统 Markdown（可后迁对象存储） | 值得记的事件正文 | 中 | 「某次发生了什么」 |
| **L2 Graph** | Neo4j | 常模、关系、原则、指针、慢变结论 | **慢** | 「ta 通常怎样 / 我如何与 ta 相处」 |

可选 **L1.5 Vector**：仅对 Episode 正文做模糊入口；命中后仍回 L1 原文。第一期可不做。

### 3.2 与 STM 的边界

- STM：当前会话工作记忆与压缩（见 STM 文档）。  
- LTM：跨会话持久；会话结束触发 **Session Review** 是主要交接面。  
- 压缩丢掉的 STM 窗口：重要结论应已/将被写入 L1；不得假设聊天永在。

### 3.3 回合流水线（目标态）

```text
1. STM.Get
2. LTM Recall：Bond 常模枝 (+ 近期波动标记)
3. 不够则工具下钻 Episode / chat
4. Assembler → Compactor(STM) → Eino ReAct
5. STM.Append
6. Online Writer：候选 Episode（默认不改常模）
7. （会话结束）Session Review：更新子树 / 常模提案
8. （低频）Dream：常模慢更新、去重、结晶
```

---

## 4. 本体论（Context Graph）

### 4.1 节点类型

| Label | 含义 | 关键属性 |
|-------|------|----------|
| `Self` | 主体（助手实例） | `id` = `agent_id` |
| `Person` | 真人/账号 | `id`（如 `user:{user_id}` / `person:{slug}`）、`display_name` |
| `Episode` | 值得记忆的事（图上多为指针+摘要） | `id`、`title`、`summary`、`doc_uri`、`session_id`、`created_at` |
| `Concept` | 主题/兴趣/场景等可指称概念（可选） | `id`、`name` |
| `Principle` | Self 的原则/方法（结晶） | `id`、`text`、`confidence` |
| `Tension` | 未决冲突（后期） | `id`、`description` |

### 4.2 边类型（核心）

| 边 | 语义 | 备注 |
|----|------|------|
| `(:Self)-[:KNOWS]->(:Person)` | 认识/交互过 | |
| `(:Self)-[:BOND]->(:Person)` | **常模主边**（或等价：Person 上挂 Bond 子图） | **慢变**；见 §5 |
| `(:Episode)-[:ABOUT]->(:Person\|:Concept\|:Self)` | 事件关于谁/什么 | 可多 ABOUT |
| `(:Episode)-[:IN_SESSION]->(:Session)` | 可选 Session 节点 | 或仅属性 `session_id` |
| `(:Self)-[:HOLDS]->(:Principle)` | 持有原则 | |
| `(:Principle)-[:CRYSTALLIZED_FROM]->(:Episode)` | 原则来源 | |
| `(:Self)-[:TRUSTS\|:DISTRUSTS]->(:Person)` | 信任（v2） | 带证据 |
| `(:Person)-[:CALLS {name}]->(:Self)` | 称谓约定 | 情境化，避免全局单值名 |
| `(:Claim)-…` | **第一期不强制 Claim 碎节点** | 完整语义优先留 Episode 正文；图上用摘要+边 |

> 反例教训：勿把一句完整偏好拆成过多细边导致召回丢信息。细节以 `doc_uri` 回读为准。

### 4.3 边/常模通用属性（Provenance）

所有进入 L2 的「认知性」边建议具备：

```text
confidence          // 0~1
kind                // user_said | behavior | model_inferred | model_decided
explicit            // 是否用户明确陈述
strength            // 可选：偏好强度
scope               // 可选：适用域（如 running_shoes / 沟通）
valid_from
valid_to            // null = 现行有效
last_confirmed_at
source_episode_ids  // []string
source_session_ids  // []string
support_count       // 支撑次数
```

### 4.4 Bond 常模（深度看见主界面）

`BOND`（或 `Person` 附属 `Norm` 节点）建议字段分组：

| 分组 | 内容示例 | 更新门槛 |
|------|----------|----------|
| 基础 | 称呼偏好、角色关系、时区/语言等稳定信息 | 中 |
| 性格与风格 | 沟通风格、节奏、直接/委婉 | **高**（多次或强证据） |
| 关注点 | 平时关心/关注什么 | 中高 |
| 边界 | 能接受 / 不能接受 | **高** |
| 状态基线 | 「平时」语气、能量、稳定度的描述性基线 | 中 |
| 相处策略 | Self 总结的「平常该怎么与 ta 说话」 | 随常模修订 |

**单次 Session Review 默认**：只写 Episode、更新 `last_seen`、可写「波动观察」；**不得**整体替换性格/边界段落，除非走 §6.3 慢更新协议。

---

## 5. Episodic 文档层（L1）

### 5.1 粒度

- **不要**「一会话一篇唯一主文档」作为真相源。  
- **推荐**：
  - 每条值得记的事 → 一个 Episode 文件（或时间线卷中的一节 + 稳定 `episode_id`）  
  - 每用户可选 **时间线卷**：`people/{person_id}/journal/YYYY-MM.md`（人读/追加友好）  
  - 图上 `Episode.doc_uri` 指向文件或 `#anchor`

### 5.2 Episode 正文最小结构

```markdown
---
id: ep_...
person_ids: ["user:local"]
session_id: cli-...
created_at: ...
kind: event | preference | boundary | state_observation | self_note
---

## 发生了什么
...

## 原话摘录（可选，短）
...

## Self 当场判断（可选）
...
```

### 5.3 与图的关系

- L1 = **证据与完整语义**  
- L2 = **索引、关系、常模、决策用压缩认知**  
- 删除/归档 Episode 时，图边应降置信或切断 `source_episode_ids`，避免幽灵结论

---

## 6. 写入体系（Memory Writers）

默认取向：**主体自主**（工具），不是程序强制抽取。

### 6.1 主体工具（Phase 1 主路径）

- `write_episode` / `read_episode` / `search_episodes`
- 仅在自认为有长期价值时写入；可带 `why`
- **cmd/see 默认 `NoopPostTurn`**，不跑强制 Extractor

### 6.2 Online Writer（可选实验，非默认）

**输入**：本轮 user/assistant  
**输出**：0..N 条 Episode  
**禁止默认开启**：与「种子」原则冲突——程序替主体决定该记什么  

若实验开启：仍禁止改写 Bond 性格/边界。

### 6.3 Session Review / Dream（后期）

同前：会话结束提案常模；Dream 慢更新。波动双假设 H1/H2 仍适用。

---

## 7. 读取与召回

### 7.1 默认顺序

```text
1. Graph：当前 Person 的 BOND 常模 + 近期 state_observation 标记 + 必要 Principle
2. 不够 → 按 tag/id 读 Episode 文档
3. 仍不够或争议 → 读 session 聊天（L0）
4. （可选）向量只作 Episode 入口
```

注入主模型时：优先「如何相处」+「是否需注意的波动」；避免每轮倾倒原话。

### 7.2 SideQuery（旁路）

- 预算：条数/token 上限（如常模摘要 + ≤K 条 Episode 摘要）  
- 路由直觉：相处/偏好/边界类 → 图；具体事实/时间 → Episode；核对语气 → chat  

### 7.3 工具面（目标）

| 工具 | 作用 |
|------|------|
| `recall_bond` | 读某 Person 常模与波动标记 |
| `read_episode` | 按 id 读事件正文 |
| `search_episodes` | 按人/时间/关键词列 Episode |
| `read_transcript` | 按 session_id 读聊天（受限长度） |
| `write_episode` | 模型主动记一件值得记的事 |
| `propose_bond_update` | 仅提案，不直接写死常模 |
| （后期）`read_principle` / 图邻域展开 | |

现有 `read_memory` / `write_memory` 迁移期可适配为 Episode 读写，文案与语义逐步替换。

---

## 8. 冲突、信任、自我认同

### 8.1 冲突类型

| 类型 | 处理 |
|------|------|
| 假冲突（缺情境） | 补槽：如 `CALLS` 分 Person |
| 时间冲突 | 双区间并存，现行看 `valid_to is null` |
| 真冲突（同槽互斥） | 新边 + 旧边关闭；或挂 Tension |
| 撞击 Self 认同 | 优先澄清/反驳；高门槛才改 Self |

### 8.2 信任

`TRUSTS` / `DISTRUSTS` 必须挂 Episode；欺骗事件 → 升不信任，可修订。

### 8.3 合理化

仅当多条高确信结构无法同时保全时生成；非常模日常更新手段。

---

## 9. 数据与配置（实现映射）

| 配置 | 用途 |
|------|------|
| `NEO4J_*` | L2 Context Graph |
| `REDIS_*` / STM 配置 | L0 会话 |
| `LTM_EPISODE_DIR`（建议） | L1 根目录，如 `data/memory/episodes` |
| `LTM_BACKEND`（迁移期） | `markdown` 旧路径 / `graph` 新路径 |

租户：`identity.TenantScope`（`user_id` + `agent_id`）→ `Person.id` / `Self.id`。

---

## 10. 分阶段落地

### Phase 0 — 原则与契约（本文）

字段纪律：慢变 vs 快变；三层权威；双假设。

### Phase 1 — Episodic 先行 + 种子取向（进行中/落地）

- Episode 文件库；旁路读索引；**记忆写入主路径 = 主体工具 `write_episode`**
- **默认关闭**强制 AfterTurn Extractor（避免程序规定该记什么）
- Soul（成长条件）+ Origin Context（mudnet 自我介绍，弱先验）
- **不做**：向量、Neo4j（本 Phase）

### Phase 2 — Graph 常模骨架（已落地）

- Neo4j：`Self` / `Person` / `Episode` 指针 / `BOND` / `CALLS`
- 启动种子：`KNOWS.role_at_origin` + 空 `BOND`（不预置 trust / 导师）
- SideQuery：常模优先（`BondAwareSideQuery`）
- 工具：`recall_bond` / `set_explicit_bond_fact` / `propose_bond_update`；图失败降级 Episode-only
- **已收权**：主 Agent 不再暴露 `patch_bond`

### Phase 3 — Session Review（已落地）

- 会话结束 / `/review`：**回看机会**（允许 No change）
- 可写 `state_observation` Episode；更新图 `last_seen`
- Bond 变更只入提案队列；**不直接改写常模**
- 偏离时输出 H1/H2；工具 `propose_bond_update`

### Phase 4 — Dream + Mutation Ledger（已落地）

- `/dream`：**巩固机会**（允许 No change）
- 采纳提案 → `PatchBond`（高门槛仍 append）+ Mutation Ledger
- Permission 三档骨架；`/backup`；Turn traces
- Birth 单测骨架见 `internal/birth`

### Phase 5 — 增强（可选）

- 向量入口、信任/认同细边、合理化、Canon/小说、群体图谱  

**明确不做（早期）**：聊天全量入图；用图替换推荐排序；一会话强制重建整棵终身树。

---

## 11. 评测与成功标准

| 指标 | 期望 |
|------|------|
| 单次会话错误翻转告模 | ≈ 0 |
| 偏离时常模外是否出现 H1/H2 或等价观察 | 高 |
| 多数轮次仅靠常模+少量 Episode 可正常相处 | 高 |
| 细节问题能否下钻到正确 Episode/原话 | 高 |
| Know → Act：有常模时策略是否更贴边界/关注点 | 定性+抽样 |

对照实验（可选）：旧扁平 Markdown vs 本方案，在长程对话上的「乱叫名 / 乱改印象 / 记错事件」率。

---

## 12. 风险与对策

| 风险 | 对策 |
|------|------|
| 图过度结构化丢语义 | Episode 正文权威；边保持粗粒度 |
| 推断污染 | `kind` 强制；推断不可当 user_said |
| 复盘过重/过频 | 仅会话结束；在线只编码 |
| 隐私与泄露 | Bond 隔离；披露边+后果；工具读聊天需预算 |
| 工程复杂度 | 严格按 Phase；Phase 1 可无 Neo4j |

---

## 13. 文档维护

- **本文**：目标架构与契约；重大原则变更时升版本（v0.2…）。  
- [memory-ltm.md](./memory-ltm.md)：实现现状、与代码差距、迁移说明。  
- [memory-stm.md](./memory-stm.md)：L0 会话与压缩。  

变更代码或配置时，同步更新「现状」文档；原则变更先改本文。
