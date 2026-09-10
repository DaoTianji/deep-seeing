# LongMemEval-S 检索与全文对照协议

起点：冻结基线 `1de6438`。分支：`codex/eval-longmemeval-controls`。
原基线分支、原始答案与评分不变。仅扩展测评适配器，不修改生产工具。

## 在运行前固定的两组对照

### BM25 Agent

- 与 native 相同的原文会话 Episode、Agent、系统提示、工具名称/描述/参数、候选格式、160字符开头摘要、读取和采用工具。
- 仅替换 search_episodes 的匹配与排序：按会话全文构建 BM25，k1=1.2、b=0.75。
- Unicode 字母/数字分词、小写；没有词干化、停用词表、向量、查询扩展、片段优化、重排或答案标签。
- 查询词去重；相同得分按最新会话优先；只保留正得分，空查询沿用最新优先。默认 limit=8，Agent 可按原接口指定 limit。
- 仍需显式读取才能获得正文，仍需满足原证据声明硬门；原有16步上限不变。
- 与 native 的差异主要用于检验检索替换；单次采样的随机性、运行时网关负载仍不是受控常量。

### Full context

- 每题全部原文会话按历史时间排序，保留用户/助手角色和时间；历史后追加问题与问题时间。
- 不截断、筛选、摘要或使用标准答案。记录输入字节数、SHA-256 和推理后的会话来源映射。
- 同一中性 system persona、同一答题模型 `gpt-5.6-sol`；一次模型请求、输出上限2048，与原 no-memory 一致。
- 没有搜索/读取工具，因此“不调用工具”不表示没看到历史；单列 context exposure，不能把它记为工具召回或证据采用。
- 这组用于检验全部资料提供后系统能够达到的表现，不是与 Agent 计算量相同的消融，也不是 oracle（仍包含全部无关会话）。
- HTTP上限180秒、每次题目执行上限4分钟；若网关拒绝上下文长度，明确记录失败，不静默裁剪。

## 数据、模型、评分与预算

- 同一公开 cleaned LongMemEval-S，500题，SHA-256：`d6f21ea9d60a0d56f34a05b609c79c88a451d2ae03597821ea3d5a9678c3a442`。
- 两组均运行完整500题一次，4个并发题目；模型固定，不因分数更换。先用前两题做长度/接口技术验证，不据此调参。
- 全文组预计数千万输入tokens，记录实际输入/输出、延迟和失败；不接入私人Episode、Soul、Bond、Redis或Neo4j。
- 沿用原协议两套裁判：gpt-4o网关别名与gpt-5.6-sol，官方提示、temperature=0、max_tokens=10。
- 裁判质量限制与原始错误见 [裁判审计](longmemeval-judge-audit.md)。有效判定不重跑；空/非法回执可另存恢复记录。两套分数并列，不挑高分。
- 运行错误计入500题分母；仅网络/限流/超时按原逻辑有限重试，错误答案和步数耗尽不重试。
- 全部原始产物写入忽略的 `data/evals/longmemeval-controls/`，只提交无答案正文的汇总。

## 复现

```sh
go test ./cmd/eval-longmemeval
go run ./cmd/eval-longmemeval -mode bm25 -workers 4 -out data/evals/longmemeval-controls/bm25.jsonl
go run ./cmd/eval-longmemeval -mode full-context -workers 4 -out data/evals/longmemeval-controls/full-context.jsonl
python3 scripts/score-longmemeval.py --data data/evals/longmemeval-baseline/longmemeval_s_cleaned.mirror.json --hypotheses data/evals/longmemeval-controls/bm25.jsonl --judge gpt-4o
# 对full-context重复；两组再分别使用 --judge gpt-5.6-sol。
```

判读顺序：先审查错误/完整性，再比较证据覆盖与最终正确率，最后比较成本和延迟。
若BM25明显改善，说明原有搜索是重要瓶颈；若全文明显高于BM25，仍需区分候选覆盖、读取选择及摘要等因素。
不把比较结果扩张为T1提炼、T3反思或角色学习的整体结论。

## 额度中断与恢复（2026-09-08）

首次运行遇到共享ops-ai网关额度不足（403）。BM25保留171个成功结果和4个步数失败，
325个额度失败；全文组保留401个成功结果，99个额度失败。未对这些不完整结果评分。
用户确认额度已恢复后，仅补额度失败的题；语义错误与步数失败不重跑。

原始文件不改动。离线恢复工具生成 `bm25-resumed.jsonl` 和 `full-context-resumed.jsonl`，
完整保留原有非额度结果，同时保存原文件SHA-256及恢复题号；新检查点沿用相同配置。
额外增加额度熔断：检测到额度不足时停止派发新题，等待已经在途的请求结束，退出码3。
该修正只影响调度与恢复，不改变BM25、提示、证据工具或模型参数。

```sh
python3 scripts/prepare-longmemeval-recovery.py --source data/evals/longmemeval-controls/bm25.jsonl --target data/evals/longmemeval-controls/bm25-resumed.jsonl
# 相同方式准备full-context；目标必须是尚不存在的新文件。
go run ./cmd/eval-longmemeval -mode bm25 -workers 4 -out data/evals/longmemeval-controls/bm25-resumed.jsonl
go run ./cmd/eval-longmemeval -mode full-context -workers 4 -out data/evals/longmemeval-controls/full-context-resumed.jsonl
# 对恢复完成的文件分别运行两套裁判，再生成报告：
python3 scripts/report-longmemeval-controls.py --bm25-hypotheses data/evals/longmemeval-controls/bm25-resumed.jsonl --full-context-hypotheses data/evals/longmemeval-controls/full-context-resumed.jsonl
```
