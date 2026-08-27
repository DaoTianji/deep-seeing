const state = {
  busy: false,
  graph: { nodes: [], edges: [], available: false },
  episodes: [],
  proposals: [],
  mutations: [],
  traces: [],
  episodeFilter: "all",
  graphPinned: new Map(),
  recallActivation: emptyRecallActivation(),
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];
const svgNS = "http://www.w3.org/2000/svg";

document.addEventListener("DOMContentLoaded", async () => {
  bindUI();
  await Promise.all([loadRuntime(), loadHistory(), refreshMemory()]);
  updateClock();
  window.setInterval(updateClock, 30_000);
});

function bindUI() {
  $$(".memory-tab").forEach((tab) => {
    tab.addEventListener("click", () => selectView(tab.dataset.view));
  });
  $$(".segmented button").forEach((button) => {
    button.addEventListener("click", () => {
      $$(".segmented button").forEach((item) => item.classList.remove("active"));
      button.classList.add("active");
      state.episodeFilter = button.dataset.status;
      renderEpisodes();
    });
  });

  $("#composer").addEventListener("submit", sendMessage);
  $("#message-input").addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      $("#composer").requestSubmit();
    }
  });
  $("#message-input").addEventListener("input", autoGrowComposer);
  $("#refresh-button").addEventListener("click", refreshMemory);
  $("#review-button").addEventListener("click", runReview);
  $("#dream-button").addEventListener("click", runDream);
  $("#backup-button").addEventListener("click", runBackup);
  $("#clear-recall-activation").addEventListener("click", () => clearRecallActivation());
  window.addEventListener("resize", debounce(() => renderGraph(), 120));
}

async function loadRuntime() {
  try {
    const data = await getJSON("/api/runtime");
    const runtime = data.runtime;
    const graph = runtime.stores?.context_graph === "available" ? "Neo4j 已连接" : "Neo4j 暂不可用";
    $("#runtime-text").textContent = `${runtime.current_person} · ${runtime.model} · ${graph}`;
    $("#runtime-dot").classList.toggle("offline", runtime.stores?.context_graph !== "available");
    $("#room-time").textContent = formatTime(runtime.now);
  } catch (error) {
    $("#runtime-text").textContent = "运行状态暂不可见";
    $("#runtime-text").title = error.message;
    $("#runtime-dot").classList.add("offline");
  }
}

async function loadHistory() {
  try {
    const data = await getJSON("/api/history");
    for (const message of data.messages || []) {
      if (["user", "assistant", "summary"].includes(message.role)) {
        addMessage(message.role, message.content, false);
      }
    }
    scrollMessages();
  } catch (error) {
    toast(`过去的会话暂时无法读取：${error.message}`, true);
  }
}

async function sendMessage(event) {
  event.preventDefault();
  if (state.busy) return;
  const input = $("#message-input");
  const message = input.value.trim();
  if (!message) return;

  state.busy = true;
  clearRecallActivation(false);
  setComposerBusy(true);
  input.value = "";
  autoGrowComposer();
  addMessage("user", message);
  const assistant = addMessage("assistant", "");
  setActivity("正在理解这句话");

  let streamError = "";
  try {
    const response = await fetch("/api/chat", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message }),
    });
    if (!response.ok) {
      let detail = `HTTP ${response.status}`;
      try {
        const payload = await response.json();
        if (payload?.error) detail = payload.error;
      } catch {
        // keep status text
      }
      throw new Error(detail);
    }
    if (!response.body) throw new Error("浏览器不支持流式回应");
    await readNDJSON(response.body, (eventData) => {
      if (eventData.type === "delta") {
        appendMessageDelta(assistant.body, eventData.data);
        setActivity("正在说话");
        scrollMessages();
      } else if (eventData.type === "tool") {
        setActivity(`正在使用 ${friendlyTool(eventData.data?.name || "tool")}`);
      } else if (eventData.type === "task_context") {
        const workspaces = eventData.data?.workspace_ids?.length || 0;
        const intents = eventData.data?.intent_ids?.length || 0;
        const focused = eventData.data?.focus_workspace_id || eventData.data?.focus_intent_id;
        setActivity(`已准备任务处境快照 · Workspace ${workspaces} / Intent ${intents}${focused ? " · 含会话焦点" : ""}`);
      } else if (eventData.type === "task_context_expand") {
        const source = eventData.data?.source === "intent" ? "Intent" : "Workspace";
        const operation = eventData.data?.operation === "list" ? "调查" : "展开";
        setActivity(eventData.data?.error ? `${source} ${operation}失败` : `正在${operation} ${source} 处境`);
      } else if (eventData.type === "task_context_focus") {
        const focus = eventData.data || {};
        if (focus.action === "clarify") {
          setActivity("当前处境仍有歧义，准备向你确认");
        } else if (focus.action === "clear") {
          setActivity("已清除当前会话焦点");
        } else {
          setActivity(`已确认当前焦点 · ${focus.workspace_id || focus.intent_id || focus.action}`);
        }
      } else if (eventData.type === "context_source") {
        activateContextSource(eventData.data || {});
      } else if (eventData.type === "context_candidate") {
        activateContextCandidate(eventData.data || {});
      } else if (eventData.type === "context_read") {
        activateContextRead(eventData.data || {});
      } else if (eventData.type === "context_use") {
        activateContextUse(eventData.data || {});
		} else if (eventData.type === "attention_snapshot") {
			activateAttentionSnapshot(eventData.data || {});
			setActivity(`注意工作区 · ${(eventData.data?.items || []).length} 个保留线索`);
		} else if (eventData.type === "attention_decision") {
			activateAttentionDecision(eventData.data || {});
			setActivity(`注意焦点调整 · ${eventData.data?.to || "unknown"}`);
      } else if (eventData.type === "recall_search") {
        activateRecallSearch(eventData.data || {});
        const count = Number(eventData.data?.result_count || 0);
        setActivity(count ? `激活了 ${count} 条记忆候选` : "这次搜索没有找到记忆候选");
      } else if (eventData.type === "recall_read") {
        activateRecallRead(eventData.data || {});
        setActivity(eventData.data?.error ? "候选正文读取失败" : "正在核对一条记忆正文");
      } else if (eventData.type === "recall_evidence") {
        activateRecallEvidence(eventData.data || {});
        setActivity(eventData.data?.status === "used" ? "确认了一条回答证据" : "排除了一条记忆候选");
      } else if (eventData.type === "error") {
        streamError = eventData.data?.message || "对话失败";
      } else if (eventData.type === "done" && !assistant.body.dataset.rawMessage && eventData.data?.answer) {
        setMessageContent(assistant.body, eventData.data.answer);
      }
    });
    if (streamError) throw new Error(streamError);
    if (!assistant.body.dataset.rawMessage) {
      setMessageContent(assistant.body, "这轮没有生成可见文字（可能只调用了工具）。可看右边「痕迹」。");
    }
  } catch (error) {
    const detail = String(error?.message || "对话失败");
    if (!assistant.body.dataset.rawMessage) {
      setMessageContent(assistant.body, `这次回应没有完成。\n\n原因：${detail}`);
    } else {
      appendMessageDelta(assistant.body, `\n\n—\n回应中断：${detail}`);
    }
    assistant.body.classList.add("message-error");
    toast(detail, true);
  } finally {
    state.busy = false;
    setComposerBusy(false);
    clearActivity();
    await refreshMemory();
    input.focus();
  }
}

