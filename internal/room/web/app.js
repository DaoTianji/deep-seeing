const state = {
  busy: false,
  graph: { nodes: [], edges: [], available: false },
  episodes: [],
  proposals: [],
  mutations: [],
  traces: [],
  episodeFilter: "all",
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
  const graph = state.graph;
  const nodes = graph.nodes || [];
  const edges = graph.edges || [];
  $("#graph-summary").textContent = graph.available
    ? `${nodes.length} 个节点 · ${edges.length} 条关系 · 来自 Neo4j`
    : "Neo4j 暂不可用；Episode 文件仍然存在";
  $("#graph-empty").hidden = graph.available && nodes.length > 2;
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
  appendGraphEdges(svg, edges, layout);
  appendGraphNodes(svg, nodes, layout);
}

function appendGraphEdges(svg, edges, layout) {
  const edgeLayer = svgEl("g", { class: "edge-layer" });
  for (const edge of edges) {
    const source = layout.get(edge.source);
    const target = layout.get(edge.target);
    if (!source || !target) continue;
    const line = svgEl("line", {
      x1: source.x, y1: source.y, x2: target.x, y2: target.y,
      class: `graph-edge ${edge.kind}`, "marker-end": "url(#arrow)",
    });
    line.addEventListener("click", () => showGraphDetail(edge, "edge"));
    edgeLayer.append(line);
    if (["BOND", "KNOWS", "CALLS"].includes(edge.kind)) {
      const label = svgEl("text", {
        x: (source.x + target.x) / 2, y: (source.y + target.y) / 2 - 6, class: "edge-label",
      });
      label.textContent = edge.kind;
      label.addEventListener("click", () => showGraphDetail(edge, "edge"));
      edgeLayer.append(label);
    }
  }
  svg.append(edgeLayer);
}

function appendGraphNodes(svg, nodes, layout) {
  const nodeLayer = svgEl("g", { class: "node-layer" });
  for (const node of nodes) {
    const pos = layout.get(node.id);
    if (!pos) continue;
    const radius = node.kind === "Self" || node.kind === "Person" ? 27 : 12;
    const group = svgEl("g", { class: "node-group", transform: `translate(${pos.x} ${pos.y})`, tabindex: "0" });
    group.append(svgEl("circle", { r: radius + 4, class: "node-halo" }));
    group.append(svgEl("circle", { r: radius, class: `node-core ${node.kind} ${node.status || ""} ${node.anchor === "Self" ? "about-self" : ""}` }));
    const label = svgEl("text", { y: node.kind === "Episode" ? radius + 14 : 4, class: "node-label" });
    label.textContent = truncate(node.title, node.kind === "Episode" ? 16 : 13);
    group.append(label);
    if (node.kind !== "Episode") {
      const sub = svgEl("text", { y: radius + 14, class: "node-sub" });
      sub.textContent = node.kind;
      group.append(sub);
    }
    group.addEventListener("click", async () => {
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
    nodeLayer.append(group);
  }
  svg.append(nodeLayer);
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
    const wrap = create("div", "property");
    wrap.append(create("dt", "", key.replaceAll("_", " ")));
    wrap.append(create("dd", "", printable(value)));
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
    for (const id of trace.recall_ids || []) meta.append(create("span", "", `recall · ${id}`));
    for (const tool of trace.tool_starts || []) meta.append(create("span", "", `tool · ${friendlyTool(tool)}`));
    if (!(trace.recall_ids || []).length && !(trace.tool_starts || []).length) meta.append(create("span", "", "没有调用工具"));
    item.append(meta);
    if (trace.answer_preview) {
      item.append(create("div", "trace-answer", `回应 · ${trace.answer_preview}`));
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
