# Living Mind 前端

> 状态：Frontend 2.0 已替换原谈话室与心智活动页；`/pet` 暂不重构。

## 产品结构

Living Mind 以一次 Turn 为连接对话、召回、注意力与长期记忆的骨架：

| 入口 | 用途 |
|---|---|
| `/` | 日常对话；实时状态默认克制，每条新回答可进入公开轨迹 |
| `/turn/{turn_id}` | 按真实时间回放处境来源、候选、读取、证据决定和注意力变化 |
| `/memory` | 真实持久关系的 2D 星图，以及 Episode、Proposal、Ledger 档案 |
| `/mind` | Bond、Workspace、Intent、Self、Agency、Wake 与 Source 的长期视图 |

长期图谱只展示真实存储关系。本轮激活和注意力属于临时覆盖状态，不会合成 `CONTEXT` 边写入或伪装成 Neo4j 关系。

## 公开边界

- 页面展示工具调用、候选、读取、采用/排除、注意力槽位和运行健康。
- 不展示或推断 Chain-of-Thought。
- `dismissed` 只来自 Agent 的公开声明；未处理候选保持候选状态。
- 历史 Turn 只返回 Trace 已持久化的回答预览，不为了界面额外保存完整记忆正文。

## 数据契约

`POST /api/chat` 继续返回 NDJSON。每行统一包含：

```json
{"type":"context_read","turn_id":"...","seq":4,"turn_offset_ns":12000000,"data":{}}
```

新增：

```text
GET /api/bootstrap
GET /api/turns?limit=60&cursor=<RFC3339Nano>
GET /api/turns/{turn_id}
GET /api/episodes?limit=60&cursor=<episode_id>
```

`/api/bootstrap` 将运行状态、图谱说明、当前注意力和最近 Turn 合并为首屏快照，避免页面启动时产生一组彼此竞态的请求。Turn 使用时间游标；Episode 使用稳定的不透明 ID 游标，档案页按需加载更早内容。

新 Turn 的用户消息、助手消息、流式事件和 `TurnTrace` 使用同一个稳定 `turn_id`。旧 Trace 在只读 API 层获得可重复的 `legacy-<timestamp>` ID，不重写历史 JSONL。

## 开发与发布

前端源码在 `webui/`，构建产物在 `internal/room/web/dist/` 并随 Go 二进制嵌入：

```bash
cd webui
npm install
npm run check
npm test
npm run build
cd ..
go test ./...
```

生产依赖审计使用 `npm audit --omit=dev`。页面按路由拆包，Cytoscape.js 仅在打开 `/memory` 时加载。