async function refreshMemory() {
  const button = $("#refresh-button");
  if (button) button.classList.add("loading");
  try {
    const [graph, episodes, proposals, mutations, traces] = await Promise.all([
      getJSON("/api/graph?limit=120"),
      getJSON("/api/episodes?limit=150&all=1"),
      getJSON("/api/proposals?limit=100"),
      getJSON("/api/mutations?limit=100"),
      getJSON("/api/traces?limit=100"),
    ]);
    state.graph = graph;
    state.episodes = episodes.episodes || [];
    state.proposals = proposals.proposals || [];
    state.mutations = mutations.mutations || [];
    state.traces = traces.traces || [];
    renderGraph();
    renderEpisodes();
    renderProposals();
    renderMutations();
    renderTraces();
  } catch (error) {
    toast(`记忆刷新失败：${error.message}`, true);
  } finally {
    if (button) button.classList.remove("loading");
  }
}

function selectView(name) {
  $$(".memory-tab").forEach((tab) => tab.classList.toggle("active", tab.dataset.view === name));
  $$(".memory-view").forEach((view) => view.classList.toggle("active", view.id === `view-${name}`));
  if (name === "graph") requestAnimationFrame(renderGraph);
}

function clearRecallActivation(render = true) {
  state.recallActivation = emptyRecallActivation();
  if (render) renderGraph();
  renderRecallActivationStatus();
}

function emptyRecallActivation() {
  return {
    candidates: new Set(), read: new Set(), used: new Set(), dismissed: new Set(), focus: new Set(),
    sourceByID: new Map(), sources: new Map(), attentionByID: new Map(), query: "", source: "", searches: 0,
  };
}

