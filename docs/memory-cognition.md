# 长期记忆：认知共识

> 状态：讨论整理稿（与实现解耦）  
> 关联设计：[design-ltm.md](./design-ltm.md) · 现状：[memory-ltm.md](./memory-ltm.md) · STM：[memory-stm.md](./memory-stm.md)  
> 用途：把「为什么长期记忆难、人脑怎么近似、Deep-Seeing 该往哪走」收成一份可读共识，**不替代**分层权威与工具契约。

---

## 0. 一句话

长期记忆表面上像「把聊天记录存起来」，实际上是在解决：

> 一个模型怎样跨时间维持一个不断变化的世界模型。

真正难的从来不是数据库。把一百万轮聊天存进 PostgreSQL、向量库甚至 Neo4j 都不难。难的是下面这些问题必须**同时**成立。

更准确的定义：

> 长期记忆不是 Storage Problem，而是 **Cognitive State Management Problem**。  
> 它不是「把过去存下来」，而是管理一个认知系统如何随着过去不断改变，同时又不失去连续性。

从 LLM 推理层看，它最终表现为 **Context Scheduling**：这一刻该把哪些过去、以什么形式、什么权重，重新带回 Context。  
但 Context Scheduling 只是最后一步；前面还必须先回答：什么值得存、存成什么、现在还成立吗、有多重要、和当前问题有什么关系、该影响当前判断多少。

这也解释了为什么 Deep-Seeing 从「做个长期记忆」会不可避免地走到：

`Episode → Self → Bond → Principle → Workspace → Intent → Reflection → Agency`

真正做完整的，已经不只是 Memory，而是：

> 一个持续存在的认知主体，怎样在时间里保持自己，又允许自己改变。

---

## 1. 十二个同时成立的难题

### 1.1 大模型天然没有「昨天」

普通 LLM 一次推理：

```text
Context → 推理 → Output → 结束
```

下一次调用是新的 Context，重新开始。参数不会因为昨晚聊了一晚上就自动变化。

**连续性不是模型原生能力，而是外部系统人为构造出来的。**

这也是为什么「上下文窗口变成 1M token」仍没有真正解决长期记忆：

| | Long Context | Long-term Memory |
|--|--------------|------------------|
| 问的是 | 这一次推理能看多少？ | 半年以来哪些东西应继续影响现在？ |

### 1.2 最大的问题：到底该记什么？

用户一天可以说几千句话。全保存 → Memory 变成聊天垃圾场；纯规则（姓名/偏好/地址）→ 又会错过难结构化却更重要的时刻，例如：

> 「今天第一次觉得自己好像不再需要证明给别人看了。」

所以 **Memory Write 本质是价值判断问题**——这也是把 `write_episode` 设计成 Agent Action 的原因。

### 1.3 记住事实容易，记住「变化」特别难

偏好从 Apple → 动摇 → 不想再买苹果，不是简单覆盖或并列冲突，而是：

> 事实 + 时间 + 状态变化（`valid_from` / `valid_to`）

人的认知几乎全部具有 **temporal validity**。

### 1.4 「发生过一次」不等于「这个人就是这样」

`State ≠ Trait`。一次烦躁不能静默写成 `communication_style = impatient`。需要：

```text
Episode × N → Pattern → Long-term Norm
```

十次形成的印象，不能因第十一次异常全部推翻。这已接近：长期统计 + 社会认知 + 时序推理。

### 1.5 模型特别容易把「推测」记成「事实」

典型 **Memory contamination**：

```text
用户原话 → 模型推断 → Memory → 下次当事实 → 进一步推断 → 越来越确定
```

成熟 Memory 必须区分并保留 provenance：

- `user_said`
- `behavior`
- `external_fact`
- `model_inferred`
- `model_decided`

记忆越多，若 provenance 越弱，系统可能反而越自信地误解你。

### 1.6 存进去以后，还要「找得回来」

