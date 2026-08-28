# T2 状态条件召回：完整计划与实现状态

> 核心原则：把“是否回忆、查什么、采用什么”的语义决策交给主 Agent；系统提供事实、工具、证据约束和公开轨迹，不用关键词规则替它作决定。

## 1. 目标

T2 要把召回从“哪段文字和当前输入相似”推进到：

> 对此刻的这个人、任务和目标而言，哪些过去现在值得被想起？

| 阶段 | 解决的问题 | 当前状态 |
|---|---|---|
| T2.1 自主召回 | Agent 自己决定是否搜索 Episode；Legacy 可回退 | 已完成 |
| T2.2 证据闭环 | 候选不等于证据；必须读取后声明采用/排除 | 已完成，48/48 |
| T2.3 当前处境 | 任务薄快照、按需调查、会话焦点、切换和歧义询问 | 已完成，真实 Agent 27/27（36 Turn）通过 |
| T2.4 多源候选 | 六类来源统一候选、语义角色、公开轨迹与激活回放 | 已完成，真实 Agent 39/39 通过 |
| T2.5 注意与竞争 | 会话级中心/支撑/外围槽位、显式竞争与跨回合公开轨迹 | 已完成，真实 Agent 48/48 通过 |
| T2.6 稳定上线 | 长期回归、成本和延迟优化、降级，评估是否默认启用 Agent 模式 | 未开始 |

向量搜索、图扩散、竞争抑制和动态可达性不是预设必做；只有前面薄切片证明需要时才引入。

## 2. T2.3 当前处境

T2.3 只建模任务与项目处境，不推断情绪、人格、关系温度或心理状态。它由两个薄切片组成。

### 2.1 每轮任务事实快照

Runtime 只在 `agent` 模式下，每轮重建一个事实快照：

- 当前 `person_id` 和 `session_id`。
- 最多 4 个 `open / in_progress` Workspace 卡片。
- 最多 4 个 active Intent 卡片。
- 数据源状态：`available / unavailable / error`。
- 卡片只有 ID、类型、状态、标题和时间。
- 已选择的会话焦点优先占一个卡位，即使它不是最近更新的四项之一。

快照明确不含 Workspace / Intent 正文、summary、自动相关性结论、情绪推断或历史事实结论。它不写入 Episode、Bond、Workspace 或 Intent。

### 2.2 调查、选择与会话连续性

Agent 可以按当前任务自主使用：

- `list_workspace` / `list_intents`：薄快照没有目标时发现更多候选。
- `read_workspace` / `read_intent`：核对正文。
- `report_context_focus`：公开声明本轮处境结论。

`report_context_focus` 是最小、公开、临时的 RecallIntent 表达：

| 字段 | 允许值或含义 |
|---|---|
| `action` | `continue / switch / check / compare / clarify / clear` |
| `certainty` | `clear / ambiguous` |
| `workspace_id / intent_id` | 实际选中的条目；新 ID 必须先读取 |
| `needs_user_confirmation` | 只有 `clarify / ambiguous` 时为 `true` |

状态规则：

- `continue / switch` 更新当前会话的临时焦点。
- `check / compare` 只记录本轮结论，不改变后续默认焦点。
- `clarify` 不选择任何 ID、不覆盖旧焦点，并要求向用户询问。
- `clear` 删除会话焦点。
- 同一 Turn 只接受一次焦点声明。
- 用户当前明确表达始终优先于旧会话焦点。
- 会话焦点只存在进程内存，不进入任何长期存储。
- `legacy` 模式不生成快照，也不暴露 `report_context_focus`。

完整路径：

~~~text
当前输入 + STM + 薄快照 + 会话焦点
                 ↓
        Agent 判断是否需要处境
          ├─ 不需要：直接回答
          └─ 需要：调查更多候选（可选）
                       ↓
                    读取正文
                       ↓
             ┌─────────┴─────────┐
             │                   │
         能可靠区分           仍有歧义
             │                   │
     report_context_focus     clarify + 询问用户
             │
       会话内继续 / 切换
~~~

### 2.3 公开轨迹与可视化

- `task_context`：版本、数据源状态、卡片 ID、进入本轮前的会话焦点。
- `task_context_expansions`：`list / read`、来源、ID 或结果 ID、成功或错误。
- `task_context_focus`：最终 action、certainty、选中 ID 与是否需要确认。
- Room 实时显示“快照 → 调查 → 展开 → 焦点/歧义”；历史 Trace 可回放同样状态。
- Trace 不保存 Workspace/Intent 标题、summary、正文或隐藏思维过程。

## 3. 测试与完成门槛

确定性测试覆盖：

- 快照限额、active 过滤、独立降级和正文隔离。
- 会话焦点在卡片溢出时仍优先进入快照。
- 新焦点未读取时拒绝；已读或已存在焦点可声明。
- 重复、冲突或非法 focus 声明被拒绝。
- list/read/focus 事件按 Turn 隔离且不落正文。
- Intent 列表只返回候选卡，不返回 Body。
- `continue / switch / check / compare / clarify / clear` 的状态变化。
- Agent/Legacy 工具与快照隔离。
- Room 实时事件、历史 Trace 和前端语法。

机器可读案例位于 `evals/t2/task_context_cases.json`，schema 2 共 9 类、12 个连续 Turn：

1. 普通任务不调查。
2. 明确 Workspace 选择。
3. 下一轮省略名称仍延续会话焦点。
4. 用户明确切换覆盖旧焦点。
5. 指代不明时声明歧义并询问。
6. 目标不在前四张卡片时通过列表发现。
7. 明确 Intent 核对。
8. Workspace 与 Intent 联合处境。
9. 用户明确离开项目时清除焦点。

