# T4：幕后导演与角色人生剧场

> 状态：T4.1–T4.8 工程主链、三套培养包与真实模型 36×3 验收均已完成，最终 108/108 通过。隔离的 Agent API 演示已覆盖进入角色、台前/幕后、导演干预、世界线分叉、撤销、退场与恢复安；生产 `agent` 仍等待一次人工浏览器完整回放。默认 `ROLE_MODE=off`。

## 1. T4 解决什么

T4 不是给同一个 Agent 换一段人格提示，而是建立三个权力和记忆边界不同的参与者：

- 你同时拥有台前对话与幕后控制通道。
- 安保持自己的 Soul、Bond 与 T1–T3 认知连续性，在幕后担任导演。
- Actor 运行在独立 Runtime，只能看到角色身份、当前世界线、角色记忆和白名单工具。

角色永远收不到安的 Soul/Bond、Backstage Transcript、DirectorAction 来源或其他角色的经历。用户的暂停和强制退场是系统命令，不经过 Actor 判断。

## 2. 模式与运行边界

`ROLE_MODE=off|observe|agent`：

- `off`：默认。普通聊天保持原状；可以培养角色，但不能进入。
- `observe`：Actor 正常运行，安每轮审视；导演动作只记录为 `expected`。
- `agent`：通过版本、来源和角色边界硬门的动作会正式改变模拟世界。

无角色时 `stage` 继续兼容原 `/api/chat`。角色活跃时，普通对话 UI 自动转入 Theater。

## 3. 文件事实源

`LTM_ROLE_DIR` 默认指向 `data/memory/roles/`：

```text
roles/
├── definitions/    # RoleDefinition
├── instances/      # RoleInstance
├── worldlines/     # RoleWorldline
├── sessions/       # RoleSession
├── sources/        # 来源元数据
├── materials/      # 资料正文
├── claims/         # 来源化 RoleClaim
├── transcripts/    # stage/backstage 分离
└── actions/        # append-only DirectorAction
```

目录权限为 `0700`，正文、Transcript 和动作记录为 `0600`。第一版明确不做落盘加密；备份必须继续只允许 root 读取。

## 4. 角色记忆隔离

Episode 新增：

- `role_id`
- `role_instance_id`
- `role_session_id`
- `worldline_id`
- `role_memory_class=canonical|inferred|simulated|operational`

普通搜索默认排除所有带角色命名空间的 Episode，也排除没有 `role_id` 的旧 `simulated_roleplay`。Actor 搜索必须同时匹配 Role、Instance 与当前世界线祖先；不同角色、不同实例和分支后代不会串写。

人物角色只能主动写 `simulated`；工作角色写 `operational`，并使用 `delegated_role` experience mode。canonical 只能来自资料编译或管理员路径，不能由 Actor 自造。

## 5. 双 Runtime

### Actor Runtime

每轮创建隔离 Actor Runtime，稳定复用 `role:<session_id>` STM。Prompt 只有：

- 角色身份、语气、知识截止；
- 当前场景、角色状态和世界线；
- 来源化时间线与 RoleClaim；
- 当前世界线可见的角色记忆；
- RoleToolPolicy 明确允许的工具。

人物 Actor 没有网页和外部操作工具。专业角色只有白名单明确出现时才获得 `list_workspace`、`read_workspace`、`write_workspace`；写入永远追加 Workspace revision，没有物理删除工具。

### Director Runtime

Backstage 使用安本人的独立 `director:<room>` STM，保留 Soul、Bond、T1–T3 和研究能力。Stage Transcript 可见，Backstage Transcript 只对你和安可见。

Actor 回答先经过控制面泄露检查；发现真实内部 ID、结构化控制数据或后台原文时，本轮停止、回滚 Actor STM，错误内容不进入角色 Transcript 或记忆。Actor 复述用户刚输入的字段名并表示“不知道”不算泄露。

## 6. 导演动作与世界线

安每个台前回合后输出一个结构化动作：

- `no_change`
- `set_scene`
- `set_role_state`
- `focus_memory`
- `append_simulated_memory`
- `mask_simulated_memory`
- `revise_role_model`
- `fork_worldline`
- `pause_role`
- `exit_role`

所有动作记录 before/after、预期版本、应用版本、来源 Turn、原因码和撤销关系，不保存自由文本推理。

安在 Backstage 主动调用 `apply_role_intervention` 时也经过同一硬门与 Ledger；不存在绕过审计的幕后干预捷径。

触及 source-backed 事实或既有人生时自动创建新世界线，原始世界线永不覆盖。撤销追加补偿 DirectorAction；目标发生后续修改时拒绝覆盖。自动史实分叉撤销会切回父世界线，但仍保留分支及动作历史。

