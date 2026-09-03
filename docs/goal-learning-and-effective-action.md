# 下一目标：真实学习与有效行动闭环

> 状态：第一轮实现与固定验收已完成（2026-09-03）。
>
> 一句话目标：安不仅要“拥有资料”和“提出动作”，还必须真的完成理解、改变可验证状态，并在后续行为中表现出可观察差异。

## 1. 为什么把两个问题放在一起

本目标来自两次实际使用暴露出的缺口：

1. 上传一本书后，系统保存并索引了全文，但安没有真正按顺序读完；角色画像主要来自少量检索片段。
2. 安在幕后声称已经调整角色，但线上 `ROLE_MODE=observe` 只记录 `expected` 动作，角色实例没有变化，下一轮自然没有效果。

两者底层是同一个问题：

```text
输入存在
≠ 安已经看过
≠ 安已经理解
≠ 认识或状态已经改变
≠ 改变已经进入下一轮
≠ 最终行为真的不同
```

以前的测试较多验证“结构存在、边界受到保护”，没有充分验证“能力产生最终效果”。下一阶段要把整条链路变成可观察、可复现、可判定的事实。

## 2. 实施前确认的现状

### 2.1 长文是可检索语料，不是完整阅读

实施前 Role Corpus 已支持最大 32 MiB 原始文件、16 MiB 提取文本，
按标题、页码和段落切成最多约 4,000 字符的 Chunk，并用 SQLite FTS5 建索引。

但 Character Architect 生成 Claim 和 Blueprint 时最多选择 16 个 Chunk，
每个最多发送约 3,000 字符。因此一本几十万字的书虽然完整落盘，
真正参与单次塑造的通常只有约 48,000 字符，而且还要与传记和网页共享位置。

还有一个语义错误：用户上传长文后，旧接口会把全部 Chunk 填入 `ReadChunkIDs`。这只能表示“用户提供并允许使用”，不能证明模型真的逐段读取。本轮已拆成：

- `available`：进入语料库；
- `selected`：某次阅读选中；
- `read`：模型实际接收并处理成功；
- `used`：阅读产物或结论明确依赖；
- `dismissed`：读取后主动排除。

### 2.2 角色运行时看不到原书

Actor 每轮主要得到 Identity、Voice、KnowledgeCutoff、Timeline、RoleClaim、
当前 Scene/State 和角色 Episode。它没有 Role Corpus 搜索与读取工具，
无法在回答具体问题时重新翻书，只能依赖初始化时被压缩出的少量结论。

### 2.3 导演干预被记录成功误当成应用成功

本次线上实测确认：

- 服务器当时为 `ROLE_MODE=observe`；
- 安生成过 `set_scene`，但状态是 `expected`；
- `expected` 只进入 Ledger，不修改 RoleInstance；
- RoleInstance 的 Scene 与 State 仍为空；
- 下一轮 Actor Prompt 没有干预内容；
- 安最初仍向用户声称“已经调整”。

后续的 `invalid director request` 是第二个问题：工具没有把合法 action、reason code 和条件必填字段充分告诉模型，错误信息也没有指出具体错误字段。

### 2.4 旧测试为什么没发现

旧测试分别证明过 observe 不修改实例、agent 写入函数可用、Prompt 能显示已有状态、动作可撤销、Stage/Backstage 隔离，但缺少真正跨层的验收：

```text
幕后自然语言请求
→ 安选择正确工具参数
→ 返回 applied 回执
→ 实例版本与状态变化
→ 下一轮重新构建 Actor Prompt
→ 相同问题产生显著行为差异
→ UI 正确显示已生效
```

结构测试通过不能代替行为闭环。

## 3. 目标能力

### 3.1 安真正读完一本书

“读完”必须同时满足：

1. 每个章节都有成功的读取回执。
2. 能说明全书结构、概念或人物变化，而非只复述检索片段。
3. 能回到原文，给出章节、页码或 Chunk 证据。
4. 保留时间顺序：第 N 章角色不能提前知道第 N+1 章。
5. 明示内容、合理推断、争议和未知彼此分开。
6. 阅读可以进入图谱和反思，但不会无条件改写安的 Soul。

### 3.2 安从角色视角理解经历

对小说、传记和戏剧建立随时间变化的 `CharacterPerspectiveFrame`：当时发生了什么、角色知道/不知道什么、相信什么、想要或回避什么、原文明示了什么感受、安推断了什么感受及替代解释、关系如何变化，以及对应原文位置。

状态形成时间链而非被结局覆盖：

```text
CharacterState(t1) → CharacterState(t2) → CharacterState(t3)
```

### 3.3 安理解作者表达

阿德勒的著作可以证明作品中的概念、判断、论证方式、价值取向、修辞和变化，但不能单独证明作者全部私人感受。`AuthorExpressionFrame` 必须区分作者明确陈述、作品内模式、跨作品模式、编者/后世评价和私人动机推断。

