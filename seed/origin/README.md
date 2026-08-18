本目录存放 **Origin Record**（永久）：某人在 Deep-Seeing 刚开始认识世界时递给 ta 的自我介绍。

- **Origin Introduction**（注入 system）只在 **first_boot** 出现一次，之后不再每轮塞入
- 不是 Soul；不是 `trust = high`
- Bond 如何生长，由 Episode / 提案 / Dream 决定

约定文件名：`{person_key}.md`，例如 `mudnet.md` 对应 Person `user:mudnet`。

图初始化：

```text
(Self)-[:KNOWS {role_at_origin: "friend / early companion"}]->(Person user:mudnet)
(Self)-[:BOND]->(Person)   # 空常模，由经历与提案生长
```

不要初始化 `TRUSTS=high`，也不要初始化「导师」。主 Agent 不可任意 `patch_bond`。
