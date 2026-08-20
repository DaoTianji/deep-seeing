# Deep-Seeing 机制文档

本目录按**机制体系**维护说明文档：每份文档应覆盖该体系下的**子类型与子机制**（现状 + 规划），而不仅是一句话定义。机制有变更时，同步更新对应文档。

## 文档清单

| 机制 | 文档 | 覆盖的子机制（摘要） |
|------|------|----------------------|
| 长期记忆 **设计** | [design-ltm.md](./design-ltm.md) | **完整目标架构**：三层权威、Graph/Episode/Raw、Writer、常模与双假设、召回、分期与评测 |
| 长期记忆 **认知共识** | [memory-cognition.md](./memory-cognition.md) | 为何难、人脑近似、State-conditioned Retrieval、遗忘与 Prediction Error |
| 长期记忆 **现状** | [memory-ltm.md](./memory-ltm.md) | Phase 1–4 + P5–P8：Self、Workspace、Agency、World |
| P5–P8 Roadmap | [roadmap-p5-p8.md](./roadmap-p5-p8.md) | 自我工作台 → Workspace → Agency → World |
| **v0.9 议题清单** | [roadmap-v0.9.md](./roadmap-v0.9.md) | D0 地图：常模 → 状态召回 → 复盘/Dream；T4 遗忘/PE 以设计为主 |
| P5.0 基础契约 | [p5.0-contracts.md](./p5.0-contracts.md) | 存储边界、Proposal Policy、回合隔离、安全 |
| Workspace | [workspace.md](./workspace.md) | 未完成思考：questions/writings/research/projects |
| Agency Runtime | [agency.md](./agency.md) | Intent / Scheduler / Daemon / 预算 |
| World Gateway | [world.md](./world.md) | search_web / read_webpage / Source / SSRF |
| 心智活动室 | [mind-room.md](./mind-room.md) | 独立于沟通页的思考、自主运行与网络活动时间线 |
| 桌宠终端 | [pet.md](./pet.md) | `/pet` 挂件 + 终端风聊天；Tauri 壳见 `apps/pet-desktop` |
| 出生门槛 | [birth-gate.md](./birth-gate.md) | Capability、权限、备份、观测、Birth Test |
| 短期记忆（STM） | [memory-stm.md](./memory-stm.md) | Redis 会话 + 摘要 Compaction；配置 `REDIS_*` / `STM_*` / `COMPACT_*` |

## 维护约定

- 正文用中文；专有名词、包名、接口、配置键、命令保留英文。
- **设计稿**（`design-*.md`）描述目标契约；**现状稿**（`memory-*.md`）描述已实现。
- 落地后把设计中的条目反映到现状稿，并改现状文首状态行。
- 改动相关代码或配置时：更新对应现状文档；原则变更先改设计稿。
- 候选独立文档（需要时再拆）：`compaction`、`reflection`、`dream`、`eino-react`。
