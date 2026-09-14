# 书中见 · 比赛版交付（2026-09-14）

> 本文记录首轮上线的历史结果，下文未提交构建、旧 SHA 和测试数量均为首轮记录。后续比赛功能、首页和 922 句默认译文已整理提交；最新功能说明见 [参赛交付稿](competition-submission.md) 与 [默认译文](reading-default-translations.md)，增量验证见 [交接表](generated/HANDOFF_TABLE.md)。各次实际发布 SHA 与线上复测结果保存在被忽略的 `data/deployments/`。

## 体验地址与定位

[打开书中见](https://deep-seeing.tail165a8d.ts.net:8443/reading)。这是公网 HTTPS 地址，评委不需要安装 Tailscale；保留 `:8443`。用户已明确授权仅公开独立比赛版。

这是可实际体验的互动阅读原型，而不是通用图书导入平台或经过大规模验证的教育产品。内置 9 部作品，其中 6 篇有课文专题。核心闭环：阅读原文 → 向安或人物提问 → 读完后尝试改变选择 → 生成新结局、比较时间线并署名。

建议演示：

1. 输入独特笔名，打开首页推荐的《桃花源记》。询问“缘溪行”或人物当时处境，查看原文依据。
2. 回书架，进入《项链》改写。建议夫妻先坦白、询问价值，而非立刻举债赔偿。
3. 手动收束故事，查看原著和新故事线、自己的影响，署名并重新打开记录。
4. 时间不够时，使用首页“完整示例”。它明确标记为此前真实测试生成，含原著结局，不冒充本次实时生成。

## 今日收尾

- 首页增加“阅读时 / 读完后”两个明确入口；统一品牌为书中见。
- 增加预生成完整示例；保留其已知叙事局限说明。
- 长时间等待时提示复杂结局可能需要数分钟，勿重复提交。
- 公共版关闭不稳定的外部研究；隐藏作者工作台入口并拒绝作者管理 API。没有开放通用上传入口。
- 新增独立服务、独立用户、独立数据目录及模型调用额度控制。没有搬运安的正式记忆或本地用户记录。

## 发布身份

- 工作区：`data/worktrees/reading-showcase`。
- 分支：`codex/vibecoding-reading-showcase`。
- Git 基线：`974d4e4e9f47896917fe079994a866bdfd260f4d`。
- 本次是未提交工作区构建，未执行 commit/push；不能把 Git 基线当作完整发布源码。
- 二进制：Linux AMD64，`/opt/shuzhongjian/releases/96e71caf4d97/reading`。
- SHA-256：`96e71caf4d97d544b29769cf8eae4f7a923b77773dcb6b328106c216d169ba22`，本机与服务器一致。
- 服务：`shuzhongjian`，开机启动、异常重启，监听 `127.0.0.1:3320`。
- 数据：`/var/lib/shuzhongjian/data`；配置：`/etc/shuzhongjian/model.env`。只配置模型凭证，不含安的数据库连接或身份文件。

## 实测结果与未覆盖项

| 检查 | 结果 |
| --- | --- |
| Go 全量测试 | `go test ./...` 通过；依赖外部服务且按环境跳过的测试不算本次真实集成验收 |
| 相关 Race 测试 | `internal/story`、`internal/room`、`cmd/reading` 通过 |
| 前端 | 63 项测试通过；类型检查和生产构建通过 |
| 外网 HTTPS | 强制连接公共 IP `103.84.155.153` 而非 Tailnet IP，返回 200 |
| 真实伴读 | 《桃花源记》“缘溪行”问题返回回答及第 1 段证据；单次样本 4,970ms、2 次模型调用、1,688 token，不是性能保证 |
| 真实故事 | 创建《项链》分支 → 提建议 → 生成《一条假项链的夜晚》 → 署名 → 重新读取一致 |
| 人工内容检查 | 本次新故事选择提前坦白、揭示仿制项链，避免原著长期债务；这是模拟分支，朋友免赔等新增细节不是史实或原文 |
| 名字记录 | 不同名字笔记分离，同名恢复；这不是密码认证 |
| 重启 | 仅重启比赛服务；数据文件集合 SHA-256 前后一致，两个服务均 active |
| 私有接口 | 比赛端 `/api/runtime` 与 `/api/story/authors` 返回 404 |
| 浏览器 | Chrome 登录页截图无白屏；解锁后确认线上已登录首页显示“书中见”、两个推荐入口、9 篇作品、6 篇课文专题和故事记录区。继续点击时用户切换到其他工作页面，停止争用前台；独立后台浏览器超时、Chrome 自动化浏览器不可用。伴读/结局完整点击链路及移动端布局仍待实测 |

本次服务实际模型调用计数为 8（验收后读数）。测试笔名为 `发布验收-399943d4` 与 `发布验收-7aedd171`，均只含公开课文和虚构测试；不会影响其他名字的记录。

API 验收采用现有 Node 轻量执行器；Apifox 没有配置好的场景环境，未临时创建账号或导入集合。详见 [交接表](generated/HANDOFF_TABLE.md) 与 [复测方式](generated/READING_CURL.md)。

## 公网与成本边界

- 只有 `8443` 通过 [Tailscale Funnel](https://tailscale.com/docs/features/tailscale-funnel) 公开；原有 `443 → 3319` Tailscale Serve 仍私有。没有使用全局 reset 命令。
- 安的 `deep-seeing` 服务、Soul/Bond/记忆目录、Redis/Neo4j/PostgreSQL 均未修改。
- 名字不是密码：知道同一个名字就能查看其记录。登录页已明示；请勿存敏感内容。
- 单服务每日最多 300 次实际模型尝试，失败也计入，UTC 日期切换；计数文件持久化，损坏时拒绝模型请求。这不是金额上限，也不是 300 次聊天。
- 最多 3 个昂贵 HTTP 请求同时处理，POST 全局每分钟最多 120 次。后台阅读任务仍受现有队列和实际模型额度限制；不能据此声称完整并发压测已通过。
- `GOMEMLIMIT=512MiB`，服务内存上限 768MiB。验收后空闲 MemoryCurrent 约 9.2MiB，是 systemd 内存指标而非完整 RSS/数据库占用。
- 无高并发压测、无本轮全书内容人工校勘、无教育准确性保证。生成故事可能失真。网络或网关限额仍可影响体验。
- 本次未配置新的自动备份任务；独立数据需要后续备份。临时传输的配置副本已删除，正式配置保留。备份应视为私人数据。

## 运维与回滚

以下命令在服务器执行，仅作用于比赛版：

```sh
systemctl status shuzhongjian --no-pager
systemctl restart shuzhongjian
journalctl -u shuzhongjian -n 100 --no-pager
sha256sum /opt/shuzhongjian/current/reading
tailscale serve status
```

紧急下线比赛版，不影响安：

```sh
tailscale funnel --https=8443 off
systemctl stop shuzhongjian
```

重新上线：

```sh
systemctl start shuzhongjian
tailscale funnel --bg --https=8443 http://127.0.0.1:3320
```

手动备份（确认无人正在生成故事时执行；不备份正式模型密钥）：

```sh
install -d -m 0700 /var/backups/shuzhongjian
systemctl stop shuzhongjian
trap 'systemctl start shuzhongjian' EXIT
umask 077
tar -czf /var/backups/shuzhongjian/data-$(date -u +%Y%m%dT%H%M%SZ).tar.gz -C /var/lib/shuzhongjian data
systemctl start shuzhongjian
trap - EXIT
```

本次是独立首次发布，没有上一版比赛 release 可回滚。失败时按上述方式下线，保留数据；后续先备份，再部署新的不可变目录、切换 current，必要时恢复上一版本。不要覆盖原有安的 release、软链接或数据。
