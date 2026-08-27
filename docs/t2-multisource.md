# T2.4 统一多源召回：开发方案

> 状态：工程实现与本地确定性验收已完成；新虚构夹具的真实 Agent 13 × 3 复验待用户审阅并明确授权。T2.4 采用架构驱动开发；测试用于验证实现和行为，不再作为“是否开发”的前置门槛。

## 1. 目标

让主 Agent 自主决定调查哪些来源，并能区分、读取和组合六种不同性质的上下文：

| 来源 | 角色 | 回答资格 |
|---|---|---|
| Bond | `baseline` | 已加载的稳定基础认识；用户当前表达优先 |
| SceneNorm | `guidance` | 特定场景的互动指导，不证明历史事实 |
| Workspace | `task` | 当前项目、材料和未完成思考 |
| Intent | `plan` | 未来计划与承诺，不代表已经发生 |
| Proposal | `hypothesis` | 尚未确认的认识假设，不能作为事实证据 |
| Episode | `evidence` | 读取并采用后可以支持历史结论 |

完整路径：

~~~text
用户输入 + STM + 完整 Bond + 任务薄快照
                    ↓
          Agent 自主选择调查来源
                    ↓
          来源专用 list / search
                    ↓
             统一薄候选卡
                    ↓
             读取必要正文
                    ↓
       声明采用、排除或当前焦点
                    ↓
              组合形成回答
                    ↓
          图谱回放跨来源激活回路
~~~

## 2. 统一候选协议

新增内部 `ContextSource`：`bond / scene_norm / workspace / intent / proposal / episode`。

新增内部 `ContextRole`：`baseline / guidance / task / plan / hypothesis / evidence`。

除自动加载的 Bond 外，来源的 list/search 统一输出薄候选卡：

~~~go
type ContextCandidate struct {
    Source       ContextSource `json:"source"`
    ID           string        `json:"id"`
    Kind         string        `json:"kind,omitempty"`
    Title        string        `json:"title,omitempty"`
    Preview      string        `json:"preview,omitempty"`
    Role         ContextRole   `json:"role"`
    Status       string        `json:"status,omitempty"`
    UpdatedAt    time.Time     `json:"updated_at,omitempty"`
    Metadata     map[string]any `json:"metadata,omitempty"`
    ReadRequired bool          `json:"read_required"`
}
~~~

约束：

- `Preview` 必须截断，`Metadata` 只容纳日期、模式、参与者等必要短字段，二者都不能代替正文。
- list/search 不返回完整正文。
- 候选内容实质影响回答前必须调用来源对应的 read。
- 不提供跨来源统一 `score`；各来源保留自身的原生排序。
- 不改变底层存储结构，候选卡只是读取适配层。

## 3. 来源接入

### Bond

- 保持完整 `NormSnapshot` 自动加载，不改成按需召回。
- 统一轨迹只记录 person、版本和 `available/unavailable/error`，不记录正文。
- 图谱显示为本轮已存在的基础节点，不参与候选竞争。

### Episode

- 保留 `search_episodes`、`read_episode`、`report_recall_evidence`。
- 搜索结果映射为 `role=evidence`、`read_required=true` 的候选卡。
- 现有 search/read/used/dismissed 事件同步投影到统一轨迹。

### Workspace 与 Intent

- 保留任务薄快照、list/read 和 `report_context_focus` 状态机。
- Workspace 映射为 `task`，Intent 映射为 `plan`。
- `continue/switch/check/compare` 投影到统一使用轨迹；会话焦点仍只存在内存。
- Intent 工具说明必须明确：active 只代表计划存在，不代表已经执行。

### SceneNorm

- `list_scene_norms` 改为薄候选卡，只返回 ID、标题、关键词、状态和时间，不返回 Body。
- `read_scene_norm` 才能返回正文并记录公开读取事件。
- Agent 模式由主 Agent 自主决定是否调查；Legacy 保留关键词旁路。
- SceneNorm 的使用角色固定为 `guidance`。

### Proposal

