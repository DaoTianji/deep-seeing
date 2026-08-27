# T2 状态条件召回：完整计划与实现状态

> 核心原则：把“是否回忆、查什么、采用什么”的语义决策交给主 Agent；系统提供事实、工具、证据约束和公开轨迹，不用关键词规则替它作决定。

## 1. 目标

T2 要把召回从“哪段文字和当前输入相似”推进到：

> 对此刻的这个人、任务和目标而言，哪些过去现在值得被想起？

完整路径分为六层：

| 阶段 | 解决的问题 | 当前状态 |
|---|---|---|
| T2.1 自主召回 | Agent 自己决定是否搜索 Episode；Legacy 可回退 | 已完成 |
| T2.2 证据闭环 | 候选不等于证据；必须读取后声明采用/排除 | 已完成，48/48 |
| T2.3 当前处境 | 给 Agent 可靠的当前人物、任务、项目和约定线索 | 3.1 薄快照已实现 |
| T2.4 多源候选 | Bond、SceneNorm、Workspace、Intent、Proposal、Episode 统一成为可选来源 | 未开始 |
| T2.5 注意与竞争 | 根据目标、关系、状态、时间和可信度动态排序，处理上下文槽位满载 | 未开始 |
| T2.6 稳定上线 | 长期回归、成本和延迟优化、降级，评估是否默认启用 Agent 模式 | 未开始 |

向量搜索、图扩散、竞争抑制和动态可达性不是预设必做；只有前面薄切片证明需要时才引入。

## 2. T2.3.1 任务/项目事实快照

本轮冻结的范围：

1. 先做任务与项目处境，不推断情绪、人格、关系温度或心理状态。
2. Runtime 每轮在 Agent 模式重建一个薄快照。
3. 快照只给活跃 Workspace / Intent 卡片；Agent 判断确有需要时，再调用 `read_workspace` / `read_intent` 展开。
4. 快照不写入 Episode、Bond、Workspace 或 Intent，不成为长期记忆；只在本轮 Prompt 和公开 Trace 中存在。
5. Legacy 模式不生成该段，保持原行为。

### 快照中有什么

- 当前 `person_id` 和 `session_id`
- 最多 4 个最近更新的 `open / in_progress` Workspace 卡片
- 最多 4 个 active Intent 卡片
- 数据源状态：`available / unavailable / error`
- 卡片只有 ID、类型、状态、标题和时间

明确没有：

- Workspace / Intent 正文
- 自动相关性判定
- 当前目标分类
- 情绪或关系推断
- 历史事实结论

### 展开与轨迹

~~~text
当前输入 + STM
       +
任务处境薄快照（卡片）
       ↓
Agent 判断是否需要延续某项任务
       ├─ 不需要：直接回答
       └─ 需要：read_workspace / read_intent
                    ↓
             公开 expansion 事件
~~~

`task_context` Trace 只保存版本、来源状态和卡片 ID；`task_context_expansions` 只保存来源、ID、成功或错误。正文不会写入 Trace。

数据源单独降级：Workspace 失败不会阻断 Intent，反之亦然；快照失败不阻断聊天。

## 3. 测试

确定性测试覆盖：

- 只选 active Workspace，排除 paused / done / archived。
- Workspace 与 Intent 卡片数量有硬上限。
- Prompt 不包含正文与 summary。
- Trace 不包含标题和正文。
- Legacy 不加载快照；Agent 才加载。
- 一个来源失败时另一个来源仍可用。
- Workspace / Intent 成功与失败展开按 Turn 隔离。
- Prompt 段为空时不出现，非空时位于 Recall policy 前。
- Room 实时事件和历史 Trace 能显示快照数量与展开次数。

机器可读行为案例位于 `evals/t2/task_context_cases.json`：

~~~bash
# 只校验案例与规则
go run ./cmd/eval-task-context

# 使用隔离的虚构 Workspace / Intent 运行真实 Agent
go run ./cmd/eval-task-context -live -repeat 3 \
  -out data/evals/t2-task-context.jsonl
~~~

案例包含：普通任务不展开、明确 Workspace 延续、明确 Intent 延续，以及相同输入在两个不同项目快照下读取不同 Workspace。原始 JSONL 被 Git 忽略。2026-08-27 的 15/15 行为结果见 [评估报告](./evals/t2-task-context-results.md)。

## 4. 下一步

T2.3.2 不急着加入情绪状态。先用行为结果判断三个问题：

1. 只有标题卡片时，Agent 能否稳定知道何时展开？
2. 多个活跃项目同时存在时，Agent 是正确选择、搜索更多线索，还是先询问？
3. 同一句输入在不同任务快照下，是否稳定激活不同项目，而且不会把项目标题当作正文证据？

这些成立后，再讨论最小 `RecallIntent`：当前目标、连续性需要、指代假设和缺失信息。它仍应是临时、可承认不确定的工作状态，而不是长期画像。
