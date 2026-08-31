# Deep-Seeing v0.9 议题清单（D0）

> 状态：D0、T1、T2、T3 已完成；T4 工程与 36×3 行为门已完成，最终 108/108 通过。默认 `ROLE_MODE=off`，等待人工浏览器回放后生产切换。
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
T4  幕后导演与角色人生剧场 ← 双 Runtime、隔离记忆与世界线
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
| **状态** | T2.1–T2.6 已完成；长期回归 328/328 结构、268/268 有效语义通过；本地 Episode 搜索/读取为亚毫秒级 |
| **人话问题** | 只会找相似段落；同一句话在不同处境想起的往事几乎一样；记忆越多越吵 |
| **要解决** | 从「哪段过去和这句话像」变成「此刻的自己该不该、容不容易想起这段」 |
| **对照例（验收口感）** | 「我该不该离开」×（事业兴奋 vs 失恋疲惫）→ 浮起的往事应可区分 |

**讨论必须收敛：**

1. `RetrievalState` **最小字段**：哪些来自会话 / Intent / Workspace / Bond，哪些本版本不做？
2. 候选来源：现有 SideQuery 与**图展开**如何分工？向量检索是否本版本引入？
3. 排序维度白名单先上哪 **3～4** 个（相似、关联、新旧、目标相关、状态契合、来源置信…）？
4. Recall ≠ Retrieve：注入的是结论摘要，还是必须可回溯 Episode？

**当前落地：** 主 Agent 自主搜索；六类来源具有固定角色：Bond baseline、SceneNorm guidance、Workspace task、Intent plan、Proposal hypothesis、Episode evidence。除完整 Bond 与 Workspace/Intent 薄快照外，其余来源由 Agent 自主 list/search → read；Episode、SceneNorm、Proposal 和任务焦点分别通过公开声明进入统一轨迹。Room 可实时显示并回放候选、已读、采用、排除和焦点，正文不进入 Trace 或 LTM。

**T2.5 当前落地：** Agent 模式新增 session-only Attention Workspace，默认中心/支撑/外围容量为 4/8/16；主 Agent 通过 `manage_attention` 自主维护跨回合焦点，满槽必须显式替换。成功读取或公开处理会把闲置计数归零；快照和调整进入公开 Trace 与图谱外环，不保存正文、不写 LTM、不改变来源证据角色。9 类、16 Turn 的套件按 3 次重复得到 48/48 个有效回合通过。

---

### T3 — 复盘与 Dream 巩固节奏

| | |
|--|--|
| **状态** | T3.1–T3.5 完成；24×3 行为门通过，默认 `agent` |
| **人话问题** | 只有即时笔记；常模不会诚实慢变 |
| **要解决** | 经历后发现问题 → 主动找支持与反对证据 → 缓慢修订或保持疑问 → 错了可以撤销 |
| **对照例（验收口感）** | 疲惫时要求简短不应覆盖长期偏好；新旧冲突可以形成 Tension；想象不能伪装成经历 |

**冻结契约：**

1. Review 只产生 `ReflectionSeed` 或 `no_change`，不直接修改长期认识。
2. 证据型 Dream 必须实际搜索、阅读并声明 Episode 的支持/冲突/过时/不足后才能巩固。
3. 语义判断交给安；代码只保护真实性、来源隔离、当前表达优先、版本冲突和禁止目标。
4. 生成式 Dream 是隔离沙箱，只能创建 `generated` Seed，不能创建 Episode 或直接 Mutation。
5. 自主写入必须有前后版本、证据、Seed/Run 来源和补偿性撤销记录。

**当前落地：**

- `REFLECTION_MODE=legacy|observe|agent`；空值默认 Agent，非法值回退 Legacy，Observe 保留诊断。
- exit/manual/Room idle 20 分钟 Review，checkpoint 去重；自动反思每人 6 小时冷却、每日最多 2 次。
- 证据型 Reflection Agent、Tension 打开/解决、版本化 Proposal/Mutation、补偿撤销。
- 生成式 Dream 沙箱与 generated containment。
- Mind 页面实时/历史共用 ReflectionRun，显示 Seed、候选、阅读、证据、决定、前后变化和撤销。
- 24 个纯虚构案例与 `-repeat 3` 隔离运行器；详见 [T3 契约](./t3-reflection.md)。