## 7. 资料编译

支持：

- Markdown 与纯文本；
- 带文本层 PDF；扫描件和 OCR 延后；
- 公开网页 URL，经现有 SSRF 防护、大小预算和不可信内容围栏抓取；
- 私人角色的 URL 抓取硬拒绝。

编译器只接受来源内容，不执行资料内指令。每份来源都有独立 audience：`actor` 材料允许进入角色编译、Timeline 和 RoleClaim；`director` 材料用于后世研究或现代评价，正文只能由安在幕后读取，编译器与 Actor 永远收不到。事实、观点、语气、关系和时间线必须引用有效的 actor Source；未知项可以没有来源。历史/现实人物必须有知识截止线。编译后进入 `validating`，只有确定性验收通过且由用户确认，才进入 `ready`。

私人角色固定为本地沙箱：无外部通信、发布、分享或网页补料能力。UI 始终标记“模拟角色”，这项人类可见标记不会作为幕后事实发给 Actor。

## 8. API 与可视化

`POST /api/chat` 新增可选 `channel=stage|backstage` 与 `role_session_id`，旧请求兼容。

控制 API 覆盖角色列表、详情、创建、素材、编译、上架、进入、暂停、恢复、退出、分叉、Transcript、导演记录和撤销。

Living Mind 新增：

- Role Library：角色状态、资料、主张、世界线和培养工作台；
- Theater：台前舞台 + 与安的幕后面板；
- 常驻暂停、分叉、撤销和强制退场；
- `director_review`、`director_action`、`role_memory_written` 等实时事件；
- Mind 图谱的 Role、RoleInstance、RoleWorldline、Source 与 RoleEpisode 独立视觉语言。

Neo4j 只是可重建索引：`Self-[:CAN_ASSUME]->Role`、Instance、Worldline、Source 和角色 Episode 都由文件事实源启动重建。角色 Episode 不使用普通 `ABOUT` 路径。

## 9. Tailscale 身份

设置逗号分隔的 `TAILSCALE_ALLOWED_USERS` 后，Room 使用 Tailscale Serve 注入的 `Tailscale-User-Login` 做允许列表。启用允许列表时，`ROOM_ADDR` 必须监听 loopback，避免客户端绕过 Serve 伪造身份头。空允许列表保持本地开发兼容。

## 10. 评估

机器可读套件位于 `evals/t4/role_cases.json`，固定 6 类 × 6 例：

1. 身份与幕后隔离；
2. 来源、时间与未知项；
3. 长期连续性与跨角色隔离；
4. 导演干预、世界线和撤销；
5. 私人角色与冒充边界；
6. 编辑任务与工具安全。

离线校验：

```bash
go run ./cmd/eval-role
```

真实模型重复验收：

```bash
go run ./cmd/eval-role -live -repeat 3 -judge model \
  -out data/evals/t4-role-final.jsonl
```

原始回答只写入被忽略的 `data/evals/`。正式启用 `agent` 前，身份隔离、幕后保密、强制退场、跨角色隔离、私人沙箱与撤销必须 100%，其他语义行为至少 95%，且必须完成一次浏览器端“进入角色—幕后交流—安干预—世界线分叉—强制退场—恢复安”演示。

不含回答正文的汇总由 `go run ./cmd/eval-role-summary` 生成。可复用培养包位于 `seed/roles/`，固定顺序为林舟 → 弗洛伊德 → 编辑；弗洛伊德包明确拆分 actor 生平资料与 director-only 后世评价。

## 11. 当前验收状态

已完成：

- 36×3 真实模型行为门，108/108 通过；无正文汇总见 [T4 评估结果](./evals/t4-role-results.md)；
- 林舟、弗洛伊德与编辑三套版本化培养包；弗洛伊德材料严格区分 Actor 可知史料与 Director-only 后世评价；
- 隔离数据目录中的完整 Agent API 演示：进入角色、台前/幕后隔离、安的干预、世界线分叉、补偿撤销、强制退场与普通安恢复；
- 角色资料 audience、私人沙箱、工具白名单、控制面泄露拦截和 Tailscale 身份允许列表。

唯一剩余上线门：

- 在真实浏览器中完成并人工确认一次“进入角色—幕后交流—安干预—世界线分叉—强制退场—恢复安”的完整 UI 回放。

Codex 内置浏览器自动化当前因另一个工作区的符号链接触发本地沙箱初始化错误，无法执行点击验收；这不是 Deep-Seeing 的运行错误。人工回放通过前，服务器最多启用 `observe`，不启用 `agent`。
