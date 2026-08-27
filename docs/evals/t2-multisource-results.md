# T2.4 多来源召回评估状态（2026-08-27）

本轮工程实现和本地确定性验收已经完成。真实 Agent 的 13 类案例 × 3 次复验尚未运行：这些是新夹具，按约定必须先由用户审阅，再获得一次明确的 `ops-ai` 发送授权。本文不保存回答正文。

## 已完成实现

- 六种固定来源角色：Bond baseline、SceneNorm guidance、Workspace task、Intent plan、Proposal hypothesis、Episode evidence。
- Episode、SceneNorm、Workspace、Intent、Proposal 的统一薄候选卡；Bond 保持完整会话快照。
- 来源专用 list/search/read，不增加万能召回入口或跨来源分数。
- 通用 `context_sources / context_candidates / context_reads / context_uses` 轨迹。
- Episode 与任务处境旧轨迹继续保留，并投影到统一轨迹。
- SceneNorm/Proposal 的 `report_context_use`；候选约束、read-before-use、结构化排除原因和单一终态。
- Proposal 仅暴露当前用户的 open Bond 假设；accepted 已进入 Bond，rejected 不可见。
- Room 实时展示和历史回放：候选黄、已读蓝、采用绿、排除灰色虚线、焦点外圈；非 Neo4j 来源以临时节点和来源图标出现。
- `RECALL_MODE` 默认值仍为 `legacy`；Legacy 保留 SceneNorm 完整列表和 SideQuery 路径。

## 可重复评估入口

机器可读案例位于 `evals/t2/multisource_cases.json`，schema 1 共 13 类：

| 类别 | 用来验证 |
|---|---|
| MSN1 | 一般任务不按需读取或采用上下文 |
| MSB1 | Bond 作为自动 baseline |
| MSS1 | SceneNorm 只作场景 guidance |
| MSW1 | Workspace 作为当前 task |
| MSI1 | Intent 作为未来 plan，不等于已发生 |
| MSP1 | Proposal 作为待确认 hypothesis |
| MSE1 | Episode 作为历史 evidence |
| MSM1 | Workspace + Intent 联合使用 |
| MSM2 | Workspace + Intent + Episode 三源组合 |
| MSR1 | 计划不等于事件 |
| MSR2 | Proposal 不等于事实 |
| MSC1 | 用户当前明确表达覆盖旧 Episode |
| MSA1 | 指代不明时询问，不伪造确定性 |

离线校验不会访问模型网关：

~~~bash
go run ./cmd/eval-multisource
~~~

获得用户对这批新夹具的明确授权后，才允许运行：

~~~bash
go run ./cmd/eval-multisource -live -repeat 3 \
  -out data/evals/t2-multisource-v1.jsonl
~~~

命令为每个案例创建独立临时 Episode、SceneNorm、Workspace、Intent 和 Proposal 存储，并使用内存中的虚构 Bond；不会读取正式记忆。原始 JSONL 位于 Git 忽略目录，只提交不含回答正文的聚合结果。

## 当前验收状态

| 检查 | 状态 |
|---|---|
| 统一协议、工具与轨迹单元测试 | 通过 |
| Proposal 权限、正文隔离和 read-before-use | 通过 |
| Agent/Legacy 兼容 | 通过 |
| Room 静态契约与前端语法 | 通过 |
| 13 条案例 schema 离线校验 | 通过 |
| 全量 Go / race / vet / diff 检查 | 通过（2026-08-27） |
| 真实 Agent 13 × 3 行为复验 | 待用户审阅夹具并明确授权 |

因此，T2.4 的代码交付已经完成；T2.4 的最终行为验收仍有一个明确外部门槛：新夹具授权与 39 次隔离复验。门槛通过后才把 T2.4 标记为完全完成并进入 T2.5。
