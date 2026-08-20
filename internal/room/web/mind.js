const mindState = {
  runtime: null,
  graph: { nodes: [], edges: [], available: false },
  episodes: [],
  artifacts: [],
  workspace: [],
  intents: [],
  wakes: [],
  sources: [],
  traces: [],
  mutations: [],
  proposals: [],
  agency: null,
  activities: [],
  activityFilter: "all",
  graphPinned: new Map(),
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];

document.addEventListener("DOMContentLoaded", async () => {
  bindMindUI();
  await refreshMind();
  updateMindClock();
  window.setInterval(updateMindClock, 30_000);
});

function bindMindUI() {
  $$(".mind-tabs button").forEach((button) => {
    button.addEventListener("click", () => selectMindView(button.dataset.mindView));
  });
  $$(".activity-filters button").forEach((button) => {
    button.addEventListener("click", () => {
      mindState.activityFilter = button.dataset.activityFilter;
      $$(".activity-filters button").forEach((item) => item.classList.toggle("active", item === button));
      renderActivityTimeline();
    });
  });
  $("#mind-refresh").addEventListener("click", refreshMind);
  window.addEventListener("resize", debounce(renderMindGraph, 120));
}

async function refreshMind() {
  const button = $("#mind-refresh");
  button.disabled = true;
  try {
    const [runtime, graph, episodes, self, workspace, intents, wakes, agency, sources, traces, mutations, proposals] = await Promise.all([
      getJSON("/api/runtime"),
      getJSON("/api/graph?limit=150"),
      getJSON("/api/episodes?limit=150&all=1"),
      getJSON("/api/self?limit=150"),
      getJSON("/api/workspace?limit=150"),
      getJSON("/api/intents?limit=150"),
      getJSON("/api/wakes?limit=200"),
      getJSON("/api/agency"),
      getJSON("/api/sources?limit=150"),
      getJSON("/api/traces?limit=200"),
      getJSON("/api/mutations?limit=150"),
      getJSON("/api/proposals?limit=150"),
    ]);
    mindState.runtime = runtime.runtime;
    mindState.graph = graph;
    mindState.episodes = episodes.episodes || [];
    mindState.artifacts = self.artifacts || [];
    mindState.workspace = workspace.documents || [];
    mindState.intents = intents.intents || [];
    mindState.wakes = wakes.wakes || [];
    mindState.agency = agency;
    mindState.sources = sources.sources || [];
    mindState.traces = traces.traces || [];
    mindState.mutations = mutations.mutations || [];
    mindState.proposals = proposals.proposals || [];
    mindState.activities = buildActivities();
    renderRuntime();
    renderSummary();
    renderActivityTimeline();
    renderMemory();
    renderThinking();
    renderAgency();
    renderSources();
  } catch (error) {
    toast(`心智活动读取失败：${error.message}`, true);
  } finally {
    button.disabled = false;
  }
}

function selectMindView(name) {
  $$(".mind-tabs button").forEach((button) => {
    button.classList.toggle("active", button.dataset.mindView === name);
  });
  $$(".mind-view").forEach((view) => view.classList.toggle("active", view.id === `mind-view-${name}`));
  if (name === "memory") window.requestAnimationFrame(renderMindGraph);
}

function renderRuntime() {
  const runtime = mindState.runtime;
  if (!runtime) return;
  const sourceAvailable = runtime.stores?.source_store === "available";
  $("#runtime-text").textContent = `${runtime.current_person} · ${runtime.model} · ${sourceAvailable ? "World 已连接" : "World 不可用"}`;
  $("#runtime-dot").classList.toggle("offline", !sourceAvailable);
  $("#mind-time").textContent = formatTime(runtime.now);
}

function renderSummary() {
  const openWorkspace = mindState.workspace.filter((item) => !["done", "archived"].includes(item.status)).length;
  $("#workspace-stat").textContent = String(openWorkspace);
  $("#self-stat").textContent = String(mindState.artifacts.filter((item) => item.status !== "deprecated").length);
  $("#wake-stat").textContent = String(mindState.wakes.length);
  $("#source-stat").textContent = String(mindState.sources.length);
}

