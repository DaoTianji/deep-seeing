# 短期记忆（STM）

> 状态：已实现（Redis SessionStore + 摘要式 Compaction，失败回退内存 / trim）  
> 相关配置：`.env` 中 `REDIS_*`、`STM_MAX_MESSAGES`、`COMPACT_*`  
> 关联：[长期记忆 LTM](./memory-ltm.md)

## 1. 定位

短期记忆 = **当前会话的工作记忆与上下文管理**：保存对话消息，并在送入主模型前做窗口压缩。它不是世界观真相源；跨会话稳定事实属于 LTM。

| 子机制 | 职责 | 现状 |
|--------|------|------|
| 会话存储 | 按 tenant+session 存消息 | Redis（优先）/ 内存回退 |
| 条数窗口 | 存储层硬上限 | `STM_MAX_MESSAGES`（默认 100） |
| 上下文压缩 | 送模前摘要或 trim | `SummarizingCompactor` |
| Token 估算 | 触发压缩 | `RoughTokenEstimator` |
| 与 LTM 交接 | 耐久事实 | 回合后 Extractor（压缩前抽取尚未做） |

## 2. 与 LTM 的分工

| | STM / 上下文 | LTM |
|--|-------------|-----|
| 内容 | 近轮对话 + 会话摘要 | 沉淀事实、关系等 |
| 寿命 | TTL（默认 24h） | 持久 |
| 压力阀 | 存储窗口 + Compaction | 索引 / 图维护 |
| 检索 | history（可含 `[会话摘要]`） | 旁路 / 工具 |

## 3. 现状：会话存储

- 接口：`memory.SessionStore` — `Get` / `Append` / `Replace`
- 内存：`memory.STM`（`internal/memory/stm.go`）
- Redis：`memory.RedisSTM`（`internal/memory/redis_stm.go`）
  - Key：`ds:stm:{userID}:{agentID}:{sessionID}`
  - Value：消息 JSON 数组；每次写入刷新 TTL
  - 启动 `Ping` 失败 → `cmd/see` 回退内存并打日志
- CLI：`SessionID = "cli"`；启动打印 `STM: redis|memory`

**配置**

| 变量 | 默认 | 含义 |
|------|------|------|
| `REDIS_ADDR` | — | host:port |
| `REDIS_PASSWORD` | — | 密码 |
| `REDIS_DB` | `0` | DB index |
| `REDIS_STM_TTL_SEC` | `86400` | 会话 TTL（秒） |
| `STM_MAX_MESSAGES` | `100` | 存储硬窗口 |

并发：当前为读改写无锁，适合单写 CLI；多写同 session 需加强。

## 4. 现状：上下文压缩（Compaction）

代码：`internal/compaction`；`SummarizingCompactor` 为主，trim 为回退。

### 4.1 配置（送模窗口，与存储窗口分离）

| 变量 | 默认 | 含义 |
|------|------|------|
| `COMPACT_MAX_TOKENS` | `24000` | token 预算 |
| `COMPACT_MAX_MESSAGES` | `32` | 送模条数预算 |
| `COMPACT_MIN_TAIL` | `8` | 保留尾部原文条数 |
| `COMPACT_TRIGGER_RATIO` | `0.8` | 有效阈值 = 预算 × ratio |

### 4.2 何时触发 / 怎么压

1. 单条过长先截断（约 4000 runes）
2. 非 system 消息数或估算 token 超过有效阈值 → 调用小模型摘要头部，保留 `MinTail` 原文
3. Summary 消息：`role=summary`，内容带前缀 `[会话摘要]`
4. 摘要失败 → `ThresholdCompactor` trim
5. 摘要后仍无法压到硬预算且已触地板 → 返回 thrash error

### 4.3 回写

`runtime.StreamTurn`：

1. `Get` history  
2. `MaybeCompact(history)`（**不含本轮 user**）  
3. `Applied && WriteBack` → `Replace` 回 SessionStore  
4. 拼本轮 user → Eino  
5. `Append(user, assistant)`

## 5. 已知局限 / 后续（S3）

- 压缩前对出局片段抽 LTM：未做  
- Tool 结果单独预算 / observation masking：未做  
- Token 估算仍为粗糙公式  
- 无 `/context` 可观测 CLI  

## 6. 变更时请更新本文

- Redis key、TTL、默认窗口  
- Compaction 算法与回写时机  
- 配置项增删  
