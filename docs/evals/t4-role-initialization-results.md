# T4.9 角色初始化评估结果

> 日期：2026-08-31。原始 JSONL 位于被忽略的 `data/evals/t4-role-init-structural.jsonl`，不含模型回答或私人资料。

## 当前结论

结构与安全库存门 **PASS：126/126**。这证明 42 个冻结案例、7 个类别、纯虚构资料和关键硬门均可重复执行；尚不代表 Character Architect 的真实模型语义行为已经毕业。

| 类别 | 三次重复 | 通过 |
|---|---:|---:|
| research_plan | 18 | 18 |
| source_governance | 18 | 18 |
| corpus_citation | 18 | 18 |
| period_cutoff | 18 | 18 |
| critic_truth | 18 | 18 |
| private_professional | 18 | 18 |
| budget_recovery | 18 | 18 |

已通过的自动测试还覆盖 InitializationRun 生命周期与恢复、FTS5/Chunk/URL 与哈希去重、observe 不写正式角色、私人模型同意、生成证据硬错误、时期变体共享 Corpus 但不共享 Blueprint、Brave 固定端点和存储层上架硬门。

## 尚未通过的上线门

- 42×3 当前 ops-ai 真实语义评估；
- 阿德勒成熟期公开资料毕业验收；
- 浏览器完整培养与进入角色回放。

## 隔离真实模型烟测

使用临时目录、内存 STM、`LTM_GRAPH=0`、`ROLE_INIT_MODE=observe` 和完全虚构林舟资料完成一次不接触正式记忆的真实 ops-ai 烟测：

- 模型两次没有遵守研究计划 JSON 形状；系统最终使用明确标记的标准计划进入 `awaiting_plan_approval`，没有生成任何人物事实。
- 七份受控资料成功进入独立 Corpus，计划确认后自动运行覆盖、Blueprint 与 Critic。
- 首次 Blueprint 被模型包在额外对象中，旧解析器产生空结构；Critic 将其作为 hard error 拦截，正式 RoleDefinition 仍为 draft。
- 随后增加了 Blueprint 包装层兼容、必填结构空值拒绝和一次格式纠正重试，并由自动测试覆盖。

这次烟测证明失败会安全停下，也暴露并修复了两个模型输出边界问题；它不替代 42×3 最终语义门。

浏览器回放本轮未完成：Codex 浏览器运行时被另一个工作区的符号链接沙箱配置阻断。没有改用未授权的替代自动化来宣称 UI 通过。

因此 `ROLE_INIT_MODE` 继续保持 `off`。上述三项完成前不得把结构门写成最终行为通过，也不得启用生产 agent。
