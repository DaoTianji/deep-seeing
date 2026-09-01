# Deep-Seeing 云端交接与跨设备入口

> 更新时间：2026-09-01  
> 用途：让新的电脑、ChatGPT 云端任务和未来的开发会话快速恢复项目上下文。本文只保存非敏感信息。

## 1. 项目是什么

Deep-Seeing 是安的长期认知与角色系统。它不是单独的聊天机器人，而是一套围绕身份连续性、长期记忆、自主召回、反思巩固和角色人生剧场建立的 Agent Runtime。

## 2. 当前阶段

| 阶段 | 能力 | 状态 |
| --- | --- | --- |
| T1 | 形成并维护长期认识，区分事实、边界、互动方式与策略 | 已完成 |
| T2 | 在需要时自主搜索、读取和采用记忆，并维护有限注意焦点 | 已完成，生产使用 `agent` |
| T3 | 形成反思问题，跨经历核验证据，版本化修订并可补偿撤销 | 已完成，生产使用 `agent` |
| T4.1–T4.8 | Actor/Director 双 Runtime、角色记忆、世界线、幕后干预与退场 | 工程与行为验收完成，生产仍在观察阶段 |
| T4.9 | 安自主规划人物研究、管理语料、生成 Blueprint 并由独立 Critic 审查 | 主链完成；阿德勒 Blueprint v10 已通过 Critic，等待最终人工确认 |

详细契约以 [roadmap-v0.9.md](./roadmap-v0.9.md)、[t4-role-theater.md](./t4-role-theater.md) 和 [t4-role-initialization.md](./t4-role-initialization.md) 为准。

## 3. 四个事实源

| 内容 | 事实源 | 跨设备方式 |
| --- | --- | --- |
| 代码、测试、设计文档 | GitHub 仓库 `DaoTianji/deep-seeing` | 新电脑克隆仓库并切换到当前开发分支 |
| 项目背景、讨论和云端工作任务 | ChatGPT 项目 `DS` | 使用同一 ChatGPT 账号从网页、桌面或移动端打开 |
| 安的正式记忆、角色语料与运行状态 | 私有服务器 | 不上传 Git 或 ChatGPT；由服务器数据与备份保存 |
| Living Mind 与角色培养室 | `https://deep-seeing.tail165a8d.ts.net/` | 设备安装 Tailscale，并登录同一 Tailnet 后访问 |

当前开发分支为 `codex/t4-role-initialization`。合并前，其他电脑应显式切换到该分支，不要默认使用旧的 `main`。

## 4. 新电脑接入

1. 安装 Tailscale，登录与服务器相同的 Tailnet，打开上面的私人 HTTPS 地址。
2. 登录同一 ChatGPT 账号，在项目列表中打开 `DS`，从云端总览任务继续讨论和规划。
3. 如需开发，克隆 GitHub 仓库并切换到 `codex/t4-role-initialization`。
4. 单独配置本机开发环境和 `.env`；密钥、正式记忆与服务器数据不会通过 GitHub 或 ChatGPT 同步。
5. 开始新开发任务前，先阅读本文和 `docs/README.md`，再根据当前阶段进入对应契约文档。

## 5. 当前最近状态

- 服务器发布版本：`19b08ee`。
- 阿德勒角色初始化运行已生成 Blueprint v10。
- 独立 Critic 结果：0 个硬错误、0 个警告。
- 当前停在最终人工确认门，不允许系统替用户自动上架。
- 服务器自主公网研究仍受出站网络限制；阿德勒本轮使用已审查并导入的公开资料完成证据闭环。

## 6. 不应同步的内容

- `.env`、API Key、数据库密码和 Tailscale 身份信息。
- `data/` 中的正式 Episode、Transcript、Reflection、角色语料正文与原始评估输出。
- Redis、Neo4j 和服务器文件数据的直接副本。
- 任何未经用户同意的私人角色资料。

## 7. 下一步

1. 在角色培养室人工检查并确认阿德勒 Blueprint。
2. 完成一次浏览器端“上架—进入角色—幕后交流—干预—退场”完整回放。
3. 根据回放结果决定何时将角色相关模式从观察切换为正式 `agent`。
4. 将 `codex/t4-role-initialization` 合并回 `main`，让其他电脑默认取得同一稳定版本。