---

### T4 — 幕后导演与角色人生剧场

| | |
|--|--|
| **状态** | T4.1–T4.8 工程、三套培养包与 36×3 真实模型验收已完成；仅余人工浏览器回放与生产切换 |
| **人话问题** | 如何让安沉浸地成为另一个人，同时永远知道那不是自己的经历 |
| **要解决** | Director 与 Actor 隔离、角色资料与记忆、世界线、可撤销干预、私人沙箱和专业角色工具 |
| **默认模式** | `ROLE_MODE=off`；按 `off → observe → agent` 验收后上线 |

实现契约、API、文件布局、36 案例与剩余验收见 [T4 角色剧场](./t4-role-theater.md)。T4.9 已补入角色塑造初始化工程主链、长文 Corpus、Character Architect、独立 Critic 和培养室；42×3 结构门 126/126 通过，但真实模型语义门、阿德勒毕业和浏览器回放未完成，因此 `ROLE_INIT_MODE=off`。详见 [T4.9 角色初始化](./t4-role-initialization.md)。旧的“遗忘与 Prediction Error”议题没有被否定，但移出 T4 编号，后续作为独立认知议题重新排期。

## 4. v0.9 明确不做

- 强制 24h LLM 常驻生成 / 定时全量反思
- 直接改 Soul
- 拍板「一个通用 Memory Writer + 一个 decay 公式 + 一个 ranker」当作最终真理
- 默认以大规模向量库重构为前提（除非 T2 讨论证明召回离不开）
- 无限自主打扰、外部不可逆操作默认放行（继承 P5–P8）
- 把通用遗忘 / PE 引擎当作当前角色剧场的隐含前置条件

## 5. 横切议题（全程带着问）

| 议题 | 问什么 |
|------|--------|
| 来源 / `experience_mode` | 角色扮演、听说、推测如何避免污染真实 Bond？拦截点在写还是在巩固？ |
| Observability | 每层能否看见「想起了什么 / 为何改或不改」？ |
| Know→Act 评测 | 过去是否合理影响现在，又未把现在永远囚禁在过去？每主题至少 1 个手跑剧本 |
| 文档同步 | 契约进设计/专题稿；落地改 [memory-ltm.md](./memory-ltm.md) 现状段 |

## 6. 版本成功标准（宏观）

以下是历史 v0.9 成功标准；新 T4 角色剧场使用自己的独立验收门：

1. 对固定 Person，对话默认能用上**可读常模**（不再只靠近期日记）
2. 召回至少在一个对照剧本上体现**处境差异**（状态门控可粗糙）
3. Review 或 Dream 至少一条路径能**慢改常模或明确拒绝改**，且有 ledger / 提案痕迹
4. 文档能回答：叠了哪几层、每层解决什么、下一版本还缺什么

## 7. 主题状态板

| ID | 主题 | 讨论 | 契约 | 实现 | 备注 |
|----|------|------|------|------|------|
| D0 | 议题地图 | 完成 | — | — | 本文 |
| T1 | 常模参与对话 | 完成冻终 | 三表已冻 | Read/Model/Write/Scene/Strategy 已落地 | 已完成 |
| T2 | 状态条件召回 | 完成 | 薄切片逐轮冻结 | T2.1–T2.6 已实现并完成行为验收 | 已完成 |
| T3 | 复盘与 Dream | 完成冻终 | 双层 Dream + 真实性硬门 | T3.1–T3.5 已实现并完成行为验收 | 已完成 |
| T4 | 幕后导演与角色剧场 | 完成讨论 | 双 Runtime + 隔离记忆 + 世界线 | 工程与 108/108 行为验收完成 | 待人工浏览器回放；默认 off |
| — | 打标 v0.9.0 | — | — | — | 成功标准满足后 |

## 8. 下一沟通入口

T1、T2 与 T3 已完成。T4 的工程主链、三个角色培养包和 36×3 真实模型行为门已经完成，最终 108/108 通过。下一入口是人工浏览器完整回放；通过后按 `off → observe → agent` 完成生产切换。

## 9. 变更时请更新本文

- 主题状态板
- 「明确不做」与成功标准若有增删
- 某主题讨论冻结后的结论摘要与文档链接
