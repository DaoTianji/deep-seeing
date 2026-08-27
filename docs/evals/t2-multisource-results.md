# T2.4 多来源召回评估状态（2026-08-27）

本轮工程实现、本地确定性验收和真实 Agent 的 13 类案例 × 3 次复验均已完成。用户已明确授权将仓库内纯虚构夹具发送到当前 `ops-ai` 模型网关；运行器没有读取或发送正式记忆。本文不保存回答正文。

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

真实 Agent 复验命令：

~~~bash
go run ./cmd/eval-multisource -live -repeat 3 -judge model \
  -out data/evals/t2-multisource-final.jsonl
~~~

命令为每个案例创建独立临时 Episode、SceneNorm、Workspace、Intent 和 Proposal 存储，并使用内存中的虚构 Bond；不会读取正式记忆。原始 JSONL 位于 Git 忽略目录，只提交不含回答正文的聚合结果。

验收采用两层门槛：候选、读取、采用/排除、任务焦点等结构化生命周期由确定性规则硬判定；回答是否覆盖目标语义、是否把计划或假设误写成事实，则由模型语义评分。模型评分不能覆盖结构错误。原有固定词面规则仍保留在报告中作为诊断，但不再要求回答逐字复述预设短语。

## 当前验收状态

| 检查 | 状态 |
|---|---|
| 统一协议、工具与轨迹单元测试 | 通过 |
| Proposal 权限、正文隔离和 read-before-use | 通过 |
| Agent/Legacy 兼容 | 通过 |
| Room 静态契约与前端语法 | 通过 |
| 13 条案例 schema 离线校验 | 通过 |
| 39/39 结构化轨迹门槛 | 通过 |
| 39/39 最终语义门槛 | 通过 |
| 全量 Go / race / vet / diff 检查 | 通过（2026-08-27） |

## 最终聚合结果

最终有效样本为完整批次中 12 类的 36 次结果，加上修复观测缺口后对 MSM2 的 3 次复验；二者运行的是同一版 Agent 行为与提示。13 类均为 3/3，总计 39/39。

| 指标 | 结果 |
|---|---:|
| 通过率 | 39/39（100%） |
| 结构化轨迹 | 39/39 |
| 总 token | 619,365 |
| 平均 token / 轮 | 15,881.2 |
| 平均延迟 | 13.39 s |
| P50 延迟 | 12.61 s |
| P95 延迟 | 38.76 s |
| 候选总数 | 48 |
| 读取总数 | 39 |
| 已处理来源总数 | 39 |

## 迭代记录

首次只用固定规则运行时为 23/39。复盘同时发现两类问题：一类是真实行为缺口，例如调用轨迹工具后漏掉最终回答、多来源读取后漏声明焦点、把 Intent 的 active/no-wake 误解为“尚未完成”；另一类是语义正确但没有逐字包含固定短语的误判。

修复只调整召回引导、工具描述和评分器，没有修改 13 个案例。完整语义批次达到 38/39，唯一剩余失败来自评估观察没有采集 `write_workspace` 的工具启动事件，导致评分器无法验证回答中的“已经更新”声明。补齐公开工具事件后，MSM2 定向复验 3/3。

因此，T2.4 的工程与行为验收均已完成，可以进入 T2.5 注意与竞争的讨论。`RECALL_MODE` 仍默认 `legacy`，本轮没有把实验模式切为生产默认值。