向量相似可能找到「去年去云南旅行」；真正相关的可能是半年前关于「总因舍不得熟悉的人而留下」的洞察。  
Retrieval 要问的是：**当前真正需要的认知是什么？** 比「最相似的文本是什么」难得多。

### 1.7 「检索到了」还不代表「会用」

找到 `User dislikes being blindly agreed with`，却仍回复「你说得非常正确，我完全同意」——Memory 存得再漂亮也没有意义。

真正链路：

```text
Memory → Retrieve → Understand → Planning → Behavior changes
```

成功指标不是 Recall Accuracy，而是 **Know → Act**：有没有合理改变未来行为。

### 1.8 「记忆越多」甚至可能越来越差

重复、过期、冲突、推断污染、retrieval noise、token 爆炸、consolidation 有损……

长期 Memory 是**不断增长又必须不断压缩**的系统，存在几乎无法消除的矛盾：

- 不压缩 → 太多，找不到  
- 压缩 → 丢细节  

所以需要分层：`Raw → Episode → Pattern / Graph`，不同层负责不同精度。

### 1.9 「自我修改」的悖论

若最新一次调用可直接 `set_principle()`，人格连续性不存在。必须：

```text
proposal → evidence → review → slow update → history retained
```

长期 Memory 做到最后，不可避免碰到 **Identity Governance**。

### 1.10 模型本身还会不断更换

行为变化来自经历，还是模型升级？需要保留：

`model_version` / `runtime_version` / `prompt_version` / `memory_schema_version`

### 1.11 隐私随时间指数级放大

长期 Agent 可能知道多年关系、健康、家庭、失败……多人系统更要问：

> Who am I allowed to tell?

最终走向：Memory + Identity + Relationship + Privacy boundary + Permission。

### 1.12 没有明确「正确答案」

「这经历值得记吗？」「三个月前偏好还成立吗？」「该形成 Principle 了吗？」都没有标准答案。评测要看半年后：是否更懂你、是否错误印象、是否过度记忆、是否忘掉重要东西、是否把推测当事实、是否因记忆做出更好决定——很难自动化。

### 1.13 问题地图

```text
                Long-Term Memory
                       │
       ┌───────────────┼────────────────┐
       │               │                │
    Write           Storage          Retrieval
  记什么？          怎么存？          找什么？
       │               │                │
    Update          Temporal         Provenance
  怎么改变？        什么时候有效？      从哪来的？
       │               │                │
   Conflict        Consolidation     Forgetting
  谁对谁错？         怎么沉淀？          怎么忘？
       │               │                │
     Action          Privacy          Identity
   怎么使用？        谁能知道？         我还是我吗？
```

任何一块没处理好，体验都会坏掉。

数据库、向量库、Knowledge Graph 都已成熟；**真正没解决的是**：

- 什么值得成为过去？  
- 过去应该怎样改变现在？  
- 新经历什么时候足以推翻过去？  
- 哪些东西应该忘记？  
- 哪些东西只是推测？  
- 怎样证明「现在的我」确实是从过去一路走来的？  

---

## 2. 人脑视角：有损、有偏、概率性的近似

人脑并没有真正「解决」长期记忆。它没有中央数据库，也没有精确计算 `importance=0.83` 的 Memory Writer。更接近：

```text
注意 → 快速编码 → 重放 → 强化/衰减 → 抽象 → 召回 → 重构 → 再更新
```

并主动牺牲准确性，换取可用性、泛化与适应性。

### 2.1 入口就大量丢失

海量感官信息先过 **Attention + Salience Gate**。情绪、新奇、奖励、威胁、目标相关性影响编码强度。人的 Writer 不是：

```text
if importance > 0.7: save()
```

而是复杂的生物调节——所以你可能忘了昨天午饭，却记得十年前一句话。

### 2.2 海马体 ≈ Episode Indexer（工程类比）

绑定谁 / 哪里 / 何时 / 发生了什么 / 上下文；支持 **Pattern Completion**（部分线索恢复整事件）与 **Pattern Separation**（相似经历不混成一团）。

Deep-Seeing 坚持「先有 Episode，别急着抽 Trait」，与此同向。