function activateRecallSearch(search) {

  for (const id of search.result_ids || []) state.recallActivation.candidates.add(id);
  for (const id of search.result_ids || []) state.recallActivation.sourceByID.set(id, "episode");
  state.recallActivation.query = search.query || state.recallActivation.query;
  state.recallActivation.source = "live";
  state.recallActivation.searches += 1;
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateRecallRead(read) {
  if (read.episode_id && !read.error) state.recallActivation.read.add(read.episode_id);
  if (read.episode_id) state.recallActivation.sourceByID.set(read.episode_id, "episode");
  state.recallActivation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateRecallEvidence(event) {
  if (!event.episode_id) return;
  state.recallActivation.sourceByID.set(event.episode_id, "episode");
  if (event.status === "used") state.recallActivation.used.add(event.episode_id);
  if (event.status === "dismissed") state.recallActivation.dismissed.add(event.episode_id);
  state.recallActivation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}
function activateContextSource(event) {
  if (!event.source) return;
  state.recallActivation.sources.set(event.source, event.state || "unknown");
  state.recallActivation.source = "live";
  renderRecallActivationStatus();
}

function activateContextCandidate(event) {
  if (event.source === "episode") return;
  for (const id of event.result_ids || []) {
    state.recallActivation.candidates.add(id);
    state.recallActivation.sourceByID.set(id, event.source || "context");
  }
  state.recallActivation.query = event.query || state.recallActivation.query;
  state.recallActivation.searches += event.operation === "snapshot" ? 0 : 1;
  state.recallActivation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateContextRead(event) {
  if (event.source === "episode" || !event.id) return;
  state.recallActivation.sourceByID.set(event.id, event.source || "context");
  if (event.ok && !event.error) state.recallActivation.read.add(event.id);
  state.recallActivation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateContextUse(event) {
  if (event.source === "episode" || !event.id) return;
  state.recallActivation.sourceByID.set(event.id, event.source || "context");
  if (event.disposition === "used") state.recallActivation.used.add(event.id);
  if (event.disposition === "dismissed") state.recallActivation.dismissed.add(event.id);
  if (event.disposition === "focus") state.recallActivation.focus.add(event.id);
  state.recallActivation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateAttentionSnapshot(snapshot) {
  const activation = state.recallActivation;
  activation.attentionByID.clear();
  for (const item of snapshot.items || []) {
    if (!item.id || !item.tier) continue;
    activation.attentionByID.set(item.id, item.tier);
    activation.sourceByID.set(item.id, item.source || "context");
  }
  activation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function activateAttentionDecision(event) {
  const activation = state.recallActivation;
  if (event.replaced_id) {
    activation.attentionByID.delete(event.replaced_id);
  }
  if (event.id) {
    if (event.to === "drop") activation.attentionByID.delete(event.id);
    else if (event.to) activation.attentionByID.set(event.id, event.to);
    activation.sourceByID.set(event.id, event.source || "context");
  }
  activation.source = "live";
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function replayRecallTrace(trace) {
  const activation = emptyRecallActivation();
  const contextCandidates = trace.context_candidates || [];
  const unified = contextCandidates.length || (trace.context_reads || []).length || (trace.context_uses || []).length;
  for (const source of trace.context_sources || []) {
    if (source.source) activation.sources.set(source.source, source.state || "unknown");
  }
  for (const item of trace.attention?.items || []) {
    if (!item.id || !item.tier) continue;
    activation.attentionByID.set(item.id, item.tier);
    activation.sourceByID.set(item.id, item.source || "context");
  }
  for (const event of trace.attention_decisions || []) {
    if (event.replaced_id) activation.attentionByID.delete(event.replaced_id);
    if (!event.id) continue;
    if (event.to === "drop") activation.attentionByID.delete(event.id);
    else if (event.to) activation.attentionByID.set(event.id, event.to);
    activation.sourceByID.set(event.id, event.source || "context");
  }

  if (unified) {
    for (const event of contextCandidates) {
      for (const id of event.result_ids || []) {
        activation.candidates.add(id);
        activation.sourceByID.set(id, event.source || "context");
      }
    }
    for (const event of trace.context_reads || []) {
      if (!event.id) continue;
      activation.sourceByID.set(event.id, event.source || "context");
      if (event.ok && !event.error) activation.read.add(event.id);
    }
    for (const event of trace.context_uses || []) {
      if (!event.id) continue;
      activation.sourceByID.set(event.id, event.source || "context");
      if (event.disposition === "used") activation.used.add(event.id);
      if (event.disposition === "dismissed") activation.dismissed.add(event.id);
      if (event.disposition === "focus") activation.focus.add(event.id);
    }
    activation.query = contextCandidates.map((event) => event.query).filter(Boolean).join(" → ");
    activation.searches = contextCandidates.filter((event) => event.operation !== "snapshot").length;
  } else {
    const searches = trace.recall_searches || [];
    for (const search of searches) {
      for (const id of search.result_ids || []) {
        activation.candidates.add(id);
        activation.sourceByID.set(id, "episode");
      }
    }
    for (const read of trace.recall_reads || []) {
      if (read.episode_id && !read.error) activation.read.add(read.episode_id);
    }
    for (const event of trace.recall_evidence || []) {
      if (event.status === "used") activation.used.add(event.episode_id);
      if (event.status === "dismissed") activation.dismissed.add(event.episode_id);
    }
    activation.query = searches.map((search) => search.query).filter(Boolean).join(" → ");
    activation.searches = searches.length;
  }
  activation.source = "trace";
  state.recallActivation = activation;
  selectView("graph");
  requestAnimationFrame(renderGraph);
}

function renderRecallActivationStatus() {
  const banner = $("#recall-activation");
  if (!banner) return;
  const activation = state.recallActivation;
  const eventCount = activation.candidates.size + activation.read.size + activation.used.size +
    activation.dismissed.size + activation.focus.size + activation.sources.size + activation.attentionByID.size;
  banner.hidden = !eventCount;
  if (!eventCount) return;
  const label = activation.source === "trace" ? "回放上下文激活" : "本轮上下文激活";
  const sourceNames = [...new Set([...activation.sourceByID.values()])].map(friendlyContextSource);
  $("#recall-activation-label").textContent =
    `${label} · 候选 ${activation.candidates.size} · 已读 ${activation.read.size} · 采用 ${activation.used.size} · 焦点 ${activation.focus.size} · 排除 ${activation.dismissed.size} · 注意 ${activation.attentionByID.size}`;
  $("#recall-activation-query").textContent = activation.query
    ? `${sourceNames.join(" + ") || "上下文"}：${truncate(activation.query, 58)}`
    : `来源：${sourceNames.join(" + ") || [...activation.sources.keys()].map(friendlyContextSource).join(" + ") || "Bond 基线"}`;
}

function recallStatus(id) {
  const activation = state.recallActivation;
  let status = "";
  if (activation.used.has(id)) status = "recall-used";
  else if (activation.dismissed.has(id)) status = "recall-dismissed";
  else if (activation.read.has(id)) status = "recall-read";
  else if (activation.candidates.has(id)) status = "recall-candidate";
  if (activation.focus.has(id)) status += " context-focus";
  return status.trim();
}

function friendlyContextSource(source) {
  return ({
    bond: "Bond", scene_norm: "SceneNorm", workspace: "Workspace",
    intent: "Intent", proposal: "Proposal", episode: "Episode",
  })[source] || source || "Context";
}

function contextSourceKind(source) {
  return ({
    scene_norm: "SceneNorm", workspace: "Workspace", intent: "Intent",
    proposal: "Proposal", episode: "Episode",
  })[source] || "Context";
}

function contextSourceGlyph(source) {
  return ({ scene_norm: "◇", workspace: "W", intent: "I", proposal: "?", episode: "E" })[source] || "·";
}

function contextGraphView(graph) {
  const nodes = [...(graph.nodes || [])];
  const edges = [...(graph.edges || [])];
  const known = new Set(nodes.map((node) => node.id));
  const activeIDs = new Set([
    ...state.recallActivation.candidates, ...state.recallActivation.read,
    ...state.recallActivation.used, ...state.recallActivation.dismissed, ...state.recallActivation.focus,
    ...state.recallActivation.attentionByID.keys(),
  ]);
  if (!nodes.length && activeIDs.size) {
    nodes.push({ id: "context:turn", kind: "Self", title: "本轮问题", status: "context-anchor" });
    known.add("context:turn");
  }
  const anchor = nodes.find((node) => node.kind === "Person") || nodes.find((node) => node.kind === "Self");
  for (const id of activeIDs) {
    if (known.has(id)) continue;
    const source = state.recallActivation.sourceByID.get(id) || "episode";
    nodes.push({
      id, kind: contextSourceKind(source), title: friendlyContextSource(source),
      subtitle: id, status: "context-virtual", context_source: source,
    });
    known.add(id);
    if (anchor) {
      edges.push({ id: `context:${anchor.id}:${id}`, source: anchor.id, target: id, kind: "CONTEXT" });
    }
  }
  return { nodes, edges, available: Boolean(graph.available || activeIDs.size) };
}

function addMessage(role, content, animate = true) {
  $("#empty-conversation")?.remove();
  const wrapper = create("article", `message ${role}`);
  if (!animate) wrapper.style.animation = "none";
  const meta = create("div", "message-meta");
  const roleLabel = create("span", "message-role", roleLabelText(role));
  const time = create("time", "", formatTime(new Date().toISOString()));
  meta.append(roleLabel, time);
  const body = create("div", "message-body");
  setMessageContent(body, content);
  if (role === "assistant" || role === "user") {
    const copy = create("button", "copy-button", "复制");
    copy.type = "button";
    copy.title = "复制这条消息";
    copy.addEventListener("click", async (event) => {
      event.stopPropagation();
      try {
        await navigator.clipboard.writeText(body.dataset.rawMessage || body.textContent || "");
        copy.textContent = "已复制";
        window.setTimeout(() => { copy.textContent = "复制"; }, 1400);
      } catch {
        toast("复制失败，请手动选择文字", true);
      }
    });
    meta.append(copy);
  }
  wrapper.append(meta, body);
  $("#messages").append(wrapper);
  scrollMessages();
  return { wrapper, body };
}

function roleLabelText(role) {
  if (role === "user") return "mudnet";
  if (role === "summary") return "会话摘要";
  return "Deep-Seeing";
}

function setMessageContent(body, markdown) {
  body.dataset.rawMessage = String(markdown || "");
  renderMarkdown(body, body.dataset.rawMessage);
}

function appendMessageDelta(body, delta) {
  body.dataset.rawMessage = (body.dataset.rawMessage || "") + String(delta || "");
  renderMarkdown(body, body.dataset.rawMessage);
}

// Markdown is rendered with DOM nodes only. Model/user text never enters innerHTML.
function renderMarkdown(container, markdown) {
  container.replaceChildren();
  const lines = String(markdown || "").replace(/\r\n?/g, "\n").split("\n");
  let index = 0;
  while (index < lines.length) {
    const line = lines[index];
    if (line.startsWith("```")) {
      index = appendCodeBlock(container, lines, index);
      continue;
    }

    const heading = parseHeading(line);
    if (heading) {
      appendHeading(container, heading);
      index += 1;
      continue;
    }

    const firstListItem = parseListItem(line);
    if (firstListItem) {
      index = appendList(container, lines, index, firstListItem.ordered);
      continue;
    }

    if (line.startsWith(">")) {
      index = appendQuote(container, lines, index);
      continue;
    }

    if (isHorizontalRule(line)) {
      container.append(create("hr", "md-rule"));
      index += 1;
      continue;
    }

    if (line.trim() === "") {
      index += 1;
      continue;
    }

    index = appendParagraph(container, lines, index);
  }
}

function appendCodeBlock(container, lines, start) {
  const language = lines[start].slice(3).trim();
  const codeLines = [];
  let index = start + 1;
  while (index < lines.length && !lines[index].startsWith("```")) {
    codeLines.push(lines[index]);
    index += 1;
  }
  if (index < lines.length) index += 1;
  const pre = create("pre", "md-code-block");
  const languageClass = language ? `language-${safeClassName(language)}` : "";
  pre.append(create("code", languageClass, codeLines.join("\n")));
  container.append(pre);
  return index;
}

function appendHeading(container, heading) {
  const level = Math.min(heading.depth + 2, 6);
  const node = create(`h${level}`, `md-heading md-heading-${heading.depth}`);
  appendInlineMarkdown(node, heading.text);
  container.append(node);
}

function appendList(container, lines, start, ordered) {
  const list = create(ordered ? "ol" : "ul", "md-list");
  let index = start;
  while (index < lines.length) {
    const itemData = parseListItem(lines[index]);
    if (!itemData || itemData.ordered !== ordered) break;
    const item = create("li");
    appendInlineMarkdown(item, itemData.text);
    list.append(item);
    index += 1;
  }
  container.append(list);
  return index;
}

function appendQuote(container, lines, start) {
  const quoteLines = [];
  let index = start;
  while (index < lines.length && lines[index].startsWith(">")) {
    const offset = lines[index].charAt(1) === " " ? 2 : 1;
    quoteLines.push(lines[index].slice(offset));
    index += 1;
  }
  const quote = create("blockquote", "md-quote");
  appendInlineMarkdown(quote, quoteLines.join("\n"));
  container.append(quote);
  return index;
}

function appendParagraph(container, lines, start) {
  const paragraphLines = [lines[start]];
  let index = start + 1;
  while (index < lines.length && lines[index].trim() !== "" && !isMarkdownBlockStart(lines[index])) {
    paragraphLines.push(lines[index]);
    index += 1;
  }
  const paragraph = create("p", "md-paragraph");
  appendInlineMarkdown(paragraph, paragraphLines.join("\n"));
  container.append(paragraph);
  return index;
}

function isMarkdownBlockStart(line) {
  return line.startsWith("```")
    || Boolean(parseHeading(line))
    || Boolean(parseListItem(line))
    || line.startsWith(">")
    || isHorizontalRule(line);
}

function appendInlineMarkdown(parent, text) {
  const source = String(text || "");
  let textStart = 0;
  let cursor = 0;
  while (cursor < source.length) {
    const token = readInlineToken(source, cursor);
    if (!token) {
      cursor += 1;
      continue;
    }
    if (cursor > textStart) appendTextWithBreaks(parent, source.slice(textStart, cursor));
    appendInlineToken(parent, token);
    cursor = token.end;
    textStart = cursor;
  }
  if (textStart < source.length) appendTextWithBreaks(parent, source.slice(textStart));
}

function parseHeading(line) {
  let depth = 0;
  while (depth < 4 && line[depth] === "#") depth += 1;
  if (depth === 0 || line[depth] !== " ") return null;
  const text = line.slice(depth + 1).trim();
  return text ? { depth, text } : null;
}

function parseListItem(line) {
  const text = line.trimStart();
  if (["-", "*", "+"].includes(text[0]) && text[1] === " ") {
    return { ordered: false, text: text.slice(2) };
  }
  let digitEnd = 0;
  while (digitEnd < text.length && text[digitEnd] >= "0" && text[digitEnd] <= "9") digitEnd += 1;
  if (digitEnd > 0 && text[digitEnd] === "." && text[digitEnd + 1] === " ") {
    return { ordered: true, text: text.slice(digitEnd + 2) };
  }
  return null;
}

function isHorizontalRule(line) {
  const compact = line.replaceAll(" ", "").replaceAll("\t", "");
  if (compact.length < 3) return false;
  const marker = compact[0];
  if (!["-", "*", "_"].includes(marker)) return false;
  return [...compact].every((char) => char === marker);
}

function readInlineToken(source, start) {
  const char = source[start];
  if (char === "`") return readDelimitedToken(source, start, "`", "code");
  if (char === "[" ) return readLinkToken(source, start);
  if (char === "*" || char === "_") {
    const strong = source[start + 1] === char;
    return readDelimitedToken(source, start, strong ? char + char : char, strong ? "strong" : "em");
  }
  return null;
}

function readDelimitedToken(source, start, delimiter, kind) {
  const contentStart = start + delimiter.length;
  const close = source.indexOf(delimiter, contentStart);
  if (close <= contentStart || source.slice(contentStart, close).includes("\n")) return null;
  return { kind, text: source.slice(contentStart, close), end: close + delimiter.length };
}

function readLinkToken(source, start) {
  const labelEnd = source.indexOf("](", start + 1);
  if (labelEnd < 0) return null;
  const hrefEnd = source.indexOf(")", labelEnd + 2);
  if (hrefEnd < 0) return null;
  const label = source.slice(start + 1, labelEnd);
  const rawHref = source.slice(labelEnd + 2, hrefEnd);
  if (!label || !rawHref || rawHref.includes("\n") || rawHref.includes(" ")) return null;
  return { kind: "link", text: label, href: rawHref, end: hrefEnd + 1 };
}

function appendInlineToken(parent, token) {
  if (token.kind === "code") {
    parent.append(create("code", "md-inline-code", token.text));
    return;
  }
  if (token.kind === "strong") {
    parent.append(create("strong", "", token.text));
    return;
  }
  if (token.kind === "em") {
    parent.append(create("em", "", token.text));
    return;
  }
  const href = safeLink(token.href);
  if (!href) {
    appendTextWithBreaks(parent, token.text);
    return;
  }
  const link = create("a", "md-link", token.text);
  link.href = href;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  parent.append(link);
}

function appendTextWithBreaks(parent, text) {
  const parts = String(text).split("\n");
  parts.forEach((part, index) => {
    if (index > 0) parent.append(document.createElement("br"));
    parent.append(document.createTextNode(part));
  });
}

function safeLink(raw) {
  try {
    const url = new URL(raw, window.location.href);
    if (!["http:", "https:"].includes(url.protocol)) return "";
    return url.href;
  } catch {
    return "";
  }
}

function safeClassName(value) {
  return String(value).toLowerCase().replace(/[^a-z0-9_-]/g, "").slice(0, 32);
}

function setActivity(text) {
  $("#activity-strip").hidden = false;
  $("#activity-text").textContent = text;
}
function clearActivity() { $("#activity-strip").hidden = true; }
function setComposerBusy(busy) {
  $("#send-button").disabled = busy;
  $("#message-input").disabled = busy;
}
function autoGrowComposer() {
  const input = $("#message-input");
  input.style.height = "auto";
  input.style.height = `${Math.min(input.scrollHeight, 150)}px`;
}
function scrollMessages() {
  const messages = $("#messages");
  requestAnimationFrame(() => { messages.scrollTop = messages.scrollHeight; });
}

function renderGraph() {
  const svg = $("#memory-graph");
  if (!svg || !$("#view-graph").classList.contains("active")) return;
  svg.replaceChildren();
  const graph = contextGraphView(state.graph);
  const nodes = graph.nodes;
  const edges = graph.edges;
  $("#graph-summary").textContent = state.graph.available
    ? `${nodes.length} 个节点 · ${edges.length} 条关系 · 含本轮上下文`
    : graph.available ? `${nodes.length} 个临时上下文节点` : "Neo4j 暂不可用；Episode 文件仍然存在";
  $("#graph-empty").hidden = graph.available && nodes.length > 0;
  renderRecallActivationStatus();
  if (!graph.available || nodes.length === 0) return;

  const rect = svg.getBoundingClientRect();
  const width = Math.max(rect.width, 420);
  const height = Math.max(rect.height, 300);
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
  const layout = semanticLayout(nodes, width, height);

  const defs = svgEl("defs");
  const marker = svgEl("marker", { id: "arrow", viewBox: "0 0 10 10", refX: "8", refY: "5", markerWidth: "5", markerHeight: "5", orient: "auto-start-reverse" });
  marker.append(svgEl("path", { d: "M 0 0 L 10 5 L 0 10 z", fill: "rgba(122,112,94,.4)" }));
  defs.append(marker);
  svg.append(defs);
  const edgeRefs = appendGraphEdges(svg, edges, layout);
  appendGraphNodes(svg, nodes, layout, edgeRefs, { width, height });
}

function appendGraphEdges(svg, edges, layout) {
  const edgeLayer = svgEl("g", { class: "edge-layer" });
  const refs = [];
  const curvatures = parallelCurvatures(edges);
  for (const edge of edges) {
    const source = layout.get(edge.source);
    const target = layout.get(edge.target);
    if (!source || !target) continue;
    const curvature = curvatures.get(edge.id) || 0;
    const geometry = edgeGeometry(source, target, curvature);
    const line = svgEl("path", {
      d: geometry.d,
      class: `graph-edge ${edge.kind} ${recallStatus(edge.target) || recallStatus(edge.source)}`,
      "marker-end": "url(#arrow)",
    });
    // transparent wide path so thin relationships stay clickable
    const hit = svgEl("path", { d: geometry.d, class: "graph-edge-hit" });
    hit.addEventListener("click", () => showGraphDetail(edge, "edge"));
    const ref = { edge, lines: [line, hit], label: null, curvature };
    edgeLayer.append(line, hit);
    if (["BOND", "KNOWS", "CALLS"].includes(edge.kind)) {
      const label = svgEl("text", { x: geometry.labelX, y: geometry.labelY, class: "edge-label" });
      label.textContent = edge.kind;
      label.addEventListener("click", () => showGraphDetail(edge, "edge"));
      edgeLayer.append(label);
      ref.label = label;
    }
    refs.push(ref);
  }
  svg.append(edgeLayer);
  return refs;
}

// parallelCurvatures fans out relationships that share the same node pair,
// so overlapping labels (e.g. BOND + KNOWS) stay readable.
function parallelCurvatures(edges) {
  const groups = new Map();
  for (const edge of edges) {
    const key = [edge.source, edge.target].sort().join("\u0000");
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(edge);
  }
  const result = new Map();
  for (const group of groups.values()) {
    group.forEach((edge, index) => {
      result.set(edge.id, group.length === 1 ? 0 : (index - (group.length - 1) / 2) * 26);
    });
  }
  return result;
}

function edgeGeometry(source, target, curvature) {
  const midX = (source.x + target.x) / 2;
  const midY = (source.y + target.y) / 2;
  if (!curvature) {
    return { d: `M ${source.x} ${source.y} L ${target.x} ${target.y}`, labelX: midX, labelY: midY - 6 };
  }
  const dx = target.x - source.x;
  const dy = target.y - source.y;
  const length = Math.hypot(dx, dy) || 1;
  const controlX = midX + (-dy / length) * curvature * 2;
  const controlY = midY + (dx / length) * curvature * 2;
  return {
    d: `M ${source.x} ${source.y} Q ${controlX} ${controlY} ${target.x} ${target.y}`,
    labelX: (source.x + 2 * controlX + target.x) / 4,
    labelY: (source.y + 2 * controlY + target.y) / 4 - 4,
  };
}

function appendGraphNodes(svg, nodes, layout, edgeRefs, bounds) {
  const nodeLayer = svgEl("g", { class: "node-layer" });
  for (const node of nodes) {
    const pos = layout.get(node.id);
    if (!pos) continue;
    const isAnchor = node.kind === "Self" || node.kind === "Person";
    const radius = isAnchor ? 27 : 12;
    const status = recallStatus(node.id);
    const group = svgEl("g", {
      class: `node-group ${status}`,
      transform: `translate(${pos.x} ${pos.y})`, tabindex: "0",
    });
    group.append(svgEl("circle", { r: radius + 4, class: "node-halo" }));
    const attentionTier = state.recallActivation.attentionByID.get(node.id);
    if (attentionTier) {
      group.append(svgEl("circle", { r: radius + 10, class: `attention-ring ${attentionTier}` }));
    }
    group.append(svgEl("circle", { r: radius, class: `node-core ${node.kind} ${node.status || ""} ${node.anchor === "Self" ? "about-self" : ""}` }));
    if (node.context_source) {
      const glyph = svgEl("text", { y: 4, class: "node-glyph" });
      glyph.textContent = contextSourceGlyph(node.context_source);
      group.append(glyph);
    }
    const label = svgEl("text", { y: isAnchor ? 4 : radius + 14, class: "node-label" });
    label.textContent = truncate(node.title, isAnchor ? 13 : 16);
    group.append(label);
    if (isAnchor) {
      const sub = svgEl("text", { y: radius + 14, class: "node-sub" });
      sub.textContent = node.kind;
      group.append(sub);
    }
    group.addEventListener("click", async () => {
      if (group.dataset.dragged === "1") {
        delete group.dataset.dragged;
        return;
      }
      $$(".node-group").forEach((item) => item.classList.remove("selected"));
      group.classList.add("selected");
      if (node.kind === "Episode") {
        try {
          const full = await getJSON(`/api/episode/${encodeURIComponent(node.id)}`);
          showGraphDetail(full, "episode");
        } catch { showGraphDetail(node, "node"); }
      } else {
        showGraphDetail(node, "node");
      }
    });
    group.addEventListener("dblclick", () => {
      state.graphPinned.delete(node.id);
      renderGraph();
    });
    enableNodeDrag(svg, group, node, layout, edgeRefs, bounds, state.graphPinned);
    nodeLayer.append(group);
  }
  svg.append(nodeLayer);
}

// enableNodeDrag lets a node be repositioned; pinned coordinates survive re-render.
function enableNodeDrag(svg, group, node, layout, edgeRefs, bounds, pinned) {
  let active = false;
  group.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) return;
    active = true;
    delete group.dataset.dragged;
    group.classList.add("dragging");
    group.setPointerCapture?.(event.pointerId);
    event.preventDefault();
  });
  group.addEventListener("pointermove", (event) => {
    if (!active) return;
    const point = svgPoint(svg, event);
    const position = {
      x: clamp(point.x, 24, bounds.width - 24),
      y: clamp(point.y, 24, bounds.height - 28),
    };
    layout.set(node.id, position);
    pinned.set(node.id, position);
    group.setAttribute("transform", `translate(${position.x} ${position.y})`);
    group.dataset.dragged = "1";
    updateEdgeGeometry(edgeRefs, layout);
  });
  const finish = (event) => {
    if (!active) return;
    active = false;
    group.classList.remove("dragging");
    group.releasePointerCapture?.(event.pointerId);
  };
  group.addEventListener("pointerup", finish);
  group.addEventListener("pointercancel", finish);
}

function updateEdgeGeometry(edgeRefs, layout) {
  for (const ref of edgeRefs) {
    const source = layout.get(ref.edge.source);
    const target = layout.get(ref.edge.target);
    if (!source || !target) continue;
    const geometry = edgeGeometry(source, target, ref.curvature);
    for (const line of ref.lines) line.setAttribute("d", geometry.d);
    if (ref.label) {
      ref.label.setAttribute("x", geometry.labelX);
      ref.label.setAttribute("y", geometry.labelY);
    }
  }
}

function svgPoint(svg, event) {
  const rect = svg.getBoundingClientRect();
  const box = svg.viewBox.baseVal;
  const scaleX = box && box.width ? box.width / rect.width : 1;
  const scaleY = box && box.height ? box.height / rect.height : 1;
  return {
    x: (event.clientX - rect.left) * scaleX,
    y: (event.clientY - rect.top) * scaleY,
  };
}

function semanticLayout(nodes, width, height) {
  const result = new Map();
  const self = nodes.find((node) => node.kind === "Self");
  const person = nodes.find((node) => node.kind === "Person");
  const selfPos = { x: width * .28, y: height * .48 };
  const personPos = { x: width * .72, y: height * .48 };
  if (self) result.set(self.id, selfPos);
  if (person) result.set(person.id, personPos);

  const aroundSelf = nodes.filter((node) => node.kind === "Episode" && node.anchor === "Self");
  const aroundPerson = nodes.filter((node) => node.kind === "Episode" && node.anchor !== "Self");
  placeEpisodeRing(result, aroundSelf, selfPos.x, selfPos.y, width, height, true);
  placeEpisodeRing(result, aroundPerson, personPos.x, personPos.y, width, height, false);
  const contextNodes = nodes.filter((node) => !["Self", "Person", "Episode"].includes(node.kind));
  placeContextLane(result, contextNodes, width, height);
  for (const [id, position] of state.graphPinned) {
    if (result.has(id)) result.set(id, position);
  }
  return result;
}

function placeEpisodeRing(result, episodes, cx, cy, width, height, leftSide) {
  const rx = Math.min(width * .22, 120);
  const ry = Math.min(height * .36, 118);
  episodes.forEach((node, index) => {
    const ring = Math.floor(index / 14);
    const inRing = Math.min(episodes.length - ring * 14, 14);
    const start = leftSide ? Math.PI * .55 : -Math.PI * .55;
    const sweep = leftSide ? Math.PI * 1.1 : -Math.PI * 1.1;
    const angle = start + (index % 14) * (sweep / Math.max(inRing - 1, 1));
    const spread = 1 + ring * .26;
    result.set(node.id, {
      x: clamp(cx + Math.cos(angle) * rx * spread, 22, width - 22),
      y: clamp(cy + Math.sin(angle) * ry * spread, 22, height - 28),
    });
  });
}

function placeContextLane(result, nodes, width, height) {
  const columns = Math.min(Math.max(nodes.length, 1), 6);
  nodes.forEach((node, index) => {
    const row = Math.floor(index / columns);
    const column = index % columns;
    result.set(node.id, {
      x: width * ((column + 1) / (columns + 1)),
      y: clamp(height - 42 - row * 54, 30, height - 28),
    });
  });
}
function showGraphDetail(item, type) {
  const card = $("#graph-detail");
  card.replaceChildren();
  const head = create("div", "detail-head");
  const titleWrap = create("div");
  const kind = graphDetailKind(type, item);
  titleWrap.append(create("span", "detail-kind", String(kind || "").toUpperCase()));
  titleWrap.append(create("h3", "detail-title", item.title || item.content || item.id || item.kind));
  head.append(titleWrap);
  if (item.status) head.append(create("span", "status-chip", item.status));
  card.append(head);

  const props = type === "episode" ? {
    status: item.status || "active",
    person_ids: item.person_ids,
    session_id: item.session_id,
    why: item.why,
    content: item.content,
    invalid_reason: item.invalid_reason,
    created_at: item.created_at,
  } : (item.properties || {
    id: item.id, subtitle: item.subtitle, status: item.status,
  });
  const grid = create("dl", "property-grid");
  for (const [key, value] of Object.entries(props)) {
    if (value === "" || value === null || value === undefined || (Array.isArray(value) && value.length === 0)) continue;
    const text = printable(value);
    const wrap = create("div", text.includes("\n") ? "property wide" : "property");
    wrap.append(create("dt", "", key.replaceAll("_", " ")));
    wrap.append(create("dd", "", text));
    grid.append(wrap);
  }
  card.append(grid);
}

function graphDetailKind(type, item) {
  if (type === "edge") return item.kind;
  if (type === "episode") return `EPISODE · ${item.kind || "event"}`;
  return item.kind;
}

function renderEpisodes() {
  const list = $("#episode-list");
  list.replaceChildren();
  const filtered = state.episodes.filter((ep) => state.episodeFilter === "all" || (ep.status || "active") === state.episodeFilter);
  $("#episode-count").textContent = `${filtered.length} 条经历`;
  if (!filtered.length) {
    list.append(emptyList("这一层目前没有内容。"));
    return;
  }
  for (const ep of filtered) {
    const status = ep.status || "active";
    const item = create("article", "memory-item");
    const top = create("div", "item-top");
    top.append(create("span", "item-kind", ep.kind || "event"));
    top.append(create("time", "item-time", formatDate(ep.created_at)));
    const content = create("div", "item-content", ep.content);
    const meta = create("div", "item-meta");
    meta.append(create("span", status, status));
    for (const person of ep.person_ids || []) {
      meta.append(create("span", person === "self" ? "active" : "", person === "self" ? "关于自己" : person));
    }
    if (ep.why) meta.append(create("span", "", `why: ${ep.why}`));
    item.append(top, content, meta);
    item.addEventListener("click", () => {
      selectView("graph");
      showGraphDetail(ep, "episode");
    });
    list.append(item);
  }
}

function renderProposals() {
  const list = $("#proposal-list");
  list.replaceChildren();
  if (!state.proposals.length) {
    list.append(emptyList("没有悬而未决的长期理解。"));
    return;
  }
  for (const proposal of state.proposals) {
    const item = create("article", "memory-item");
    const top = create("div", "item-top");
    top.append(create("span", "item-kind", `${proposal.field} · ${proposal.hypothesis || "none"}`));
    top.append(create("time", "item-time", formatDate(proposal.created_at)));
    item.append(top, create("div", "item-content", proposal.suggested_text));
    const meta = create("div", "item-meta");
    meta.append(create("span", "", proposal.mode || "append"));
    meta.append(create("span", "", proposal.source || "unknown"));
    item.append(meta);
    list.append(item);
  }
}

function renderMutations() {
  const list = $("#mutation-list");
  list.replaceChildren();
  if (!state.mutations.length) {
    list.append(emptyList("还没有发生过需要写入变化史的慢变。"));
    return;
  }
  for (const mutation of state.mutations) {
    const item = create("article", "memory-item");
    const top = create("div", "item-top");
    top.append(create("span", "item-kind", `${mutation.kind} · ${mutation.field || "state"}`));
    top.append(create("time", "item-time", formatDate(mutation.timestamp)));
    item.append(top, create("div", "item-content", mutation.reason_summary || "没有额外理由摘要"));
    const diff = create("div", "diff-grid");
    diff.append(create("div", "diff-box", printable(mutation.before || {})));
    diff.append(create("div", "diff-arrow", "→"));
    diff.append(create("div", "diff-box", printable(mutation.after || {})));
    item.append(diff);
    const meta = create("div", "item-meta");
    if (mutation.actor) meta.append(create("span", "", `actor: ${mutation.actor}`));
    if (mutation.model_version) meta.append(create("span", "", mutation.model_version));
    if (mutation.dream_id) meta.append(create("span", "", mutation.dream_id));
    item.append(meta);
    list.append(item);
  }
}

function renderTraces() {
  const list = $("#trace-list");
  list.replaceChildren();
  if (!state.traces.length) {
    list.append(emptyList("这里还没有工具和召回轨迹。"));
    return;
  }
  for (const trace of state.traces) {
    const item = create("article", "memory-item");
    const top = create("div", "item-top");
    top.append(create("span", "item-kind", trace.session_id || "turn"));
    top.append(create("time", "item-time", formatDate(trace.timestamp)));
    item.append(top, create("div", "item-content", trace.user_text || "未记录用户文本"));
    const meta = create("div", "item-meta");
    const searches = trace.recall_searches || [];
    const candidateIDs = new Set(searches.flatMap((search) => search.result_ids || []));
    const reads = (trace.recall_reads || []).filter((read) => read.episode_id && !read.error);
    const used = (trace.recall_evidence || []).filter((event) => event.status === "used");
    const dismissed = (trace.recall_evidence || []).filter((event) => event.status === "dismissed");
    const contextCandidates = trace.context_candidates || [];
    const contextIDs = new Set(contextCandidates.flatMap((event) => event.result_ids || []));
    const contextReads = (trace.context_reads || []).filter((event) => event.id && event.ok && !event.error);
    const contextUses = trace.context_uses || [];
    const contextUsed = contextUses.filter((event) => event.disposition === "used");
    const contextDismissed = contextUses.filter((event) => event.disposition === "dismissed");
    const contextFocused = contextUses.filter((event) => event.disposition === "focus");
    const contextSources = new Set(contextCandidates.map((event) => event.source).filter(Boolean));
    const hasUnifiedContext = contextCandidates.length || contextReads.length || contextUses.length;
    const taskContext = trace.task_context;
    const contextExpands = trace.task_context_expansions || [];
    const contextFocus = trace.task_context_focus;
    const attentionItems = trace.attention?.items || [];
    const attentionDecisions = trace.attention_decisions || [];
    const attentionState = new Map(attentionItems.map((entry) => [entry.id, entry.tier]));
    for (const event of attentionDecisions) {
      if (event.replaced_id) attentionState.delete(event.replaced_id);
      if (event.to === "drop") attentionState.delete(event.id);
      else if (event.id && event.to) attentionState.set(event.id, event.to);
    }

    if (taskContext) {
      const focused = taskContext.focus_workspace_id || taskContext.focus_intent_id;
      meta.append(create("span", "", `处境 · Workspace ${(taskContext.workspace_ids || []).length} / Intent ${(taskContext.intent_ids || []).length}${focused ? ` / 焦点 ${focused}` : ""}`));
    }
    if (contextExpands.length) {
      const lists = contextExpands.filter((event) => event.operation === "list").length;
      const reads = contextExpands.filter((event) => !event.operation || event.operation === "read").length;
      meta.append(create("span", "", `处境调查 · 列表 ${lists} / 展开 ${reads}`));
    }
    if (contextFocus) {
      const selected = contextFocus.workspace_id || contextFocus.intent_id || "未选择";
      const label = contextFocus.action === "clarify" ? "需要确认" : selected;
      meta.append(create("span", "", `处境结论 · ${contextFocus.action} / ${label}`));
    }
    if (hasUnifiedContext) {
      meta.append(create("span", "", `上下文 · ${[...contextSources].map(friendlyContextSource).join(" + ")} · 候选 ${contextIDs.size} / 已读 ${contextReads.length} / 采用 ${contextUsed.length} / 焦点 ${contextFocused.length} / 排除 ${contextDismissed.length}`));
    } else if (searches.length) {
      meta.append(create("span", "", `召回 · 候选 ${candidateIDs.size} / 已读 ${reads.length} / 采用 ${used.length} / 排除 ${dismissed.length}`));
    }
    if (attentionItems.length || attentionDecisions.length) {
      const center = [...attentionState.values()].filter((tier) => tier === "center").length;
      const support = [...attentionState.values()].filter((tier) => tier === "support").length;
      const periphery = [...attentionState.values()].filter((tier) => tier === "periphery").length;
      meta.append(create("span", "", `注意 · 中心 ${center} / 支撑 ${support} / 外围 ${periphery} / 调整 ${attentionDecisions.length}`));
    }

    for (const id of trace.recall_ids || []) meta.append(create("span", "", `recall · ${id}`));
    for (const tool of trace.tool_starts || []) meta.append(create("span", "", `tool · ${friendlyTool(tool)}`));
    if (!hasUnifiedContext && !searches.length && !attentionItems.length && !attentionDecisions.length && !(trace.recall_ids || []).length && !(trace.tool_starts || []).length) meta.append(create("span", "", "没有调用工具"));
    item.append(meta);
    if (trace.answer_preview) {
      item.append(create("div", "trace-answer", `回应 · ${trace.answer_preview}`));
    }
    if (hasUnifiedContext || searches.length || reads.length || used.length || dismissed.length || attentionItems.length || attentionDecisions.length) {
      item.classList.add("trace-replay");
      item.tabIndex = 0;
      item.title = "点击在图中回放本轮召回候选";
      item.addEventListener("click", () => replayRecallTrace(trace));
      item.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") replayRecallTrace(trace);
      });
    }
    list.append(item);
  }
}

async function runReview() {
  setActivity("正在提供一次安静回看的机会");
  try {
    const result = await postJSON("/api/review", {});
    toast(result.skipped ? `回看：${result.reason || "No change"}` : `回看留下了 ${result.proposal_ids?.length || 0} 条提案`);
    await refreshMemory();
  } catch (error) {
    toast(`回看失败：${error.message}`, true);
  } finally {
    clearActivity();
  }
}

async function runDream() {
  try {
    const result = await postJSON("/api/dream", {});
    toast(result.skipped ? `Dream：${result.reason || "No change"}` : `Dream 接纳了 ${result.accepted?.length || 0} 条变化`);
    await refreshMemory();
  } catch (error) {
    toast(`Dream 失败：${error.message}`, true);
  }
}

async function runBackup() {
  try {
    const result = await postJSON("/api/backup", {});
    toast(`已备份到 ${result.path}`);
  } catch (error) {
    toast(`备份失败：${error.message}`, true);
  }
}

async function readNDJSON(stream, onEvent) {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const lines = buffer.split("\n");
    buffer = lines.pop() || "";
    for (const line of lines) {
      if (!line.trim()) continue;
      let eventData;
      try {
        eventData = JSON.parse(line);
      } catch (error) {
        throw new Error(`流式解析失败：${error.message}`);
      }
      onEvent(eventData);
    }
    if (done) break;
  }
  if (buffer.trim()) {
    try {
      onEvent(JSON.parse(buffer));
    } catch (error) {
      throw new Error(`流式解析失败：${error.message}`);
    }
  }
}

async function getJSON(url) {
  const response = await fetch(url);
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
  return data;
}

async function postJSON(url, body) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
  return data;
}

