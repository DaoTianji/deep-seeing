# 桌宠终端（Pet）

> 状态：Web MVP 与 Tauri 日常可用壳已落地。
> Web 入口：`http://127.0.0.1:3319/pet`
> 桌面壳：[`apps/pet-desktop`](../apps/pet-desktop)

## 定位

极简挂件 + 终端风聊天，只关心 ta 的可见回复：

- `/`：完整沟通室（含记忆侧栏）
- `/mind`：心智可观测
- `/pet`：点击挂件展开终端，只看回复
- `apps/pet-desktop`：Tauri 透明置顶窗，桌面只显示右下角圆球

`/pet` 与主沟通页共用同一 Room 进程与会话后端，走同一个 `POST /api/chat`。

## 浏览器用法（调试）

1. 启动 Room（例如 `go run ./cmd/see`）
2. 浏览器打开 `http://127.0.0.1:3319/pet`
3. 点击右下角挂件展开终端；再点挂件或按 `Esc` / `×` 收起

## Tauri 桌面壳

壳直接加载本机 `/pet`（同源，不改 CORS），Go 仍是唯一大脑。

### 依赖

- Node.js + npm（本目录用 npm）
- Rust（`cargo` / `rustc`，建议 rustup 安装 stable）
- macOS 优先；本轮不做 Windows 打包

`src-tauri/Cargo.toml` 对 `bit-vec` / `bit-set` 使用了本地 [`vendor/`](../apps/pet-desktop/vendor) patch（避免部分环境解压 crate 内 `.vscode` 失败）；功能与 crates.io 同版本。

### 一步启动（推荐）

```bash
# 仓库根目录
./pet
```

启动器会：

- 复用已经运行的 `http://127.0.0.1:3319` Room
- Room 未运行时启动 `go run ./cmd/see` 并等待就绪
- 自动设置仓库内 Rust 工具链与 `no_proxy`
- 启动 Tauri；退出时只清理由它启动的子进程

首次运行仍需 Node.js + npm 和 Rust stable；`apps/pet-desktop/dev.sh` 会在缺少
`node_modules` 时自动执行 `npm install`。

### 分别启动（排障）

```bash
# 终端 1
go run ./cmd/see

# 终端 2
apps/pet-desktop/dev.sh
```

行为：

- 启动后窗口约 `72×72`，贴主屏右下角，仅见圆球
- 点击展开为约 `520×680` 终端面板；再点 / `Esc` / `—` 缩回圆球
- 展开窗口可拖拽边缘或右下角调整大小（最小约 `380×460`）
- 本次运行内会记住上一次面板尺寸；展开/收起以当前位置右下角为锚点
- 进程名 / 标题为中性 `Notes`
- 圆球与面板标题栏可拖拽（`data-tauri-drag-region`）
- 顶部 `↗` 在系统浏览器打开完整 Room；`⌫` 只清空当前轻量视图，不清后端记忆

覆盖 Room 地址：改 [`apps/pet-desktop/src-tauri/tauri.conf.json`](../apps/pet-desktop/src-tauri/tauri.conf.json) 里的 `build.devUrl` 与 `app.windows[0].url`（默认 `http://127.0.0.1:3319/pet`）。

### 常见问题：一直卡在 `Waiting for your frontend dev server...`

若终端设了 `http_proxy` / `https_proxy`（如 `127.0.0.1:7897`）但没有 `no_proxy`，`tauri dev` 探测 `http://127.0.0.1:3319/pet` 的请求会被丢给代理，健康检查过不去，窗口（圆球）不弹出。解决：让本机绕过代理。

```bash
export no_proxy="127.0.0.1,::1,localhost"
export NO_PROXY="$no_proxy"
```

`./pet` 和 `dev.sh` 已自动设置该项。

## 行为边界

- 流式事件：渲染 `delta` / `done.answer`；工具调用只反映为简短状态
- 不请求 `/api/graph`、`/api/episodes` 等记忆 API
- `⌫` 清空的是 DOM 视图，不删除 STM、Episode 或 Turn Trace
- **本轮不做**：托盘、开机自启、安装器 sidecar、Flutter、Windows

## 相关文件

- [`internal/room/web/pet.html`](../internal/room/web/pet.html)
- [`internal/room/web/pet.css`](../internal/room/web/pet.css)
- [`internal/room/web/pet.js`](../internal/room/web/pet.js)
- 路由：`GET /pet`（[`internal/room/server.go`](../internal/room/server.go)）
- 桌面壳：[`apps/pet-desktop`](../apps/pet-desktop)