~~~bash
# 校验案例与确定性规则
go run ./cmd/eval-task-context

# 使用隔离的虚构数据运行真实 Agent
go run ./cmd/eval-task-context -live -repeat 3 \
  -out data/evals/t2-task-context-v2.jsonl
~~~

全量 Go 测试、前端语法、案例静态契约和补丁检查已通过。经用户明确授权，真实 Agent 使用仓库内纯虚构夹具在当前 `ops-ai` 网关完成 9 类案例 × 3 次复验：27/27 通过，共 36 个连续 Turn；没有触发 Episode 搜索，也没有改变 Episode、Workspace 或 Intent 的持久化数据。原始 JSONL 被 Git 忽略。

此前 T2.3.1 的 15/15 薄快照行为结果见 [评估报告](./evals/t2-task-context-results.md)。T2.3 完整闭环的本地验收与真实 Agent 复验结果见 [完整评估报告](./evals/t2-task-context-complete-results.md)。

## 4. T2.3 完成定义

T2.3 只有同时满足以下条件才标记完成：

- 上述实现与确定性测试全部通过。
- 真实 Agent 的 9 类案例重复运行达到门槛；基础设施失败单独记录，不修改案例迁就结果。
- 普通任务不滥用处境工具。
- 明确任务能够调查、读取并声明正确焦点。
- 多轮连续、显式切换、溢出发现和清除焦点都符合状态机。
- 歧义场景不伪造确定性，并确实询问用户。
- 处境快照和会话焦点没有写入长期记忆。

通过后进入 T2.4 多源候选；不在 T2.3 中加入情绪状态、统一多源召回、动态排序、向量搜索或图扩散。

## 5. T2.4 统一多源召回

T2.4 已冻结为架构驱动的完整多源方案，不再以“先证明现状失败”作为是否开发的门槛。测试用于验证实现正确和行为改善，而不是决定是否建设统一架构。完整开发契约见 [t2-multisource.md](./t2-multisource.md)。

本轮统一六种上下文来源：

- Bond：自动提供的 `baseline`。
- SceneNorm：按需读取的 `guidance`。
- Workspace：当前工作的 `task`。
- Intent：未来安排的 `plan`。
- Proposal：未确认认识的 `hypothesis`。
- Episode：历史经历的 `evidence`。

实施原则：

- 建立统一 `ContextCandidate`，但保留来源专用 list/search/read 工具。
- Agent 自主选择调查来源，可以同轮组合多个来源。
- 建立统一 candidate → read → use/dismiss/focus 的公开轨迹和 Room 回放。
- Bond 保持完整自动加载；Proposal 只读 pending 且不能作为事实证据。
- Legacy 与默认 `RECALL_MODE=legacy` 保持不变。
- 不在本轮实现统一搜索入口、Recall Broker、跨来源评分、向量、图扩散或注意竞争。

当前代码已经实现统一协议、来源适配、公开轨迹、Room 激活回放与隔离评估器。机器可读案例位于 `evals/t2/multisource_cases.json`：

~~~bash
# 离线校验，不访问模型
go run ./cmd/eval-multisource

# 已授权的纯虚构夹具行为复验
go run ./cmd/eval-multisource -live -repeat 3 -judge model \
  -out data/evals/t2-multisource-final.jsonl
~~~

工程、确定性测试与真实 Agent 13 类 × 3 次行为复验均已完成，最终 39/39 通过。详见 [T2.4 评估结果](./evals/t2-multisource-results.md)。下一步进入 T2.5 注意与竞争。

## 6. T2.5 注意与竞争

T2.5.1 已实现一个仅存在于当前 session 的 Attention Workspace：

- `center=4`：预计后续回合仍需直接处理的核心焦点。
- `support=8`：已读取、可能继续补充核心焦点的材料。
- `periphery=16`：已经成为候选、尚未展开的弱线索。
- Bond 仍是自动 baseline，不参与有限槽位竞争。
- 新中心/支撑必须在本轮成功读取；新外围必须先成为本轮候选。
- 槽位满时拒绝隐式淘汰，由 Agent 通过 `manage_attention` 明确指定替换项。
- `idle_turns` 只提供可见的闲置信号，不自动降级或删除。
- 工作区不保存正文、不跨进程恢复、不写入 LTM，也不改变来源的证据角色。

`TurnTrace` 和 Room 实时事件公开记录回合开始快照与层级调整；图谱用独立外环表现中心、支撑和外围，不覆盖候选/已读/采用/排除的证据颜色。完整契约见 [t2-attention.md](./t2-attention.md)。

机器可读套件位于 `evals/t2/attention_cases.json`，共 9 类、16 个连续 Turn，覆盖无污染、焦点维持、中心与支撑、未读外围、满槽替换、显式切换、20+ 干扰项、闲置增长/读取归零和当前表达优先：

~~~bash
# 离线校验；不会访问模型网关
go run ./cmd/eval-attention

# 真实 Agent 多轮复验；只在获得本套夹具的明确授权后运行
go run ./cmd/eval-attention -live -repeat 3 -judge model \
  -out data/evals/t2-attention-final.jsonl
~~~

全量测试、前端语法、静态检查、并发测试和离线套件均已通过。经授权，真实 Agent 的 9 类 × 3 次复验共 48/48 个有效回合通过；1 次基础设施失败被自动重试并单列。T2.5 已完成，结果没有证明当前需要 Recall Broker、向量检索、图展开或自动权重；下一步进入 T2.6 稳定上线讨论。详见 [评估结果](./evals/t2-attention-results.md)。