function create(tag, className = "", text = "") {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== undefined && text !== null) element.textContent = String(text);
  return element;
}

function svgEl(tag, attrs = {}) {
  const element = document.createElementNS(svgNS, tag);
  for (const [key, value] of Object.entries(attrs)) element.setAttribute(key, String(value));
  return element;
}

function emptyList(text) { return create("div", "empty-list", text); }
function truncate(text, size) {
  const chars = [...String(text || "")];
  return chars.length > size ? `${chars.slice(0, size).join("")}…` : chars.join("");
}
function printable(value) {
  if (Array.isArray(value)) return value.join("\n");
  if (typeof value === "object" && value !== null) {
    return Object.entries(value).map(([key, val]) => `${key}: ${printable(val)}`).join("\n");
  }
  return String(value);
}
function formatDate(value) {
  if (!value) return "时间未记";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(date);
}
function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit" }).format(date);
}
function updateClock() { $("#room-time").textContent = formatTime(new Date().toISOString()); }
function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
function friendlyTool(name) {
  return String(name || "tool").replace(/^.*\//, "").replaceAll("_", " ");
}
function debounce(fn, delay) {
  let timer;
  return (...args) => {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => fn(...args), delay);
  };
}
function toast(message, error = false) {
  const item = create("div", `toast ${error ? "error" : ""}`, message);
  $("#toast-region").append(item);
  window.setTimeout(() => item.remove(), 5200);
}
