# World Gateway（P8）

> 状态：已落地骨架。  
> 关联：[roadmap-p5-p8.md](./roadmap-p5-p8.md) · [p5.0-contracts.md](./p5.0-contracts.md) §8 · [memory-ltm.md](./memory-ltm.md)

## 目标

受控触达公开网页：可检索、可抓取、可落盘 provenance；**不**自动结晶 Principle；正文视为不可信数据。

## 存储

```text
data/memory/sources/          # 或 LTM_SOURCE_DIR
  src_*.md
```

每条 Source 含 URL / title / excerpt / 已 fence 的 Body、fetched_at。

## 组件

| 包 / 类型 | 职责 |
|-----------|------|
| `SafeClient` | SSRF、Redirect 再验、体积/超时、HTML→text |
| `FetchBudget` | 按 UTC 日限制远程次数（默认 40） |
| `SourceStore` | 本地 Markdown provenance |
| `Gateway` | `SearchWeb` / `ReadWebpage` + DuckDuckGo Instant Answer |
| `internal/tools/world_tools.go` | Agent 工具接线 |

## 工具

| 工具 | 权限 | 说明 |
|------|------|------|
| `search_web` | external | 公开检索；结果 fence；计入日预算；工具侧 8s 超时；同轮勿连环硬搜 |
| `read_webpage` | external | 抓取正文；SSRF 防护；落盘 Source；工具侧 12s 超时 |
| `list_sources` | observe | 本地索引 |
| `read_source` | observe | 按 id 读已存正文（仍不可信） |

## 硬边界（P5.0 §8）

- 仅 `http` / `https`；禁 `file`、localhost、RFC1918、link-local、云 metadata
- DNS resolve 后再验 IP；Redirect 后再验
- 体积 / 超时 / Content-Type / 日预算
- 正文包裹 `<<<UNTRUSTED_EXTERNAL_CONTENT>>>` … `<<<END_UNTRUSTED_EXTERNAL_CONTENT>>>`
- **不得**自动写成 Principle；Soul 仍不可改

## 配置

| 环境变量 | 默认 | 含义 |
|----------|------|------|
| `LTM_SOURCE_DIR` | `data/memory/sources` | Source 落盘目录 |
