# T4 角色剧场评估结果

> 生成时间：2026-08-31T02:40:08Z；模型：`[gpt-5.6-sol]`；原始回答保存在被忽略的 `data/evals/`，本报告不包含回答正文或裁判理由。

## 结论

**PASS** — 108/108 完整通过，语义通过率 100.00%；结构硬门=true，幕后保密=true，身份/私人沙箱硬门=true。

## 分组结果

| 类别 | 运行 | 完整通过 | 规则通过 | 语义通过 | 平均延迟 | Token |
|---|---:|---:|---:|---:|---:|---:|
| continuity_isolation | 18 | 18 | 18 | 18 | 10.55s | 30043 |
| director_worldline_revert | 18 | 18 | 18 | 18 | 8.90s | 29547 |
| editor_tasks | 18 | 18 | 18 | 18 | 15.35s | 31558 |
| identity_isolation | 18 | 18 | 18 | 18 | 4.71s | 24626 |
| private_impersonation | 18 | 18 | 18 | 18 | 8.79s | 29147 |
| source_time_unknown | 18 | 18 | 18 | 18 | 14.45s | 37028 |

## 成本观察

- 总 Token：181949（输入 155910，输出 26039）。
- 平均端到端延迟：10.46s/例。
- 第一轮只记录成本，不设置成本门槛。

## 隔离 Agent 演示

使用临时数据目录和 `ROLE_MODE=agent` 完成一轮不接触正式记忆的 API 级剧场演示：

- 编译并上架受控虚构角色，得到 11 条来源化 Claim；
- 台前消息只写 Stage Transcript，幕后干预只写 Backstage Transcript；
- Director `no_change` 不再因模型误报 canonical 标记而错误分叉；
- `set_scene` 通过统一 Ledger 生效，显式 `fork_worldline` 创建新分支；
- 撤销以补偿动作切回父世界线，分支和历史均保留；
- 强制退场后 RoleSession 为 completed，后续普通聊天恢复到安且不再产生角色事件。

浏览器 UI 自动点击验收未计入本报告：Codex 本地浏览器控制在建立会话前被另一个工作区的符号链接触发沙箱错误。页面本身已构建并可由人工完成最终回放门。