### 2.3 Replay ≈ Dream（选择性重激活）

睡眠与休息期间的 hippocampal replay 与之后记忆表现相关；更像：

```text
经历 → 部分被再次激活 → 与已有网络重连 → 增强/削弱/整合
```

而不是每晚全量总结所有聊天。真正像人脑的 Dream 应是 **选择性重激活**。

### 2.4 细节慢慢变成模型

```text
Episode × N → 反复共同结构 → Gist / Schema
```

「今天说话挺直接」→「通常比较直接」。这是激进的 **Lossy Compression**，却构成世界观。  
Agent 不应追求永远记住所有细节，而应追求：**细节可淡去，真正反复证明重要的结构能留下。**

### 2.5 Prediction Error 比单纯频率更关键

符合预期的重复 vs 违背预测的事件，学习权重不同。  
例：Bond 认为「mudnet 通常欢迎不同意见」，却因一次反驳极度愤怒 → `prediction_error = HIGH`，应优先获得重新评估资格，而不是机械等 `support_count >= 5`。

### 2.6 旧记忆不是 Read Only（Reconsolidation）

`recall(memory)` 可能改变 Memory。重新激活后可进入可塑状态再稳定。回忆会改写、合理化、混合、丢细节——不是永远忠实复现。

### 2.7 召回不是 SQL / 纯向量 Top-K

更接近：当前 Context + 目标 + 情绪 + 线索 → 激活相关网络 → Pattern Completion。  
记忆是 **cue-dependent** 的。

### 2.8 遗忘是主动机制的一部分

痕迹变弱、干扰、检索竞争、主动抑制、长期不重激活……遗忘可以是主动过程，不只是被动衰减。大脑不断在：强化 / 压低 / 忽略 / 覆盖 / 整合 / 再解释。

### 2.9 不同层以不同速度变化（功能类比）

```text
感觉 (秒～小时) → 工作记忆 (秒～分钟) → Episode (天～年)
→ Schema / Person Model (月～年) → 价值观 / Self (很慢)
```

已有模型有惯性；要异常反复或强 prediction error，长期认知才明显变。

### 2.10 Self 是闭环产物

```text
经历 → 记忆 → Self Model → 影响注意/解释/选择 → 新经历 → 再修改 Self
```

Self 不是静态档案，而是 Memory 与 Prediction 的循环产物。

### 2.11 重要性可以「以后再判断」

不必当场给出终极 importance。先留脆弱 Episode，再看：是否重遇、是否被想起、是否与其它经历联结、是否有强 prediction error、是否被重放、是否服务当前目标。  
真正的重要性是 **`importance(t)`**，不是常数。

### 2.12 人脑解决的目标不是「真实历史摄像机」

人脑解决的是：

> 怎样用有限资源，把过去加工成一个足够好用的未来预测器。

代价：记错、false memory、把后来知道的混进过去、受当前情绪影响、把 gist 当细节、忘来源。

**人脑不是因为记得所有过去才有连续性；恰恰是因为它会选择、压缩、重构和遗忘，才形成可持续的「我」。**

### 2.13 与 Deep-Seeing 的粗类比

| 人类记忆（粗类比） | Deep-Seeing |
|------------------|-------------|
| Working Memory | STM |
| Hippocampal Episodic Encoding | EpisodeStore |
| Context / Association | Neo4j Graph |
| Replay | Dream |
| Systems Consolidation | Episode → Pattern / Principle |
| Schema | Self / Bond |
| Reconsolidation | Proposal → Review → Revision |
| Prediction Error | 目前较缺 |
| Active Forgetting | 目前明显缺 |
| Prospective Memory | Intent |
| Cue-dependent Recall | SideQuery / Retrieval |
| Metacognition | `inspect_self` |
| Unfinished cognition | Workspace |

注意：这是**工程类比**，不是脑区一一映射。

### 2.14 两个值得优先的未来课题