function buildActivities() {
  return [
    ...mindState.traces.map(traceActivity),
    ...mindState.workspace.flatMap(workspaceActivities),
    ...mindState.artifacts.flatMap(selfActivities),
    ...mindState.intents.map(intentActivity),
    ...mindState.wakes.map(wakeActivity),
    ...mindState.sources.map(sourceActivity),
    ...mindState.proposals.map(proposalActivity),
    ...mindState.mutations.map(mutationActivity),
  ].filter((item) => item.time)
    .sort((left, right) => new Date(right.time) - new Date(left.time));
}

function traceActivity(trace) {
  const auto = String(trace.session_id || "").startsWith("auto:");
  return {
    id: `trace:${trace.timestamp}:${trace.session_id || "turn"}`,
    category: auto ? "autonomy" : "conversation",
    kind: auto ? "autonomous turn" : "conversation turn",
    title: auto ? `自主会话 ${trace.session_id}` : (trace.user_text || "完成一轮沟通"),
    summary: summarizeTrace(trace),
    time: trace.timestamp,
    data: trace,
    objectType: "trace",
  };
}

function workspaceActivities(document) {
  const revisions = document.revisions?.length ? document.revisions : [{ at: document.updated_at, summary: "更新思考" }];
  return revisions.map((revision) => ({
    id: `workspace:${document.id}:${revision.at}`,
    category: "thinking",
    kind: `workspace · ${document.type}`,
    title: document.title || "未命名思考",
    summary: revision.summary || document.summary,
    time: revision.at || document.updated_at,
    data: document,
    objectType: "workspace",
  }));
}

function selfActivities(artifact) {
  const revisions = artifact.revisions?.length ? artifact.revisions : [{ at: artifact.updated_at, summary: "更新自我理解" }];
  return revisions.map((revision) => ({
    id: `self:${artifact.id}:${revision.at}`,
    category: "thinking",
    kind: `self · ${artifact.type}`,
    title: artifact.title || "未命名自我理解",
    summary: revision.summary || artifact.summary,
    time: revision.at || artifact.updated_at,
    data: artifact,
    objectType: "self",
  }));
}

function intentActivity(intent) {
  return {
    id: `intent:${intent.id}`,
    category: "autonomy",
    kind: `intent · ${intent.kind}`,
    title: intent.title,
    summary: `约定在 ${formatDate(intent.due_at)} 唤醒 · 已尝试 ${intent.attempt || 0} 次`,
    time: intent.created_at,
    data: intent,
    objectType: "intent",
  };
}

function wakeActivity(wake) {
  return {
    id: `wake:${wake.id}`,
    category: "autonomy",
    kind: `wake · ${wake.status}`,
    title: wakeTitle(wake),
    summary: wake.notes || `${wake.decision || "未记录决策"} · ${wake.session_id || "无会话"}`,
    time: wake.finished_at || wake.started_at || wake.created_at,
    data: wake,
    objectType: "wake",
  };
}

function sourceActivity(source) {
  return {
    id: `source:${source.id}`,
    category: "world",
    kind: source.query ? "web search" : "webpage read",
    title: source.query ? `搜索：${source.query}` : (source.title || source.url),
    summary: source.excerpt || source.url,
    time: source.fetched_at,
    data: source,
    objectType: "source",
  };
}

function proposalActivity(proposal) {
  return {
    id: `proposal:${proposal.id}`,
    category: "thinking",
    kind: `proposal · ${proposal.kind || "bond"}`,
    title: proposal.field || "提出一项长期理解更新",
    summary: proposal.suggested_text || proposal.rationale,
    time: proposal.created_at,
    data: proposal,
    objectType: "proposal",
  };
}

function mutationActivity(mutation) {
  return {
    id: `mutation:${mutation.mutation_id}`,
    category: "thinking",
    kind: `mutation · ${mutation.kind}`,
    title: mutation.field || "长期理解发生变化",
    summary: mutation.reason_summary || `${mutation.actor || "system"} 写入 Mutation Ledger`,
    time: mutation.timestamp,
    data: mutation,
    objectType: "mutation",
  };
}

function wakeTitle(wake) {
  if (wake.result === "acted") return "自主运行并采取行动";
  if (wake.result === "noop") return "自主唤醒后选择 noop";
  return `自主唤醒：${wake.result || wake.status}`;
}

