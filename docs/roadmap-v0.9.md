# Deep-Seeing v0.9 议题清单（D0）

> 状态：D0 已立；**T1 契约已冻终**，T1-Read 可开发；T2–T4 仍先讨论。
> 基线：v0.8 · 认知共识：[memory-cognition.md](./memory-cognition.md) · 现状：[memory-ltm.md](./memory-ltm.md) / [memory-stm.md](./memory-stm.md) · 前序能力：[roadmap-p5-p8.md](./roadmap-p5-p8.md) · 契约：[p5.0-contracts.md](./p5.0-contracts.md)

## 0. 版本一句话

v0.9 **不是**再铺 P9 大功能，而是把 v0.8 已有的记忆容器**真正用起来**：从「会记笔记的书记员」叠到「对人有常模、想起往事看处境、长期看法只慢改」。

方法：**讨论 → 写清契约与对照例子 → 薄切片实现 → 可观察验收**。答案在沟通中长出，不一次定死实现细节。

## 1. v0.8 已具备什么（一页内）

| 能力 | 人话 | 工程摘要 |
|------|------|----------|
| STM | 当面聊得下去，太长会概括 | Redis 会话 + Compaction |
| Episode / 笔记 | 能记事、能翻旧账 | 文件 Episode + 工具 CRUD |
| 关系草图 | 有 Self/Person/Bond/Episode 节点，但日常很少顺着图想事 | Neo4j 种子 + 工具提案；召回仍偏 SideQuery / 最近条 |
| 机会式复盘 | 想起来可以整理，不是每晚必做 | Session Review / Dream 入口 + Mutation Ledger |
| 自我与外脑架子 | 有自我观察、未完成思考、意图、上网 | Self / Workspace / Agency / World（P5–P8） |

**口感缺口**：日记可以很多，仍像第一次见面；召回偏「找相似段落」；常模不会诚实地慢变。

## 2. 叠层顺序（先谈什么）

```text
D0  议题地图与边界          ← 本文（已完成）
 ↓
T1  常模参与对话            ← 默认第一个实现主题
 ↓
T2  状态条件召回
 ↓
T3  复盘与 Dream 巩固节奏
 ↓
T4  遗忘与 Prediction Error ← 默认以设计共识为主，不强制完整落地
 ↓
v0.9.0
```

每层共用四步，**未完成讨论不进入大规模编码**：

1. 问题陈述（人比喻 + 对照例子）
2. 范围边界（必做 / 不做 / 后置）
3. 最小契约（读写权威、失败降级）
4. 薄实现 + 可观察（Mind / traces 能看见想起了什么、改或不改）

## 3. 主题议题清单

### T1 — 常模参与对话

| | |
|--|--|
| **状态** | 契约已冻终；**T1-Read / Model / Write / Scene / Strategy 缓存已落地** |
| **人话问题** | 日记很多，没有「你平时怎样」 |
| **要解决** | 规定 Agent 可以如何「认识一个人」、怎样形成长期判断、怎样防止一次误解永久污染 |
| **对照例（验收口感）** | 同一 Person：有常模后，语气/策略能区分「熟人」与「几乎陌生人」 |

#### 主轴共识（已冻）

> **槽位不是为了把记忆整理整齐，而是语义治理单元：**
> 规定系统**允许形成哪几类**关于人的长期结论，以及这类结论**凭什么成立、如何改、如何注入**。

```text
Raw / 聊天原话
  → Episode：发生过什么（证据）
  → Reflection：这些事说明了什么
  → Slot：属于哪一种「关于此人的长期认识」
  → Bond：当前整体人物模型（结论层）
  → Strategy（派生）：Self 当下该怎么做（不是 Bond 真相源）
```

| 概念 | 职责 |
|------|------|
| Episode | 管经历（证据） |
| Slot | 管认识类型（治理） |
| Item / claim | 管一条可独立撤销的稳定结论（记忆原子） |
| Evidence 指针 | 管可信度与可回溯 |
| SceneNorm | 管适用范围（场景） |
| Strategy | 管如何行动（派生视图） |

**Slot vs Item：** 槽是治理单位；槽内是若干 Item，不是小作文。推断不是原罪——**无证据的推断**才是。

**全局有效 ≠ 每轮全文注入。** Global vs Scene 裁判：去掉当前场景后是否大半仍成立。

#### 结构终稿（已冻）

```text
Global Bond
├── Basics
├── Interaction      ← 原 Style；禁止人格标签
├── Boundaries       ← 重大错误快写**唯一**白名单
├── Priorities       ← 原 Concerns
└── Baseline

Derived（非 SoT）
└── Strategy         ← 旧 Strategy 字符串**不再作为真相源**
```

