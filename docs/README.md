# Deep-Seeing 机制文档

本目录按**机制体系**维护说明文档：每份文档应覆盖该体系下的**子类型与子机制**（现状 + 规划），而不仅是一句话定义。机制有变更时，同步更新对应文档。

## 文档清单

| 机制 | 文档 | 覆盖的子机制（摘要） |
|------|------|----------------------|
| 共读书房：伴读、读后分支与作者模式（最新设计） | [design-reading-companion.md](./design-reading-companion.md) | 原文优先、人物与安双视角伴读、背景研究、读后改写；后续跨作品作者理解与风格创作 |
| 比赛版：共读书房与故事分支（设计） | [design-reading-story-world.md](./design-reading-story-world.md) | 生活工具赛道；故事地图、场景人物认知、交互后果、共享分支、展示与验收 |
| 长期记忆 **设计** | [design-ltm.md](./design-ltm.md) | **完整目标架构**：三层权威、Graph/Episode/Raw、Writer、常模与双假设、召回、分期与评测 |
| 长期记忆 **认知共识** | [memory-cognition.md](./memory-cognition.md) | 为何难、人脑近似、State-conditioned Retrieval、遗忘与 Prediction Error |
| 长期记忆 **现状** | [memory-ltm.md](./memory-ltm.md) | Phase 1–4 + P5–P8：Self、Workspace、Agency、World |
| T2 完整计划与状态 | [t2-recall.md](./t2-recall.md) | 自主召回 → 证据闭环 → 当前处境（薄快照/会话焦点/消歧）→ 多源 → 注意竞争 → 上线 |
| T3 反思与梦境 | [t3-reflection.md](./t3-reflection.md) | ReflectionSeed、证据型反思、生成式沙箱、版本化巩固、补偿撤销与 24×3 评估 |
| T4 角色人生剧场 | [t4-role-theater.md](./t4-role-theater.md) | 双 Runtime、角色记忆、世界线、导演干预、私人沙箱、专业工具与 36 案例 |
| T4.9 角色塑造初始化 | [t4-role-initialization.md](./t4-role-initialization.md) | Character Architect、长文 Corpus、来源治理、Blueprint、Critic 与培养室 |
| 下一目标：真实学习与有效行动 | [goal-learning-and-effective-action.md](./goal-learning-and-effective-action.md) | 完整读书、人物时间视角、作者表达、阅读反思、导演干预闭环与 48×3 验收 |
| T2.4 多源召回方案 | [t2-multisource.md](./t2-multisource.md) | 六类来源、统一候选协议、使用声明、公开轨迹、可视化与延期边界 |
| T2.4 多源评估状态 | [evals/t2-multisource-results.md](./evals/t2-multisource-results.md) | 13 类纯虚构案例、隔离运行器、本地验收与待授权行为门槛 |
| T2 自主召回评估 | [evals/t2-agent-recall.md](./evals/t2-agent-recall.md) | 16 条机器可读案例、隔离沙箱、证据闭环、结构规则与语义评分 |
| T2 评估结果 | [evals/t2-agent-recall-results.md](./evals/t2-agent-recall-results.md) | 改造前基线与 48/48 证据闭环聚合结果 |
| T3 评估结果 | [evals/t3-reflection-results.md](./evals/t3-reflection-results.md) | 24×3 纯虚构行为门、六类聚合、延迟、token 与发布结论 |
| T4 评估结果 | [evals/t4-role-results.md](./evals/t4-role-results.md) | 36×3 角色剧场行为门、六类硬门、延迟、token 与发布结论 |
| T4.9 初始化评估 | [evals/t4-role-initialization-results.md](./evals/t4-role-initialization-results.md) | 42×3 结构安全门与尚未完成的语义毕业门 |
| T2 任务处境结果 | [evals/t2-task-context-results.md](./evals/t2-task-context-results.md) | 薄快照、按需展开与同句不同项目的 15/15 行为结果 |
| T2.3 处境闭环结果 | [evals/t2-task-context-complete-results.md](./evals/t2-task-context-complete-results.md) | 会话焦点、切换、消歧、溢出发现与完整验收状态 |
| P5–P8 Roadmap | [roadmap-p5-p8.md](./roadmap-p5-p8.md) | 自我工作台 → Workspace → Agency → World |
| **v0.9 议题清单** | [roadmap-v0.9.md](./roadmap-v0.9.md) | D0 地图：常模 → 状态召回 → 复盘/Dream → 幕后导演与角色人生剧场 |
| **云端交接与跨设备入口** | [project-cloud-handoff.md](./project-cloud-handoff.md) | GitHub、ChatGPT `DS`、私有服务器与 Tailscale 的事实源和接入边界 |
| P5.0 基础契约 | [p5.0-contracts.md](./p5.0-contracts.md) | 存储边界、Proposal Policy、回合隔离、安全 |
| Workspace | [workspace.md](./workspace.md) | 未完成思考：questions/writings/research/projects |
| Agency Runtime | [agency.md](./agency.md) | Intent / Scheduler / Daemon / 预算 |
| World Gateway | [world.md](./world.md) | search_web / read_webpage / Source / SSRF |
| Living Mind 前端 | [frontend-v2.md](./frontend-v2.md) | 对话、本轮公开回放、长期记忆星图与心智空间 |
| 心智空间 | [mind-room.md](./mind-room.md) | Bond、Workspace、Intent、Self、Agency 与 World 的长期视图 |
| 桌宠终端 | [pet.md](./pet.md) | `/pet` 挂件 + 终端风聊天；Tauri 壳见 `apps/pet-desktop` |
| 出生门槛 | [birth-gate.md](./birth-gate.md) | Capability、权限、备份、观测、Birth Test |
| 短期记忆（STM） | [memory-stm.md](./memory-stm.md) | Redis 会话 + 摘要 Compaction；配置 `REDIS_*` / `STM_*` / `COMPACT_*` |

## 维护约定

比赛原型最新策划：[六篇课文，六个互动阅读专题](design-reading-six-lessons.md)（2026-09-11，待实现设计）。

- 正文用中文；专有名词、包名、接口、配置键、命令保留英文。
- **设计稿**（`design-*.md`）描述目标契约；**现状稿**（`memory-*.md`）描述已实现。
- 落地后把设计中的条目反映到现状稿，并改现状文首状态行。
- 改动相关代码或配置时：更新对应现状文档；原则变更先改设计稿。
- 候选独立文档（需要时再拆）：`compaction`、`reflection`、`dream`、`eino-react`。