- 新增只读 `list_proposals` 和 `read_proposal`。
- 只暴露当前用户的 pending Proposal；accepted/applied 已进入 Bond，rejected 默认不可见。
- 候选包含目标 slot/kind 和截断假设，不在列表中暴露完整 rationale。
- Proposal 始终为 `hypothesis`，不能声明为历史证据，也不能通过这些工具采纳、拒绝或写 Bond。

## 4. 自主决策与使用声明

Agent 模式增加软性来源说明：过去查 Episode，当前项目查 Workspace，未来安排查 Intent，场景指导查 SceneNorm，尚未确认的认识谨慎查 Proposal；Bond 已经提供。

一个问题可以调查多个来源；不规定固定顺序、关键词、次数或强制路径。空结果正常，用户当前明确表达始终优先。

保留现有专用声明：Episode 使用 `report_recall_evidence`；Workspace/Intent 使用 `report_context_focus`。

新增 `report_context_use`，只处理 SceneNorm 与 Proposal：

- `source` 只允许 `scene_norm/proposal`。
- `id` 必须是本轮候选。
- `disposition` 只允许 `used/dismissed`。
- used 前必须成功读取；同一 ID 不能重复或同时采用、排除。
- 原因码限定为 `irrelevant/not_applicable/conflicting/stale/insufficient/superseded/duplicate/unconfirmed`。
- 不记录自由文本推理；Proposal used 只表示“作为假设参与询问或谨慎说明”。

## 5. 统一公开轨迹与可视化

`TurnTrace` 增加：

- `context_sources`：来源可用状态和版本。
- `context_candidates`：来源、操作、截断查询、结果 ID 和错误。
- `context_reads`：来源、ID、成功或错误。
- `context_uses`：来源、ID、角色、used/dismissed/focus 和原因码。

现有 Episode 与任务处境 Trace 保留兼容；Runtime 将旧事件投影到统一字段。查询最多记录 120 字，不保存标题、正文、隐藏思维或自由文本理由；所有事件由当前 Turn 的 context-scoped recorder 隔离。

Room 实时动画和历史回放使用统一状态：黄色候选、蓝色已读、绿色已采用、灰色虚线已排除、固定底色 Bond；Workspace/Intent 焦点增加外圈，Proposal 始终保留问号或虚线标识。颜色表达状态，形状或图标表达来源。

## 6. 开发与验收

建议按五个本地提交交付：

1. `feat(t2): add unified context source contracts`
2. `feat(t2): add scene and proposal candidates`
3. `feat(t2): add multi-source use reporting`
4. `feat(t2): visualize multi-source activation`
5. `test(t2): add multi-source recall evaluation`

当前实现、13 类虚构案例、离线命令和待授权行为门槛见 [T2.4 评估状态](./evals/t2-multisource-results.md)。`go run ./cmd/eval-multisource` 只做 schema 与规则校验，不访问模型；`-live` 才会把夹具发到当前模型网关。


自动测试覆盖候选正文隔离、read-before-use、Proposal 权限、Intent 时间语义、重复/冲突声明、Turn 隔离、来源独立降级、Agent/Legacy 兼容、Room 状态和旧 Trace 回放。

行为验收使用纯虚构跨来源项目，覆盖无需召回、六种来源的正确角色、Workspace+Intent、Workspace+Intent+Episode、计划不等于发生、Proposal 不等于事实、当前表达优先、歧义询问和来源降级。每个案例重复 3 次，记录回答、来源选择、读取/采用、token、延迟和工具次数；成本只记录，不设门槛。

新夹具先提交给用户审阅，得到明确授权后才能发送到当前 `ops-ai` 网关；不得读取或发送正式记忆。全部改动只做本地 commit，T2 全部结束前不 push。

## 7. 延期范围

T2.4 不实现万能 `recall_context`、Recall Broker、自动搜索所有来源、跨来源相关性分数、动态权重、向量搜索、图扩散、竞争抑制、动态 token 槽位、情绪推断、Proposal 自动采纳或 Agent 默认上线。这些留给 T2.5 注意与竞争及 T2.6 稳定上线。