迁移读路径：`Style→Interaction`，`Concerns→Priorities`；旧散文可拆为 `legacy` Item。

#### 工程冻结（开发启动闸门）

| 项 | 冻结值 |
|----|--------|
| 槽名 | 上表终稿 |
| 快写 | **仅 Boundaries** |
| 工具 | `recall_bond`；`set_explicit_bond_fact` 仅 Basics/称呼；`propose_bond_update` 按 slot+item；`append_bond_boundary`；`list/read/write_scene_norm`；`set_bond_strategy_cache`；禁止 `patch_bond` |
| 表② N | Interaction/Baseline：`N=3` 或确认 1 次；Priorities：跨会话 2 次或宣称 |
| Item 上限 | Boundaries 20；Interaction 15；Priorities 15；Baseline 10 |
| 占位 | `对该人常模尚薄，避免臆测人格；优先询问与观察。` |
| Strategy T1-Read | 默认不注入旧 SoT 散文；**版本匹配的派生缓存**可注入 ≤120 |
| 切片 | T1-Read → Write → Scene → Strategy 缓存（本切片已齐） |

#### 三张契约表（已冻）

##### ① 结论类型

| 类型 ID | 槽 | 认识论 | 典型该写 | 不该写 |
|---------|-----|--------|----------|--------|
| fact_profile | Basics | 事实 | 称呼、语言、关系标签 | 今日心情 |
| interaction_pref | Interaction | 观察/归纳 | 互动偏好 | 人格标签 |
| boundary_rule | Boundaries | 规则 | 红线、禁区 | 无规则宣泄 |
| long_priority | Priorities | 长期主题 | 长期关注域 | deadline/旅行 |
| behavior_baseline | Baseline | 行为模式 | 可观察节奏 | 心理诊断 |
| action_policy | Strategy（派生） | Policy | 派生行动 | 人物 SoT |

##### ② 成立 / 修改 / 撤销

| 类型 ID | 成立 | 快写 | 冲突 |
|---------|------|------|------|
| fact_profile | 明确表达或可核对 | 否 | 最新明确表达 |
| interaction_pref | N=3 或确认 1 次 | 否 | 追加为主 |
| boundary_rule | 红线或重大踩线 | **是（唯一）** | 明确表达优先 |
| long_priority | 跨会话 2 次或宣称 | 否 | 合并 |
| behavior_baseline | 多次模式 | 否 | 缓慢修订 |
| action_policy | Bond 派生 | 不写 Bond | 以 Bond 重算 |

单条注入裁剪：**80** 字（Interaction 等）。

##### ③ 注入预算（已冻）

| 优先级 | 内容 | 预算 |
|--------|------|------|
| 必注入 | Basics + Boundaries | 合计 ≤ **800** 字；空则占位 |
| 常注入 | Interaction top-**5**，每条 ≤ **80** | 每轮 |
| 条件注入 | Priorities/Baseline（关键词命中） | ≤ **400** 字 |
| 派生 | Strategy | T1-Read **省略**；若启用 ≤ **120** 字 |
| 场景 | SceneNorm | T1-Scene |

#### 开发同步

- 落点：`BondAwareSideQuery`、Assembler、`graph` compact、`observe.TurnTrace`、`SceneStore`、`strategy_cache`
- 旧 `Strategy`：compact **丢弃 SoT**；命中 `strategy_cache_version == bond_version` 时注入派生缓存
- SceneNorm：关键词旁路命中后并入 Bond 段；traces 含 `scene_ids`
- 验收：空占位；Boundaries+称呼可见；Interaction top-N；不相关 Priorities 默认不进；场景命中旁路；版本失配不注 Strategy

**后置：** H1/H2；群聊；向量；Selector；Scene 产品化（编辑 UI / 冲突治理）。

---

### T2 — 状态条件召回

| | |
|--|--|
| **状态** | 待讨论（依赖 T1 常模可读） |
| **人话问题** | 只会找相似段落；同一句话在不同处境想起的往事几乎一样；记忆越多越吵 |
| **要解决** | 从「哪段过去和这句话像」变成「此刻的自己该不该、容不容易想起这段」 |
| **对照例（验收口感）** | 「我该不该离开」×（事业兴奋 vs 失恋疲惫）→ 浮起的往事应可区分 |

**讨论必须收敛：**

1. `RetrievalState` **最小字段**：哪些来自会话 / Intent / Workspace / Bond，哪些本版本不做？
2. 候选来源：现有 SideQuery 与**图展开**如何分工？向量检索是否本版本引入？
3. 排序维度白名单先上哪 **3～4** 个（相似、关联、新旧、目标相关、状态契合、来源置信…）？
4. Recall ≠ Retrieve：注入的是结论摘要，还是必须可回溯 Episode？

