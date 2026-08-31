# Deep-Seeing

> 版本：**v0.10.0 + T4 候选版本**（T1 记忆形成 · T2 自主召回 · T3 反思巩固 · T4 角色剧场）

用 [Eino](https://github.com/cloudwego/eino) 的 **ReAct Agent** 做编排壳；记忆机制参考 Claude Code / ascentia：明文文件 + 旁路选型，不用向量库。

机制体系说明见 [`docs/`](./docs/README.md)。长期记忆**目标架构**：[docs/design-ltm.md](./docs/design-ltm.md)；**实现现状**：[docs/memory-ltm.md](./docs/memory-ltm.md)。

## 记忆怎么存（摘要）

- **STM**：Redis 会话（TTL）+ 摘要式 Compaction，失败回退内存/trim（详见 [docs/memory-stm.md](./docs/memory-stm.md)）
- **LTM**：Episode + Bond + ReflectionSeed + 双层 Dream + 可撤销 Mutation Ledger；Origin 仅 first_boot；见 [docs/t3-reflection.md](./docs/t3-reflection.md)
- **召回模式**：`legacy` 为 Bond → 场景常模 → 开放提案 → Episode 固定旁路；`agent` 为完整 Bond 背景 + Agent 自主使用记忆工具
- **能力**：`inspect_runtime` / `list_capabilities`；命令 `/review` `/dream` `/dream-gen` `/backup`

## 其它

- **Eino ReAct**：模型决定是否再调记忆工具  
- 退出 CLI、输入 `/review` 或 Room 空闲触发 Session Review；非 Legacy 模式只形成 ReflectionSeed
- Neo4j / Redis 连接见 `.env.example`（`LTM_GRAPH=0` 可强制关图）
## 快速开始

```bash
cp .env.example .env
# 填写 OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL
# 可选：NEO4J_* 启用 L2 图
# 可选：RECALL_MODE=agent 试用 T2 自主召回（默认 legacy）
# T3 已默认 agent；可选 REFLECTION_MODE=observe 仅观察，或 legacy 紧急回退
# T4 默认关闭；ROLE_MODE=observe 只记录导演意图，agent 才作用于角色世界

# 在仓库根目录执行（不要在 cmd/see 子目录里）
go run ./cmd/see
# 打开 http://127.0.0.1:3319
```

`cmd/see` 默认启动 Living Mind 前端。生产构建已随 Go `embed.FS` 打包，直接运行 Go 不需要本机 Node。修改前端源码时使用：

```bash
cd webui
npm install
npm test
npm run build
```

终端 REPL 用：

```bash
go run ./cmd/see --cli
```

桌宠模式可从仓库根目录一步启动 Go Room 与 Tauri：

```bash
./pet
```

详见 [`docs/pet.md`](docs/pet.md)。

Living Mind 提供五个相互连接的空间：

- `/`：对话与可收起的实时“此刻”栏
- `/turn/<turn_id>`：处境、召回、证据与注意力的公开回放
- `/memory`：长期记忆星图与结构化档案
- `/mind`：Bond、Reflection/Dream、可撤销变化、Workspace、Intent、Self、Agency 与 World
- `/roles`：Role Library、台前舞台、与安的幕后通道、世界线和导演记录

界面只展示公开工具与结构状态，不展示隐藏思维过程。完整说明见 [docs/frontend-v2.md](docs/frontend-v2.md)。

## 目录

```text
docs/                    设计与现状文档（design-ltm / memory-*）
seed/                    SOUL.md + origin/ 初始相识文稿
cmd/see/                 默认谈话室；--cli 为终端 REPL
cmd/room/                兼容入口（等同默认 see）
internal/app/            运行时装配
webui/                  React + TypeScript 前端源码与测试
internal/room/           HTTP API + embed.FS 生产构建
internal/runtime/        Prepare → Norm/Recall mode → Eino → STM → PostTurn
internal/agent/          Eino ReAct 工厂
internal/soul/           Soul 加载（embed 后备）
internal/origin/         Origin Context 加载
internal/graph/          Neo4j Bond / Episode 指针 / CALLS
internal/memory/         STM + Episode + Proposal + SessionReview
internal/theater/        Role Library + Actor/Director Runtime + Worldline
internal/prompt/         拼图式 system（Soul + Origin + Memory）
internal/compaction/     上下文压缩
internal/tools/          episode + bond + propose_bond_update
```
