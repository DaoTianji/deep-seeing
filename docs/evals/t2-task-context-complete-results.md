# T2.3 任务处境闭环评估结果（2026-08-27）

本报告只保存聚合结果和验收状态，不保存回答正文。案例与 Workspace/Intent 内容均为仓库内虚构数据；真实运行时每个案例使用独立临时存储，不读取正式记忆。

## 实现范围

- 每轮 active Workspace/Intent 薄快照。
- 会话内临时焦点，当前焦点优先进入下一轮快照。
- `list_workspace` / `list_intents` 公开发现轨迹。
- `read_workspace` / `read_intent` 正文核对轨迹。
- `report_context_focus` 的 continue、switch、check、compare、clarify、clear。
- 新焦点必须先读取；歧义不覆盖焦点；当前用户表达优先。
- Room 实时事件与历史 Trace。
- Agent/Legacy 隔离；会话焦点不进入 LTM。

## 本地确定性验收

| 检查 | 结果 |
|---|---|
| `go test ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test -race ./internal/observe ./internal/runtime ./internal/tools` | 通过 |
| `node --check internal/room/web/app.js` | 通过 |
| `go run ./cmd/eval-task-context` | schema 2，9 案例、12 Turn，通过 |
| `git diff --check` | 通过 |

自动测试进一步证明：

- 焦点存储只存在于会话内存。
- 工具选中焦点后，下一 Turn 快照优先携带它。
- 已存在焦点可延续，新焦点必须先读取。
- 显式清除后下一轮不再有焦点。
- 超出前四张卡片的项目可通过 list 发现。
- Intent 列表、公开轨迹和 Trace 不包含正文。
- Legacy 不暴露焦点工具。

## 真实 Agent 行为验收

已运行 9 类案例 × 3 次，共 36 个连续 Turn，检查：

- 普通任务不滥用处境工具。
- 明确 Workspace/Intent 正确读取并声明。
- 会话连续、显式切换、联合处境和清除焦点。
- 快照溢出后主动 list。
- 歧义时声明 `clarify / ambiguous` 并确实询问。
- 不搜索无关 Episode。

经用户明确授权，仓库内纯虚构夹具已发送到当前配置的 `ops-ai` 模型网关；未读取或发送正式 Episode。

| 指标 | 最终结果 |
|---|---:|
| 模型 | `gpt-5.6-sol` |
| 通过案例 | 27 / 27（100%） |
| 连续 Turn | 36 |
| 总 token | 361,046 |
| 平均 token / 运行 | 13,372.1 |
| list/read 公开轨迹 | 33 |
| focus 公开声明 | 33 |
| Episode 搜索 | 0 |
| Episode/Workspace/Intent 持久化变化 | 0 |

每一类案例均为 3/3：普通任务、明确 Workspace、会话连续、显式切换、歧义询问、快照溢出发现、明确 Intent、Workspace/Intent 联合处境和清除焦点全部通过。

评估迭代保留了固定案例，没有修改案例迁就结果：初始烟测为 7/9，暴露了同秒时间精度和 action 语义问题；修正后第一次完整重复为 26/27，其中一次联合处境只承诺读取但没有实际调用工具。补强工具执行约束后，最终 3 次完整重复达到 27/27。最后一次运行额外比较每个案例前后的 Episode、Workspace、Intent JSON 快照，确认没有任何长期写入。

结论：T2.3 的任务事实快照、按需展开、会话焦点、切换、消歧和清除已经形成可重复验证的闭环，满足进入 T2.4 的完成门槛。