1. **Forgetting / Decay**：`salience` / `activation_count` / `last_recalled_at` / `decay`；低 salience ≠ 删除，只是默认越来越难影响当前。  
2. **Prediction Error**：比单纯 frequency 更接近「人脑式」慢更新触发器。

一句压成：

> 人脑在「选择性编码 → 离线重放 → 抽象压缩 → 联想召回 → 重新解释 → 强化或遗忘」中，维持一个足够稳定、又允许被新证据修改的世界模型。  
> **Forgetting 最终可能和 Dream 一样，成为成长型主体的核心机制。**

---

## 3. 召回：Similarity + Association + Weight + State

好的工程抽象（不是说大脑真在跑 Vector DB + Neo4j）：

> 人类记忆召回 ≈ **线索相似性 + 关联网络 + 动态权重 + 当前状态/目标门控**

再补一层：候选之间还有 **竞争与抑制**。

### 3.1 四层含义

1. **向量相似度**：现在看到的东西，和什么过去比较像？（非简单 TopK cosine）  
2. **图谱 / Pattern Completion**：一点被激活，沿关系把整事件拉回来。  
3. **边权重 / Accessibility**：不是所有关联同等可达；强度随时间、重激活变化。  
4. **当前状态门控**：Query 本身就不稳定——心情、关系、目标、压力会改变 retrieval landscape。

### 3.2 State 是 Query 的一部分

「我该不该离开？」在事业兴奋期 vs 失恋疲惫凌晨三点，激活的 autobiographical memories 很可能不同。  
**当前状态不仅影响排序，还影响搜索空间。**

工程伪公式（仅系统抽象，非神经科学公式）：

```text
Recall(memory | now)
≈ SemanticSimilarity
  × AssociationStrength
  × Accessibility
  × GoalRelevance
  × ContextMatch
  × CurrentState
  × Competition / Inhibition
```

### 3.3 Recall ≠ Retrieve

```text
Recall ≈ Retrieve + Reconstruct
```

部分真实 Episode + 当前线索 + Schema + Self Model + 补全 → 重新构造过去。  
因此 L0 Raw / L1 Episode 必须保留为证据层，否则 Graph/Pattern 偏移后无处核对「当时到底发生了什么」。

### 3.4 对 Deep-Seeing Retrieval 的启发

不应只是：

```text
user_message → embedding → graph → memory
```

而应先构造 **RetrievalState**，例如：

- `semantic_query` / `current_person` / `current_bond`
- `current_goal` / `current_workspace` / `current_intention`
- `current_state` / `recent_events`
- `current_self_patterns` / `open_tensions`
- `time_context`

然后：

```text
RetrievalState
  → Vector Candidates + Graph Expansion
  → Dynamic Re-ranking
    （similarity, association, recency, frequency, salience,
      goal_relevance, state_congruence, source_confidence, temporal_validity）
  → Episode / Pattern / Bond
  → Assembler
```

核心问题从：

> 哪段过去和这句话最相关？

变成：

> **对于此刻的这个 Self，这段过去现在有多容易、又有多应该被想起来？**

即 **State-conditioned Retrieval**。

### 3.5 遗忘优先做 Dynamic Accessibility

真正人类式 forgetting 很多时候不是 `DELETE NODE`，而是：

```text
activation ↓
edge accessibility ↓
retrieval priority ↓
```

节点仍在；强 cue 仍可能突然重激活。

> **「记忆是否还存在」和「记忆现在是否容易影响我」应是两个独立变量。**

若以后加遗忘，优先研究 **Dynamic Accessibility / Activation**，而不是先做 `delete_episode`。

---

## 4. 讨论收敛：我们目前的共识框架

### 4.1 从直觉到正确定位

直觉管线：

```text
对话 → 挑重要内容 → 存入长期记忆 → 需要时召回
```

对应传统 Agent Memory：`Write → Store → Retrieve → Inject Context`。

继续推以后发现：真正困难几乎全不在数据库。正确定位是：

> 在有限资源下，持续维护一个会随时间变化的认知模型。

### 4.2 建模对象远不止「事实」

