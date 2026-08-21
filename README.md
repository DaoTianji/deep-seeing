# Deep-Seeing

> 版本：**v0.8.1**（T1 常模与 Context Graph 更新）

用 [Eino](https://github.com/cloudwego/eino) 的 **ReAct Agent** 做编排壳；记忆机制参考 Claude Code / ascentia：明文文件 + 旁路选型，不用向量库。

机制体系说明见 [`docs/`](./docs/README.md)。长期记忆**目标架构**：[docs/design-ltm.md](./docs/design-ltm.md)；**实现现状**：[docs/memory-ltm.md](./docs/memory-ltm.md)。

## 记忆怎么存（摘要）

- **STM**：Redis 会话（TTL）+ 摘要式 Compaction，失败回退内存/trim（详见 [docs/memory-stm.md](./docs/memory-stm.md)）
- **LTM**：Episode + Bond + Review/Dream 机会 + Mutation Ledger；Origin 仅 first_boot；见 [docs/birth-gate.md](./docs/birth-gate.md)  
- **旁路召回**：Bond → 开放提案 → Episode  
- **能力**：`inspect_runtime` / `list_capabilities`；命令 `/review` `/dream` `/backup`

## 其它

- **Eino ReAct**：模型决定是否再调记忆工具  
- 退出 CLI 或输入 `/review` 触发 Session Review（不自动改写常模）
- Neo4j / Redis 连接见 `.env.example`（`LTM_GRAPH=0` 可强制关图）
## 快速开始

```bash
cp .env.example .env
# 填写 OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL
# 可选：NEO4J_* 启用 L2 图

# 在仓库根目录执行（不要在 cmd/see 子目录里）
go run ./cmd/see
# 打开 http://127.0.0.1:3319
```

`cmd/see` **默认启动谈话室**（内嵌 `embed.FS`，无独立前端构建）。终端 REPL 用：

```bash
go run ./cmd/see --cli
```

桌宠模式可从仓库根目录一步启动 Go Room 与 Tauri：

```bash
./pet
```

详见 [`docs/pet.md`](docs/pet.md)。

右侧记忆区展示：

- Neo4j `Self` / `Person` / `Episode` 与 `BOND` / `KNOWS` / `CALLS` / `ABOUT`
- L1 Episode 正文及 `active` / `archived` / `invalid` 状态
- 待 Dream 决定的 Bond Proposal 与 Mutation Ledger
- 每轮召回、工具调用、记忆写入和错误等结构化轨迹

## 目录

```text
docs/                    设计与现状文档（design-ltm / memory-*）
seed/                    SOUL.md + origin/ 初始相识文稿
cmd/see/                 默认谈话室；--cli 为终端 REPL
cmd/room/                兼容入口（等同默认 see）
internal/app/            运行时装配
internal/room/           HTTP API + embed.FS 页面
internal/runtime/        Prepare → SideQuery → Eino → STM → PostTurn
internal/agent/          Eino ReAct 工厂
internal/soul/           Soul 加载（embed 后备）
internal/origin/         Origin Context 加载
internal/graph/          Neo4j Bond / Episode 指针 / CALLS
internal/memory/         STM + Episode + Proposal + SessionReview
internal/prompt/         拼图式 system（Soul + Origin + Memory）
internal/compaction/     上下文压缩
internal/tools/          episode + bond + propose_bond_update
```
