# 书中见接口冒烟

在比赛 worktree 执行。基础运行创建两个虚构测试笔名及一条笔记，不调用模型。模型开关会实际消耗额度，勿无意义重复运行。

```sh
node scripts/check-reading-release.mjs https://deep-seeing.tail165a8d.ts.net:8443
READING_RELEASE_MODEL=1 node scripts/check-reading-release.mjs https://deep-seeing.tail165a8d.ts.net:8443
READING_RELEASE_STORY=1 node scripts/check-reading-release.mjs https://deep-seeing.tail165a8d.ts.net:8443
```

基础 curl（只使用自己的测试笔名；cookie 文件是临时测试记录访问凭据）：

```sh
release_cookie=$(mktemp)
chmod 600 "$release_cookie"
curl --fail-with-body -sS -c "$release_cookie" \
  -H 'Content-Type: application/json' \
  -d '{"name":"公开接口验收-请换成自己的独特测试名"}' \
  https://deep-seeing.tail165a8d.ts.net:8443/api/story/session
curl --fail-with-body -sS -b "$release_cookie" \
  https://deep-seeing.tail165a8d.ts.net:8443/api/story/books
curl --fail-with-body -sS -b "$release_cookie" \
  'https://deep-seeing.tail165a8d.ts.net:8443/api/story/companion?book=taohuayuan'
```

预期：登录响应包含 reader；书库长度 9、lesson_id 非空项 6；伴读返回本名字下状态。不同名字不是严格身份权限；同名可恢复同一数据。

只读隔离检查（预期 404，不应返回安的运行状态或作者管理数据）：

```sh
curl -sS -o /dev/null -w '%{http_code}\n' https://deep-seeing.tail165a8d.ts.net:8443/api/runtime
curl -sS -o /dev/null -w '%{http_code}\n' https://deep-seeing.tail165a8d.ts.net:8443/api/story/authors
```

真实模型流式请求需要当前 revision、request_id 与同一名字的 cookie；用上面的脚本完成，不复制过期版本号。`passed: true` 代表脚本断言通过，不代表全部语义和浏览器体验验收。