**本主题预期包含：** 显式 RetrievalState（可先规则/启发式）；图展开至少服务「当前 Person → Bond + 相关 Episode 指针」；traces / Mind 可解释本轮选中原因。

---

### T3 — 复盘与 Dream 巩固节奏

| | |
|--|--|
| **状态** | 待讨论 |
| **人话问题** | 只有即时笔记；常模不会诚实慢变 |
| **要解决** | 编码 → 复盘 → 巩固；重要性可事后再判；单次冲击不翻长期画像 |
| **对照例（验收口感）** | 多次温和证据才动常模；单次冲击进 Episode（或状态波动标记），常模可明确 No change |

**讨论必须收敛：**

1. Session Review 触发：exit / 手动 / idle / daemon？默认仍机会式，还是可开关的轻自动？
2. Dream 频率与输入：跨会话合并什么、**绝不碰**什么？
3. 常模修订门槛：次数、冲突强度、是否必须 Mutation Ledger？
4. 与双假设 H1/H2：复盘产出是提案还是直接写？

**本主题预期包含：** 三条路径职责表落地；Review/Dream 可追踪且允许 No change；至少一个手跑验收剧本。

---

### T4 — 遗忘与 Prediction Error（设计为主）

| | |
|--|--|
| **状态** | 待讨论；**默认不强制完整算法落地** |
| **人话问题** | 旧事永远同等大声；只靠次数更新，真正颠覆预期的事不够「刺痛」 |
| **要解决** | 「还在不在」≠「现在还影不影响我」；意外比重复更能驱动慢更新 |
| **本版本默认产出** | 共识文档 + 可选数据钩子（如 accessibility / last_recalled_at）；非完整遗忘产品 |

**讨论必须收敛：**

1. 六种遗忘形态里，v0.9 是否只预留可达性降权接口？
2. Prediction Error 的操作定义（相对常模的偏离如何计量）与 Dream/提案的关系？
3. 明确拒绝：通用 decay 公式崇拜；默认物理删除。

**例外：** 若 T2 讨论证明「没有衰减就无法控噪」，允许在 T2 末尾加**最小可达性降权**，仍不展开完整遗忘产品。

## 4. v0.9 明确不做

- 强制 24h LLM 常驻生成 / 定时全量反思
- 直接改 Soul
- 拍板「一个通用 Memory Writer + 一个 decay 公式 + 一个 ranker」当作最终真理
- 默认以大规模向量库重构为前提（除非 T2 讨论证明召回离不开）
- 无限自主打扰、外部不可逆操作默认放行（继承 P5–P8）
- 把 T4 完整遗忘 / PE 引擎当作 v0.9 必达

## 5. 横切议题（全程带着问）

| 议题 | 问什么 |
|------|--------|
| 来源 / `experience_mode` | 角色扮演、听说、推测如何避免污染真实 Bond？拦截点在写还是在巩固？ |
| Observability | 每层能否看见「想起了什么 / 为何改或不改」？ |
| Know→Act 评测 | 过去是否合理影响现在，又未把现在永远囚禁在过去？每主题至少 1 个手跑剧本 |
| 文档同步 | 契约进设计/专题稿；落地改 [memory-ltm.md](./memory-ltm.md) 现状段 |

## 6. 版本成功标准（宏观）

同时满足即可称 v0.9，**不必**等 T4 算法完美：

1. 对固定 Person，对话默认能用上**可读常模**（不再只靠近期日记）
2. 召回至少在一个对照剧本上体现**处境差异**（状态门控可粗糙）
3. Review 或 Dream 至少一条路径能**慢改常模或明确拒绝改**，且有 ledger / 提案痕迹
4. 文档能回答：叠了哪几层、每层解决什么、下一版本还缺什么

## 7. 主题状态板

| ID | 主题 | 讨论 | 契约 | 实现 | 备注 |
|----|------|------|------|------|------|
| D0 | 议题地图 | 完成 | — | — | 本文 |
| T1 | 常模参与对话 | 完成冻终 | 三表已冻 | Read/Model/Write/Scene/Strategy 已落地 | 已完成 |
| T2 | 状态条件召回 | 进行中 | 待冻结 | — | 当前工程焦点 |
| T3 | 复盘与 Dream | 待开始 | — | — | |
| T4 | 遗忘与 PE | 待开始 | — | — | 设计为主 |
| — | 打标 v0.9.0 | — | — | — | 成功标准满足后 |

## 8. 下一沟通入口

T1 已完成，当前工程焦点切换到 **T2 状态条件召回**。
先冻结 `RetrievalState`、候选来源、排序维度和可回溯契约，再进入薄实现。

## 9. 变更时请更新本文

- 主题状态板
- 「明确不做」与成功标准若有增删
- 某主题讨论冻结后的结论摘要与文档链接
