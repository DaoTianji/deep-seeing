# Workspace（未完成思考）

> 状态：P6 已落地。  
> 关联：[roadmap-p5-p8.md](./roadmap-p5-p8.md) · [memory-ltm.md](./memory-ltm.md) · [p5.0-contracts.md](./p5.0-contracts.md)

## 定位

| 层 | 回答 |
|----|------|
| Episode / SelfArtifact | 我经历过什么 / 我相信什么 |
| **Workspace** | **我正在想什么**（可续写、可暂停） |
| Intent（P7） | 我以后想做什么 |

Workspace **不是** Self 结晶，也 **不会** 自动升 Principle。

## 布局

```text
data/memory/workspace/          # 或 LTM_WORKSPACE_DIR
  questions/wq_*.md
  writings/ww_*.md
  research/wr_*.md
  projects/wp_*.md
```

文档含：title / summary / body、status、revisions、episode_ids、related_self_ids。

状态：`open | in_progress | paused | done | archived`

## 工具

- `list_workspace` / `read_workspace`
- `write_workspace`（create 或 update；update 追加 revision）
- `link_workspace_episode`

实现：`internal/workspace` · 接线：`internal/tools/workspace_tools.go`