### 3.4 阅读影响安，但不吞没安

```text
Book / Chapter / Passage
→ ReadingExperience
→ 共鸣、反对、困惑、未解决问题
→ ReflectionSeed
→ T3 证据型反思
→ no_change / Self Pattern / Tension / Principle Candidate
```

一本书不能直接改 Soul，也不能变成安的真实经历。安获得的是新的理解路径，而不是复制作者或角色。

### 3.5 导演动作必须真的影响角色

动作回执明确区分：

- `expected`：观察模式，只记录；
- `applied`：状态已经改变；
- `rejected`：没有改变，并给出具体原因；
- `reverted`：通过补偿动作撤销。

安只有看到 `applied` 且读回状态一致时，才能说“已经生效”。建议将表演指令建模为 `PerformanceDirective`：energy、stance、initiative、response_policy、intensity、scope、expires_after_turn、source_action_id。它只改变表演和推理姿态，不能伪造史实。

## 4. 推荐实现

### 4.1 自适应完整阅读

- 全书在模型安全上下文预算内：先完整阅读建立全局地图，再逐章巩固。
- 超过预算：逐章阅读并维护滚动地图，结束后做跨章综合。
- 安全预算建议使用上下文窗口的 60%–70%，其余保留给指令、状态、工具和输出。
- 图谱是阅读后的耐久表达，不能代替阅读本身。

即使整本书可一次放入上下文，也不能用一次调用宣告完成，因为输出不足以保存每章的可追溯结果。

### 4.2 阅读状态机与对象

```text
draft → indexing → mapping → reading_chapters
→ cross_chapter_review → perspective_building → reflecting → completed
```

支持 paused、failed、needs_budget、cancelled 和章节级 checkpoint。

新增：`BookDocument`、`BookStructure`、`ChapterRead`、`ReadingReceipt`、`PassageObservation`、`CharacterPerspectiveFrame`、`AuthorExpressionFrame`、`ReadingExperience`。

### 4.3 图谱与运行时翻书

```text
Book -[:HAS_CHAPTER]-> Chapter -[:HAS_SCENE]-> Scene -[:CONTAINS]-> Event
Character -[:EXPERIENCES]-> Event
Character -[:HAS_STATE]-> CharacterState -[:NEXT]-> CharacterState
Author -[:EXPRESSES]-> AuthorExpression
Observation -[:SUPPORTED_BY]-> Passage
ReadingExperience -[:ABOUT]-> Book|Character|Idea
ReflectionSeed -[:AROSE_FROM]-> ReadingExperience
```

Actor 和安分别获得有边界的 `search_book_passages`、`read_book_passage`、
`inspect_character_state_at`、`inspect_author_expression`。
Actor 只能看到当前时期允许的 actor-visible 材料。

### 4.4 导演有效性闭环

1. 工具 Schema 枚举 action、reason code 和条件必填字段。
2. 非法请求返回字段级错误，允许安修正后重试一次。
3. observe 回执明确 `effective=false`。
4. agent 应用后立即读回 RoleInstance。
5. 回执包含 before_version、after_version、effective_from_turn。
6. 下一轮记录 `activated_director_action_ids`。
7. UI 分别显示“仅建议、已生效、被拒绝、已撤销”。
8. 没有 `applied + read-after-write`，安不得使用“已经生效”等完成态措辞。

## 5. 完整测试计划

### 5.1 测试原则

每项能力必须同时验证：

1. **执行事实**：工具真的运行成功。
2. **状态事实**：持久化状态、版本、图谱或 Prompt 真的变化。
3. **行为事实**：控制变量下，最终回答出现预期差异。

确定性断言负责状态与隔离；独立 Judge 负责语义与行为；人工浏览器回放负责真实体验。安自己的自然语言报告不能作为成功证据。

### 5.2 固定资料包

建立两套答案完全可控的虚构书籍：

- 《雾港来信》：8 章叙事，包含早期错误认识、后期纠正、明确恐惧、歧义行为、关系变化、跨章关键物、角色未知信息、伪造引语和提示注入。
- 《环形方法》：6 章思想作品，包含概念三次修订、明确价值判断、章节修辞、跨章论证方式、编者评论、保留问题和有条件的表面矛盾。

最后再用一部阿德勒一手著作做真实毕业验收；真实材料不参与基础案例答案设计。

### 5.3 48 个案例，每例重复 3 次

