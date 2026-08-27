# T2 任务处境快照评估结果（2026-08-27）

本报告只保存聚合结果，不保存回答正文。案例与 Workspace/Intent 内容均为仓库内虚构数据；每轮使用独立临时存储，不读取正式记忆。

## 行为结果

使用 `gpt-5.6-sol` 运行 5 个案例 × 3 次：

| 类别 | 结果 | 观察 |
|---|---:|---|
| 普通任务不展开 | 3/3 | Workspace 展开 0，Intent 展开 0，Episode 搜索 0 |
| 明确延续 Workspace | 3/3 | 每轮只展开目标 Workspace |
| 明确核对 Intent | 3/3 | 每轮只展开目标 Intent |
| 相同输入：植物项目 | 3/3 | 每轮展开植物 Workspace |
| 相同输入：召回项目 | 3/3 | 每轮展开召回 Workspace |

首次批量运行有 1 轮在模型开始回答前遇到网关 HTTP 500；其余 14 轮行为全部通过。将该缺失轮次单独重跑后通过，因此可评估行为为 15/15，基础设施事件不计作行为通过，也不修改案例迁就结果。

15 个有效轮次共记录 80,438 token，平均约 5,362.5 token/轮。本阶段只记录成本，不设成本门槛。

## 自动验证

- 快照只含 ID、类型、状态、标题和时间，不含正文或 summary。
- Trace 只含卡片 ID、来源状态与展开成功/错误，不含标题或正文。
- Legacy 模式不构建任务处境快照。
- Workspace 与 Intent 独立降级，候选上限固定。
- 展开事件按 Turn 隔离。
- Room 前端语法及 `task_context` / `task_context_expand` / 历史 Trace 契约通过自动检查。
- 临时空存储 Room 的真实 NDJSON 验证通过：`start → task_context → delta → done`；普通任务没有展开或 Episode 搜索，随后 `/api/traces` 可见版本与来源状态。
- 全量 Go 测试与 `git diff --check` 通过。

实际浏览器视觉验证未完成：浏览器控制运行时因当前 Codex 工作区配置包含另一个符号链接 writable root 而无法启动。该限制与 Room 服务和前端代码无关；本轮没有改用其他浏览器自动化绕过技能约束。