function summarizeTrace(trace) {
  const parts = [];
  if (trace.recall_ids?.length) parts.push(`召回 ${trace.recall_ids.length} 条`);
  if (trace.tool_starts?.length) parts.push(`调用 ${trace.tool_starts.map(friendlyTool).join("、")}`);
  if (trace.errors?.length) parts.push(`发生 ${trace.errors.length} 个错误`);
  if (trace.answer_preview) parts.push(`回应：${truncate(trace.answer_preview, 90)}`);
  return parts.join(" · ") || "没有调用工具或写入";
}

function renderActivityTimeline() {
  const timeline = $("#activity-timeline");
  timeline.replaceChildren();
  const items = mindState.activityFilter === "all"
    ? mindState.activities
    : mindState.activities.filter((item) => item.category === mindState.activityFilter);
  $("#activity-count").textContent = `${items.length} 条记录`;
  if (!items.length) {
    timeline.append(emptyMind("这个范围内还没有活动。"));
    return;
  }
  for (const activity of items) {
    const button = create("button", "activity-item");
    button.type = "button";
    button.dataset.category = activity.category;
    const top = create("div", "activity-item-top");
    top.append(create("span", "activity-kind", activity.kind), create("time", "", formatDate(activity.time)));
    button.append(top, create("strong", "", activity.title), create("p", "", activity.summary || "没有摘要"));
    button.addEventListener("click", async () => {
      $$(".activity-item").forEach((item) => item.classList.toggle("selected", item === button));
      await renderObjectDetail($("#activity-detail"), activity.objectType, activity.data);
    });
    timeline.append(button);
  }
}

function renderMemory() {
  renderMindGraph();
  const list = $("#mind-episode-list");
  list.replaceChildren();
  $("#mind-episode-count").textContent = `${mindState.episodes.length} 条`;
  for (const episode of mindState.episodes) {
    const card = mindCard(
      episode,
      `episode · ${episode.kind || "event"}`,
      truncate(episode.content || episode.id, 70),
      episode.why || episode.session_id,
      episode.created_at,
      [episode.status || "active", ...(episode.person_ids || [])],
      () => renderMemoryDetail(episode, "episode"),
    );
    list.append(card);
  }
  if (!mindState.episodes.length) list.append(emptyMind("还没有 Episode。"));
}

