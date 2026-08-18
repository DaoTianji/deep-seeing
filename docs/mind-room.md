# 心智活动室

> 状态：第一版只读可观测界面已落地。  
> 入口：`http://127.0.0.1:8787/mind`

## 定位

沟通界面与心智活动界面分开：

- `/`：此刻与 ta 沟通
- `/mind`：回看 ta 在什么时间留下了哪些可观察行为与产物
- `/pet`：桌宠式挂件 + 终端风聊天，只看回复（见 [`pet.md`](pet.md)）

心智活动室不展示 Chain-of-Thought。它只展示结构化外部轨迹、持久化对象、修订记录与来源证据。

## 四个视图

| 视图 | 内容 |
|------|------|
| 活动 | 合并 Turn Trace、Workspace/Self 修订、Intent/Wake、Source、Proposal、Mutation 的时间线 |
| 记忆 | Context Graph 与 Episode；保留原谈话室右栏的记忆深潜能力 |
| 思考 | Workspace 与 SelfArtifact；正文、状态、修订和 Episode 证据 |
| 自主 | 全状态 Intent 与 Wake Job；Scheduler 状态、日预算、计划/实际时间、`auto:<intent_id>:<attempt>`、决策和结果 |
| 外界 | `search_web` / `read_webpage` 保存的 Source；查询词、URL、抓取时间与 fenced 正文 |

## 只读 API

```text
GET /api/self
GET /api/self/{id}
GET /api/workspace
GET /api/workspace/{id}
GET /api/intents
GET /api/wakes
GET /api/agency
GET /api/sources
GET /api/source/{id}
```

已有 `/api/traces`、`/api/proposals`、`/api/mutations` 也参与统一时间线。

## 边界

- “思考”是 Workspace、SelfArtifact、Proposal 等**可观察产物**，不是隐藏推理文本
- 网络正文继续显示 `UNTRUSTED_EXTERNAL_CONTENT` fence
- Source 列表不返回全文；只有选择详情时才读取 Body
- 页面当前只读，不从可观测后台直接修改 Self、Intent 或 Source