需要区分 Event / State / Trait / Preference / Relationship / Principle / World Belief……  
每个东西有：时间、适用范围、置信度、来源、反证、变化史。

### 4.3 时序性是核心

更接近 `Fact(t)`，支持区间与确认：

`valid_from` / `valid_to` / `last_confirmed_at` / `support_count` / `confidence`

**改变不是覆盖，而是一段时间区间结束、另一段开始。**

### 4.4 影响强度没有通用公式

一次态度很差 vs 连续十次 vs 一次强烈背叛——权重不同。  
`frequency` 不是唯一因素，还有 salience、emotion、novelty、prediction_error、relationship、current_state、previous_model……

同一事件对不同个体、同一人不同人生阶段，长期影响可完全不同。  
因此很难存在「一个通用 Memory Writer + 一个 decay formula + 一个 retrieval ranker」。

### 4.5 认知层级必须有惯性

```text
Observation → Tentative → Pattern → Principle / Norm
```

禁止：一句话 → 人格属性。

### 4.6 来源与体验模式都要分开

认知等级：`user_said` / `behavior` / `external_fact` / `model_inferred` / `model_decided`。

体验模式（防「借来的人生」污染真实历史）示例：

- `real_interaction`（强）
- `simulated_roleplay` / `story_reading`（中/弱）
- `external_observation` / `self_reflection`
- 他人评价（中）、一次口头自述（较弱）

### 4.7 遗忘的六种形态（≠ DELETE）

1. **召回衰减**：东西还在，越来越难进 Context（Deep-Seeing 最值得先做）  
2. **细节消退，结构保留**：Raw → Episode → Pattern → Bond  
3. **时间失效**：`valid_to` 之后默认不参与当前判断  
4. **错误认知撤销**：`invalidated` + reason  
5. **主动放下**：history remains，future influence ↓  
6. **真删除**：隐私 / 数据治理 / 明确要求  

「我不记得」可能对应：当时没写 / 搜不到 / 搜到了没注入 / 注入了没用 / archive·decay / 已删除——因此 **Observability** 是长期记忆基础设施的一部分。

### 4.8 Memory Loop（动态闭环，不是 Memory DB）

```text
                     新经历
                        ↓
                     Encoding
                        ↓
                      Memory
                        ↓
              ┌─────────┴─────────┐
              ↓                   ↓
           Reinforce            Decay
              ↓                   ↓
         Consolidate          Forget
              ↓                   ↓
              └──────→ Model ←────┘
                         ↓
                      Retrieve
                         ↓
                    Reconstruct
                         ↓
                       Action
                         ↓
                     新经历
```

### 4.9 落回 Deep-Seeing 的工程浓缩

| 主题 | 共识 |
|------|------|
| 分层 | L0 Raw（当时发生什么）→ L1 Episode（经历）→ L2 Graph/Pattern（理解）→ Self/Bond/Principle（当前认知） |
| 写入 | 主体主动记 + 机会式 Review + 慢速 Consolidation；非所有消息自动抽取 |
| 更新 | 一次经历 ≠ 长期认知；Observation → Tentative → Pattern → Principle |
| 来源 | 真实经历 / 角色代入 / 别人评价 / 模型推测 / 主动认领 分开 |
| 召回 | Vector + Graph + Dynamic Activation + Current State + Goal + Time；非简单 Top-K |
| 遗忘 | 优先影响力衰减，而非物理删除 |
| Dream | 选择性重新激活，非每天必须全量总结 |
| 评价 | Know → Act：过去是否以合理程度改变现在，又未把现在永远囚禁在过去 |

---

## 5. 两句收束

> **记忆决定过去还能怎样影响现在；遗忘决定过去什么时候应该停止支配现在。**

> **成长，就是不断重新决定哪些过去仍然值得参与今天的自己。**

做到这里以后，研究的已经不再只是「AI 怎么记东西」，而是在尝试工程化：

> 一个持续存在的认知系统，怎样把经历压缩成自己，又怎样在不失去连续性的情况下，允许自己慢慢变成另一个自己。