function renderMindGraph() {
  const svg = $("#memory-graph");
  if (!svg || !$("#mind-view-memory").classList.contains("active")) return;
  svg.replaceChildren();
  const nodes = mindState.graph.nodes || [];
  const edges = mindState.graph.edges || [];
  $("#mind-graph-summary").textContent = mindState.graph.available
    ? `${nodes.length} 节点 · ${edges.length} 关系`
    : "Neo4j 暂不可用";
  $("#graph-empty").hidden = mindState.graph.available && nodes.length > 2;
  if (!mindState.graph.available || !nodes.length) return;

  const rect = svg.getBoundingClientRect();
  const width = Math.max(rect.width, 420);
  const height = Math.max(rect.height, 300);
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
  const layout = semanticLayout(nodes, width, height);
  appendGraphArrow(svg);
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
      d: geometry.d, class: `graph-edge ${edge.kind}`, "marker-end": "url(#arrow)",
    });
    // transparent wide path so thin relationships stay clickable
    const hit = svgEl("path", { d: geometry.d, class: "graph-edge-hit" });
    hit.addEventListener("click", () => renderMemoryDetail(edge, "edge"));
    const ref = { edge, lines: [line, hit], label: null, curvature };
    edgeLayer.append(line, hit);
    if (["BOND", "KNOWS", "CALLS"].includes(edge.kind)) {
      const label = svgEl("text", { x: geometry.labelX, y: geometry.labelY, class: "edge-label" });
      label.textContent = edge.kind;
      label.addEventListener("click", () => renderMemoryDetail(edge, "edge"));
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
    const position = layout.get(node.id);
    if (!position) continue;
    const isAnchor = ["Self", "Person"].includes(node.kind);
    const radius = isAnchor ? 27 : 12;
    const group = svgEl("g", { class: "node-group", transform: `translate(${position.x} ${position.y})`, tabindex: "0" });
    group.append(svgEl("circle", { r: radius + 4, class: "node-halo" }));
    group.append(svgEl("circle", { r: radius, class: `node-core ${node.kind} ${node.status || ""} ${node.anchor === "Self" ? "about-self" : ""}` }));
    const label = svgEl("text", { y: isAnchor ? 4 : radius + 14, class: "node-label" });
    label.textContent = truncate(node.title, isAnchor ? 13 : 16);
    group.append(label);
    group.addEventListener("click", () => {
      if (group.dataset.dragged === "1") {
        delete group.dataset.dragged;
        return;
      }
      openGraphNode(node, group);
    });
    group.addEventListener("dblclick", () => {
      mindState.graphPinned.delete(node.id);
      renderMindGraph();
    });
    enableNodeDrag(svg, group, node, layout, edgeRefs, bounds, mindState.graphPinned);
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

function appendGraphArrow(svg) {
  const defs = svgEl("defs");
  const marker = svgEl("marker", {
    id: "arrow", viewBox: "0 0 10 10", refX: "8", refY: "5",
    markerWidth: "5", markerHeight: "5", orient: "auto-start-reverse",
  });
  marker.append(svgEl("path", { d: "M 0 0 L 10 5 L 0 10 z", fill: "rgba(122,112,94,.4)" }));
  defs.append(marker);
  svg.append(defs);
}

async function openGraphNode(node, group) {
  $$(".node-group").forEach((item) => item.classList.toggle("selected", item === group));
  if (node.kind !== "Episode") {
    renderMemoryDetail(node, "node");
    return;
  }
  try {
    const episode = await getJSON(`/api/episode/${encodeURIComponent(node.id)}`);
    renderMemoryDetail(episode, "episode");
  } catch {
    renderMemoryDetail(node, "node");
  }
}

function semanticLayout(nodes, width, height) {
  const result = new Map();
  const self = nodes.find((node) => node.kind === "Self");
  const person = nodes.find((node) => node.kind === "Person");
  const selfPosition = { x: width * .28, y: height * .48 };
  const personPosition = { x: width * .72, y: height * .48 };
  if (self) result.set(self.id, selfPosition);
  if (person) result.set(person.id, personPosition);
  placeEpisodeRing(result, nodes.filter((node) => node.kind === "Episode" && node.anchor === "Self"), selfPosition, width, height, true);
  placeEpisodeRing(result, nodes.filter((node) => node.kind === "Episode" && node.anchor !== "Self"), personPosition, width, height, false);
  for (const [id, position] of mindState.graphPinned) {
    if (result.has(id)) result.set(id, position);
  }
  return result;
}

function placeEpisodeRing(result, episodes, center, width, height, leftSide) {
  const radiusX = Math.min(width * .22, 120);
  const radiusY = Math.min(height * .36, 118);
  episodes.forEach((node, index) => {
    const ring = Math.floor(index / 14);
    const count = Math.min(episodes.length - ring * 14, 14);
    const start = leftSide ? Math.PI * .55 : -Math.PI * .55;
    const sweep = leftSide ? Math.PI * 1.1 : -Math.PI * 1.1;
    const angle = start + (index % 14) * (sweep / Math.max(count - 1, 1));
    const spread = 1 + ring * .26;
    result.set(node.id, {
      x: clamp(center.x + Math.cos(angle) * radiusX * spread, 22, width - 22),
      y: clamp(center.y + Math.sin(angle) * radiusY * spread, 22, height - 28),
    });
  });
}

function renderMemoryDetail(item, type) {
  const target = $("#memory-detail");
  target.replaceChildren();
  const header = create("header", "detail-header");
  const title = type === "episode" ? truncate(item.content, 100) : (item.title || item.kind || item.id);
  header.append(
    create("span", "activity-kind", type === "episode" ? `Episode · ${item.kind || "event"}` : `${type} · ${item.kind || ""}`),
    create("h3", "", title || "记忆详情"),
    create("p", "", item.subtitle || item.why || item.status || ""),
  );
  target.append(header);
  if (type === "episode" && item.content) target.append(create("div", "detail-body", item.content));
  const properties = item.properties || {
    id: item.id,
    status: item.status,
    person_ids: item.person_ids,
    session_id: item.session_id,
    created_at: item.created_at,
    invalid_reason: item.invalid_reason,
  };
  const meta = create("dl", "detail-meta");
  for (const [label, value] of Object.entries(properties)) {
    if (value === "" || value === null || value === undefined) continue;
    const text = formatValue(value);
    const cell = create("div", text.includes("\n") ? "wide" : "");
    cell.append(create("dt", "", label.replaceAll("_", " ")), create("dd", "", text));
    meta.append(cell);
  }
  if (meta.children.length) target.append(meta);
}

function renderThinking() {
  $("#workspace-count").textContent = `${mindState.workspace.length} 项`;
  $("#self-count").textContent = `${mindState.artifacts.length} 项`;
  $("#workspace-list").replaceChildren(...mindState.workspace.map((document) => {
    return mindCard(document, `workspace · ${document.type}`, document.title, document.summary, document.updated_at, [
      document.status,
      `${document.revisions?.length || 0} 次修订`,
    ], () => renderObjectDetail($("#thinking-detail"), "workspace", document));
  }));
  $("#self-list").replaceChildren(...mindState.artifacts.map((artifact) => {
    return mindCard(artifact, `self · ${artifact.type}`, artifact.title, artifact.summary, artifact.updated_at, [
      artifact.status,
      `置信度 ${formatConfidence(artifact.confidence)}`,
    ], () => renderObjectDetail($("#thinking-detail"), "self", artifact));
  }));
  if (!mindState.workspace.length) $("#workspace-list").append(emptyMind("还没有 Workspace 思考。"));
  if (!mindState.artifacts.length) $("#self-list").append(emptyMind("还没有形成 SelfArtifact。"));
}

function renderAgency() {
  const scheduler = mindState.agency?.scheduler;
  const budget = mindState.agency?.budget;
  $("#scheduler-state").textContent = scheduler?.running ? "运行中" : "未启动";
  $("#scheduler-state").classList.toggle("active", Boolean(scheduler?.running));
  $("#scheduler-interval").textContent = scheduler ? `${scheduler.interval_seconds}s` : "—";
  $("#agency-budget").textContent = budget ? `${budget.remaining} / ${budget.max_per_day}` : "—";
  $("#outbound-state").textContent = budget?.allow_outbound ? "允许" : "关闭";
  $("#intent-count").textContent = `${mindState.intents.length} 项`;
  $("#wake-count").textContent = `${mindState.wakes.length} 次`;
  $("#intent-list").replaceChildren(...mindState.intents.map((intent) => {
    return mindCard(intent, `intent · ${intent.kind}`, intent.title, intent.body || `下次：${formatDate(intent.due_at)}`, intent.due_at, [
      intent.status,
      intent.allow_outbound ? "允许外联" : "禁止外联",
      `attempt ${intent.attempt || 0}`,
    ], () => renderObjectDetail($("#agency-detail"), "intent", intent));
  }));
  $("#wake-list").replaceChildren(...mindState.wakes.map((wake) => {
    return mindCard(wake, `wake · ${wake.status}`, wake.result || "等待结果", wake.notes || wake.decision, wake.finished_at || wake.created_at, [
      wake.trigger,
      wake.session_id || "无 session",
    ], () => renderObjectDetail($("#agency-detail"), "wake", wake));
  }));
  if (!mindState.intents.length) $("#intent-list").append(emptyMind("目前没有活跃 Intent。"));
  if (!mindState.wakes.length) $("#wake-list").append(emptyMind("还没有自主唤醒记录。"));
}

function renderSources() {
  $("#source-count").textContent = `${mindState.sources.length} 项`;
  $("#source-list").replaceChildren(...mindState.sources.map((source) => {
    const kind = source.query ? "web search" : "webpage";
    const title = source.query ? `搜索：${source.query}` : (source.title || source.url);
    return mindCard(source, kind, title, source.excerpt || source.url, source.fetched_at, [
      source.id,
      "不可信外部内容",
    ], () => renderObjectDetail($("#source-detail"), "source", source));
  }));
  if (!mindState.sources.length) $("#source-list").append(emptyMind("还没有搜索或读取网页。"));
}

function mindCard(data, kind, title, summary, time, tags, onClick) {
  const button = create("button", "mind-card");
  button.type = "button";
  const top = create("div", "mind-card-top");
  top.append(create("span", "card-kind", kind), create("time", "", formatDate(time)));
  const meta = create("div", "mind-card-meta");
  for (const tag of tags.filter(Boolean)) meta.append(create("span", "", tag));
  button.append(top, create("strong", "", title || "未命名"), create("p", "", summary || "没有摘要"), meta);
  button.addEventListener("click", () => {
    button.closest(".mind-view")?.querySelectorAll(".mind-card").forEach((item) => item.classList.toggle("selected", item === button));
    onClick(data);
  });
  return button;
}

async function renderObjectDetail(target, type, data) {
  target.replaceChildren();
  let full;
  try {
    full = await loadDetailObject(type, data);
  } catch (error) {
    target.append(emptyMind(`详情读取失败：${error.message}`));
    return;
  }

  const header = create("header", "detail-header");
  header.append(
    create("span", "activity-kind", detailKind(type, full)),
    create("h3", "", detailTitle(type, full)),
    create("p", "", detailSubtitle(type, full)),
  );
  target.append(header);

  const body = detailBody(type, full);
  if (body) target.append(create("div", `detail-body ${type === "source" ? "source-body" : ""}`, body));
  appendDetailMeta(target, type, full);
  appendDetailLinks(target, type, full);
  appendDetailRevisions(target, full);
}

async function loadDetailObject(type, data) {
  if (type === "source" && !data.body) {
    return getJSON(`/api/source/${encodeURIComponent(data.id)}`);
  }
  return data;
}

function appendDetailMeta(target, type, data) {
  const meta = create("dl", "detail-meta");
  for (const [label, value] of detailMeta(type, data)) {
    if (value === "" || value === null || value === undefined) continue;
    const cell = create("div");
    cell.append(create("dt", "", label), create("dd", "", value));
    meta.append(cell);
  }
  if (meta.children.length) target.append(meta);
}

function appendDetailLinks(target, type, data) {
  const links = detailLinks(type, data);
  if (links.length) {
    const wrap = create("div", "detail-links");
    for (const link of links) wrap.append(create("span", "", link));
    target.append(wrap);
  }
}

function appendDetailRevisions(target, data) {
  if (!data.revisions?.length) return;
  const revisions = create("section", "detail-revisions");
  revisions.append(create("h4", "", "修订记录"));
  for (const revision of [...data.revisions].reverse()) {
    const item = create("article");
    item.append(create("span", "", revision.summary || "更新"), create("time", "", `${formatDate(revision.at)} · ${revision.actor || "unknown"}`));
    revisions.append(item);
  }
  target.append(revisions);
}

function detailKind(type, data) {
  if (type === "workspace") return `Workspace · ${data.type}`;
  if (type === "self") return `SelfArtifact · ${data.type}`;
  if (type === "intent") return `Intent · ${data.kind}`;
  if (type === "wake") return `Wake · ${data.status}`;
  if (type === "source") return data.query ? "Web Search Source" : "Webpage Source";
  if (type === "trace") return `Turn Trace · ${data.session_id || "turn"}`;
  if (type === "proposal") return `Proposal · ${data.kind || "bond"}`;
  if (type === "mutation") return `Mutation · ${data.kind}`;
  return type;
}

function detailTitle(type, data) {
  if (type === "trace") return data.user_text || "结构化回合轨迹";
  if (type === "wake") return data.result || "自主唤醒";
  if (type === "source") return data.query ? `搜索：${data.query}` : (data.title || data.url);
  if (type === "proposal") return data.field || "长期理解提案";
  if (type === "mutation") return data.field || "长期理解变化";
  return data.title || data.id || "详情";
}

function detailSubtitle(type, data) {
  if (type === "trace") return data.answer_preview || "不包含隐藏推理，仅记录外部轨迹。";
  if (type === "wake") return data.notes || data.decision || "没有附加说明";
  if (type === "source") return data.url || "";
  if (type === "proposal") return data.rationale || data.source || "";
  if (type === "mutation") return data.reason_summary || `actor · ${data.actor || "unknown"}`;
  return data.summary || data.status || "";
}

function detailBody(type, data) {
  if (["workspace", "self", "intent"].includes(type)) return data.body || "";
  if (type === "source") return data.body || data.excerpt || "";
  if (type === "proposal") return data.suggested_text || "";
  if (type === "mutation") return formatValue({ before: data.before, after: data.after });
  if (type === "trace") return summarizeTrace(data);
  return "";
}

function detailMeta(type, data) {
  if (type === "workspace") return [
    ["ID", data.id], ["状态", data.status], ["创建", formatDate(data.created_at)], ["更新", formatDate(data.updated_at)],
  ];
  if (type === "self") return [
    ["ID", data.id], ["状态", data.status], ["置信度", formatConfidence(data.confidence)], ["更新", formatDate(data.updated_at)],
  ];
  if (type === "intent") return [
    ["ID", data.id], ["状态", data.status], ["触发", data.trigger], ["计划唤醒", formatDate(data.due_at)],
    ["尝试次数", data.attempt || 0], ["主动外联", data.allow_outbound ? "允许" : "关闭"],
  ];
  if (type === "wake") return [
    ["Wake ID", data.id], ["Intent", data.intent_id], ["计划时间", formatDate(data.scheduled_at)],
    ["开始", formatDate(data.started_at)], ["完成", formatDate(data.finished_at)], ["Session", data.session_id],
    ["决策", data.decision], ["结果", data.result],
  ];
  if (type === "source") return [
    ["Source ID", data.id], ["抓取时间", formatDate(data.fetched_at)], ["原始 URL", data.url],
    ["最终 URL", data.final_url], ["Content-Type", data.content_type],
  ];
  if (type === "trace") return [
    ["时间", formatDate(data.timestamp)], ["Session", data.session_id], ["模型", data.model_version],
    ["Toolset", data.toolset_version],
  ];
  if (type === "proposal") return [
    ["ID", data.id], ["状态", data.status], ["来源", data.source], ["创建", formatDate(data.created_at)],
  ];
  if (type === "mutation") return [
    ["ID", data.mutation_id], ["时间", formatDate(data.timestamp)], ["Actor", data.actor], ["Model", data.model_version],
  ];
  return [];
}

function detailLinks(type, data) {
  if (type === "workspace") return [
    ...(data.episode_ids || []).map((id) => `episode · ${id}`),
    ...(data.related_self_ids || []).map((id) => `self · ${id}`),
  ];
  if (type === "self") return [
    ...(data.source_episode_ids || []).map((id) => `evidence · ${id}`),
    ...(data.experience_modes || []).map((mode) => `mode · ${mode}`),
  ];
  if (type === "trace") return [
    ...(data.recall_ids || []).map((id) => `recall · ${id}`),
    ...(data.tool_starts || []).map((tool) => `tool · ${friendlyTool(tool)}`),
    ...(data.memory_writes || []).map((id) => `write · ${id}`),
  ];
  if (type === "mutation") return [
    ...(data.source_episode_ids || []).map((id) => `episode · ${id}`),
    ...(data.source_session_ids || []).map((id) => `session · ${id}`),
  ];
  return [];
}

async function getJSON(url) {
  const response = await fetch(url);
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

function svgEl(tag, attributes = {}) {
  const element = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [key, value] of Object.entries(attributes)) element.setAttribute(key, String(value));
  return element;
}

function emptyMind(text) {
  return create("div", "empty-mind", text);
}

function formatDate(value) {
  if (!value || String(value).startsWith("0001-")) return "时间未记";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat("zh-CN", {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  }).format(date);
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit" }).format(date);
}

function formatConfidence(value) {
  const number = Number(value);
  return Number.isFinite(number) ? `${Math.round(number * 100)}%` : "未记录";
}

function formatValue(value) {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
}

function truncate(value, size) {
  const chars = [...String(value || "")];
  return chars.length > size ? `${chars.slice(0, size).join("")}…` : chars.join("");
}

function friendlyTool(name) {
  return String(name || "tool").replace(/^.*\//, "").replaceAll("_", " ");
}

function clamp(value, minimum, maximum) {
  return Math.max(minimum, Math.min(maximum, value));
}

function debounce(callback, delay) {
  let timer;
  return (...args) => {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => callback(...args), delay);
  };
}

function updateMindClock() {
  $("#mind-time").textContent = formatTime(new Date().toISOString());
}

function toast(message, error = false) {
  const item = create("div", `toast ${error ? "error" : ""}`, message);
  $("#toast-region").append(item);
  window.setTimeout(() => item.remove(), 5200);
}