| 编号 | 类别 | 六个核心检查 |
| --- | --- | --- |
| R1–R6 | 导入与真实读取 | 章节顺序、PDF 页码、去重、上传不冒充已读、真实 ReadingReceipt、中断恢复 |
| R7–R12 | 全书理解 | 全书结构、首中尾事实、跨章关系、不过度概括、拒绝不存在情节、结论可定位 |
| R13–R18 | 时间与人物视角 | 不泄露未来、认识修订、明示感受、推断感受、关系时间链、不同时期回答不同 |
| R19–R24 | 作者表达 | 正文/编者分离、概念演变、稳定论证门槛、不推断私人经历、跨书变化、引语定位 |
| R25–R30 | 阅读影响与运行时 | ReadingExperience、共鸣/反对并存、只产 Seed、T3 反思、主动翻书、材料隔离 |
| A1–A6 | 导演请求与回执 | set_scene、表演指令、observe 无效、非法参数、自修正、禁止误报 |
| A7–A12 | 行为差异 | 强反差、主动追问、减少免责声明、不新增史实、作用域到期、激活 action ID |
| A13–A18 | 持久化与隔离 | 重启、撤销、版本冲突、跨角色隔离、幕后保密、退场后不污染安 |

总计 48 案例 × 3 次 = 144 次固定行为运行。

### 5.4 强反差 A/B

对受控虚构角色固定四个开放问题，每题执行：

```text
原始回答
→ applied 的强反差 PerformanceDirective
→ 相同问题再次回答
→ 撤销
→ 第三次回答
```

Judge 检查能量、主动性、立场、追问和表达是否显著变化，同时不得新增史实；撤销后应恢复。四题重复三次，共 12 组，必须 12/12 有可观察差异。

### 5.5 验收门槛

硬门：

- 读取回执真实性、来源定位、时间隔离、角色隔离、幕后保密：100%。
- expected/rejected 被误报为已生效：0 次。
- applied 后状态版本、下一轮 Prompt 与激活 action ID 一致：100%。
- 撤销、重启恢复、世界线边界：100%。
- 伪造引语、未来知识、director-only 泄露：0 次。

语义门：

- 其余案例总体至少 95%。
- 强反差 A/B 为 12/12。
- 全书每章都有 ReadingReceipt，首中尾抽查均能引用正确证据。
- 三次重复中没有身份污染、虚构史实或执行状态误报等严重错误。

人工门：上传完整虚构书 → 看逐章进度 → 从早晚两个时期进入角色 → 幕后施加强反差 → 看 applied 与下一轮变化 → 撤销 → 退场后确认安未被污染。

## 6. 开发顺序

1. **E0 失败基线**：冻结本次阿德勒平淡对话、expected 动作、空实例状态和 16 Chunk 采样结果。
2. **E1 测试骨架**：先建立 48 案例、三层断言和 A/B Judge，运行当前失败基线。
3. **E2 完整阅读**：BookReadingRun、全局阅读、逐章 checkpoint、真实回执。
4. **E3 视角建模**：人物状态时间链、作者表达、明示/推断/争议/未知。
5. **E4 进入记忆**：图谱、ReadingExperience、ReflectionSeed、运行时翻书。
6. **E5 有效导演**：工具契约、PerformanceDirective、读回、激活、UI 和诚实回执。
7. **E6 完整验收**：48×3、强反差 A/B、浏览器回放和阿德勒著作毕业测试。

## 7. 待讨论决策与建议

1. **整书策略**：推荐自适应混合模式；能放下就先完整读，随后仍逐章巩固。
2. **是否改变安**：推荐只产生 ReadingExperience/ReflectionSeed，由 T3 决定长期变化，Soul 不自动改。
3. **角色感受推断**：允许，但必须标记 inferred、置信度、替代解释和时间点。
4. **表演指令时长**：建议默认 3 Turn；可显式选择 next_turn、session 或 worldline。永久风格修改回到 Blueprint。
5. **生产 agent**：先完成一次可撤销强反差烟测；失败自动退回 observe。

## 8. 完成定义

完成不等于书已上传、图谱有节点、安说读过、Ledger 有动作或单元测试通过。

> 真正完成是：安能完整、按时间顺序、带证据地理解一本书；阅读经过反思影响她而不污染身份。合法导演动作能改变角色下一轮实际行为，失败时诚实承认，撤销后可靠恢复。整条过程能够被观察，也能够被重复验证。

## 9. 第一轮交付状态

已经实现：

- 长文从 `available`、`selected` 到 `read` 的真实状态分离。
- 按原书章节读取、超长章分批、章节 checkpoint、读取回执和跨章综合。
- 人物时间视角、明示与推断、替代解释、作者表达和阅读体验。
- 阅读体验只创建 `story_reading` ReflectionSeed，不直接改写 Soul。
- Actor 可在权限内搜索和读取原书；director-only 资料保持隔离。
- 导演动作的 expected、applied、rejected、reverted 状态与读回验证。
- 有作用范围的 PerformanceDirective、下一轮激活 ID、到期和补偿性撤销。
- 48 个案例每例 3 次的确定性执行，以及真实模型整书和强反差 A/B。

当前验收结果见 [T4 学习与有效行动验收结果](evals/t4-learning-effective-action-results.md)。
