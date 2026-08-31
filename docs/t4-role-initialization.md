# T4.9：角色塑造初始化与 Character Architect

> 状态：工程主链、培养室、固定资料包和 42×3 结构安全门已完成；真实模型语义门与阿德勒毕业验收尚未执行。默认 `ROLE_INIT_MODE=off`，不得提前启用 agent。

## 1. 要解决的问题

T4.1–T4.8 已能编译和运行角色，但此前仍需要人手动创建角色、整理材料并点击编译。T4.9 把角色进入剧场前的工作变成独立培养闭环：安制定研究计划，用户只确认计划与最终 Blueprint，中间的公开资料研究、来源审查、覆盖分析、证据编译和独立 Critic 由系统自主完成。

RoleCompiler 仍只负责结构化抽取。安是 Character Architect，负责研究问题、来源判断、冲突/未知和角色塑造；Critic 使用独立上下文复核真实性。

## 2. 状态与模式

`ROLE_INIT_MODE=off|observe|agent`：

- `off` 保留旧手动流程。
- `observe` 生成计划、语料、覆盖、Blueprint 和 Critique，但不修改正式 RoleDefinition。
- `agent` 只有在 Critic 硬门通过后才可写入 `validating`；上架仍必须由用户确认。

流程为：`planning → awaiting_plan_approval → collecting → analyzing → compiling → blueprinting → critiquing → awaiting_final_approval → completed`。暂停、额度不足、失败和取消都会保留 checkpoint；进程重启会把运行中任务恢复为 paused。

## 3. 语料与证据

角色目录增加 `initializations/`、`corpus/documents/`、`corpus/chunks/`、`blueprints/`、`critiques/` 和 `corpus-index.db`。原始文本与 JSON 是事实源，SQLite FTS5 仅为可重建索引。

- 原始文档最大 32 MiB，提取文本最大 16 MiB。
- 支持文本、Markdown、网页正文和带文本层 PDF；扫描 PDF 明确返回 OCR 未支持。
- 按标题、页和段落切分，每个 Chunk 最多约 4,000 Unicode 字符。
- canonical URL 与正文 SHA-256 双重去重。
- 搜索只返回 Chunk 卡；正式证据必须实际读取。
- 直接引语必须关联 Chunk 和页/章节位置。
- 不同人生时期使用独立 RoleDefinition/Blueprint，通过 `variant_of_role_id` 共享 Corpus；世界线仍只表示进剧场后的模拟分支。

## 4. 研究与真实性硬门

`ROLE_SEARCH_PROVIDER=brave|duckduckgo`。存在 `BRAVE_SEARCH_API_KEY` 时使用 Brave，否则降级 DuckDuckGo 并在培养室标记覆盖受限。每个 Run 默认 24 次远程操作，每次用户追加 12 次；World Gateway 的每日 40 次全局上限继续生效。

搜索摘要、未读网页和生成内容没有事实资格。独立确定性校验与模型 Critic 共同阻止：无来源事实、无定位直接引语、时代穿越、actor/director 泄露、后世评价冒充人物自我认识、生成内容循环证明、未读 Chunk、私人角色自动联网和资料提示注入越权。

普通资料稀少或争议会成为 warning；用户接受时必须写理由。硬错误没有绕过接口。旧手动 publish 对处于 InitializationRun 的角色也会被存储层拒绝。

## 5. 私人与权限

私人角色不自动联网；在任何资料或身份信息发送模型前必须明确同意。Character Architect 使用隔离输入，不接触 Actor/Backstage STM、普通 Episode、其他角色或 Bond。专业角色在本轮也不会自动读取 Bond/Workspace，后续只有显式授权的指定内容才能加入语料。

本阶段继续接受未加密落盘：目录 0700、正文与索引 0600。不修改 Redis、Neo4j、PostgreSQL 或服务器网络配置。

## 6. API 与培养室

`/api/role-initializations` 提供创建、查看、计划确认、预算、暂停、恢复、取消、修订和最终确认；文档接口接收长文和 URL，Corpus 接口列出文档、搜索卡片并读取 Chunk。

Role Library 的培养室显示研究步骤、提供者与预算、来源 audience、覆盖矩阵、冲突/未知、Blueprint 证据数和 Critic 结果。公开事件持久化在 Run，可实时轮询并历史回放；不保存模型隐藏推理。

## 7. 评估与上线门

纯虚构林舟资料包覆盖两个时期、冲突传记、伪造引语、后世评价、缺失年份、长文关键证据、重复资料、提示注入和混合 audience。机器套件为 `evals/t4/role_initialization_cases.json`，7 类 42 案例。

```sh
go run ./cmd/eval-role-init -repeat 3 -out data/evals/t4-role-init-structural.jsonl
```

当前 126/126 是结构与安全库存门，不等同于真实模型语义毕业。启用顺序仍是 `off → observe → agent`；必须先完成 42×3 真实模型语义评估、阿德勒成熟期毕业验收，以及浏览器端“提出人物—确认计划—观察研究—审查 Blueprint—确认上架—进入角色”回放。
