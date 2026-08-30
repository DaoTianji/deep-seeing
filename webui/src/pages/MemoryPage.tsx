import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { BookOpen, GitBranch, Search, X } from "lucide-react";
import { useMemo, useState } from "react";
import { api } from "../api";
import { MemoryGraph } from "../components/MemoryGraph";
import { evidenceFromEvents, traceToEvents } from "../turn";
import type { EvidenceState, GraphNode, GraphView } from "../types";

function text(item: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) if (item[key]) return String(item[key]);
  return "未命名";
}
function date(item: Record<string, unknown>) {
  const raw = item.created_at || item.updated_at || item.timestamp;
  return raw ? new Date(String(raw)).toLocaleDateString("zh-CN") : "";
}

export function MemoryPage() {
  const [view, setView] = useState<"map" | "archive">("map");
  const [query, setQuery] = useState("");
  const [graphStatus, setGraphStatus] = useState("all");
  const [overlayTurnID, setOverlayTurnID] = useState("");
  const [selected, setSelected] = useState<GraphNode>();
  const [selectedEpisodeID, setSelectedEpisodeID] = useState("");
  const graph = useQuery({ queryKey: ["graph"], queryFn: api.graph });
  const turns = useQuery({ queryKey: ["turns"], queryFn: api.turns });
  const episodes = useInfiniteQuery({
    queryKey: ["episodes"], initialPageParam: "",
    queryFn: ({ pageParam }) => api.episodes(pageParam),
    getNextPageParam: (lastPage) => lastPage.next_cursor || undefined,
  });
  const episodeItems = episodes.data?.pages.flatMap((page) => page.episodes) || [];
  const episode = useQuery({ queryKey: ["episode", selectedEpisodeID], queryFn: () => api.episode(selectedEpisodeID), enabled: !!selectedEpisodeID });
  const proposals = useQuery({ queryKey: ["proposals"], queryFn: api.proposals });
  const mutations = useQuery({ queryKey: ["mutations"], queryFn: api.mutations });
  const graphView = useMemo<GraphView | undefined>(() => {
    if (!graph.data || graphStatus === "all") return graph.data;
    const nodes = graph.data.nodes.filter((node) => node.kind !== "Episode" || node.status === graphStatus);
    const ids = new Set(nodes.map((node) => node.id));
    return { ...graph.data, nodes, edges: graph.data.edges.filter((edge) => ids.has(edge.source) && ids.has(edge.target)) };
  }, [graph.data, graphStatus]);
  const activation = useMemo<Map<string, EvidenceState>>(() => {
    const trace = turns.data?.turns.find((item) => (item.turn_id || item.timestamp) === overlayTurnID);
    return trace ? evidenceFromEvents(traceToEvents(trace)) : new Map();
  }, [turns.data, overlayTurnID]);
  const filtered = useMemo(() => episodeItems.filter((item) => {
    const target = `${text(item, "title", "summary", "id")} ${text(item, "summary", "content")}`.toLowerCase();
    return target.includes(query.trim().toLowerCase());
  }), [episodeItems, query]);
  const detail = episode.data;
  return (
    <div className="memory-page">
      <header className="page-heading wide-heading">
        <div><span className="eyebrow">LONG-TERM MEMORY</span><h1>记忆留下的形状</h1><p>星图看关系，档案看内容。临时激活永远不会伪装成持久关系。</p></div>
        <div className="view-switch" role="tablist" aria-label="记忆视图">
          <button role="tab" aria-selected={view === "map"} className={view === "map" ? "active" : ""} onClick={() => setView("map")}><GitBranch size={15} />星图</button>
          <button role="tab" aria-selected={view === "archive"} className={view === "archive" ? "active" : ""} onClick={() => setView("archive")}><BookOpen size={15} />档案</button>
        </div>
      </header>
      {view === "map" ? (
        <div className="map-layout">
          <section className="map-card">
            <div className="map-toolbar">
              <div><strong>长期结构</strong><span>{graphView?.nodes?.length || 0} 个节点 · {graphView?.edges?.length || 0} 条真实关系</span></div>
              <div className="graph-filter-group"><label className="graph-filter"><span>状态</span><select aria-label="图谱经历状态" value={graphStatus} onChange={(event) => setGraphStatus(event.target.value)}><option value="all">全部</option><option value="active">现行</option><option value="archived">归档</option><option value="invalid">失效</option></select></label><label className="graph-filter"><span>本轮覆盖</span><select aria-label="叠加历史 Turn" value={overlayTurnID} onChange={(event) => setOverlayTurnID(event.target.value)}><option value="">关闭</option>{(turns.data?.turns || []).map((turn) => <option key={turn.turn_id || turn.timestamp} value={turn.turn_id || turn.timestamp}>{turn.user_text || turn.turn_id}</option>)}</select></label></div>
            </div>
            <MemoryGraph view={graphView} activation={activation} onSelect={setSelected} />
            <div className="map-legend"><span><i className="self" />Self</span><span><i className="person" />Person</span><span><i className="episode" />Episode</span><span><i className="role-node" />Role</span><span><i className="worldline-node" />Worldline</span>{overlayTurnID && <><span><i className="candidate" />候选</span><span><i className="used" />采用</span></>}<em>{overlayTurnID ? "覆盖层只高亮真实节点" : "实线仅表示持久关系"}</em></div>
          </section>
          <aside className="object-inspector">
            {!selected ? <div className="inspector-empty"><span>⟡</span><strong>选择一个记忆节点</strong><p>查看它是什么、从何而来，以及现在处于什么状态。</p></div> : <>
              <header><div><span className="eyebrow">{selected.kind || selected.type}</span><h2>{selected.title || selected.label || selected.id}</h2></div><button onClick={() => setSelected(undefined)} aria-label="关闭详情"><X size={16} /></button></header>
              {selected.subtitle ? <p className="object-subtitle">{String(selected.subtitle)}</p> : null}
              <dl className="property-list"><div><dt>ID</dt><dd>{selected.id}</dd></div>{Object.entries((selected.properties || {}) as Record<string, unknown>).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{typeof value === "string" ? value : JSON.stringify(value)}</dd></div>)}</dl>
            </>}
          </aside>
        </div>
      ) : (
        <div className="archive-layout">
          <section className="archive-main">
            <div className="archive-toolbar"><label><Search size={16} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="寻找一段经历…" /></label><span>{filtered.length} 条可见经历</span></div>
            <div className="episode-grid">
              {filtered.map((item) => <button className={selectedEpisodeID === text(item, "id") ? "episode-card selected" : "episode-card"} key={text(item, "id")} onClick={() => setSelectedEpisodeID(text(item, "id"))}><header><span>{text(item, "kind", "type")}</span><time>{date(item)}</time></header><h3>{text(item, "content", "summary", "title", "id")}</h3><footer><span>{text(item, "status")}</span><small>{text(item, "experience_mode", "session_id")}</small></footer></button>)}
              {!episodes.isLoading && filtered.length === 0 && <div className="collection-empty">没有找到符合条件的经历。</div>}
            </div>
            {episodes.hasNextPage && <button className="load-more" onClick={() => episodes.fetchNextPage()} disabled={episodes.isFetchingNextPage}>{episodes.isFetchingNextPage ? "正在继续读取…" : "加载更早的经历"}</button>}
          </section>
          <aside className="archive-side">
            {selectedEpisodeID && <section className="episode-detail"><header><div><span className="eyebrow">EPISODE</span><h2>经历详情</h2></div><button onClick={() => setSelectedEpisodeID("")} aria-label="关闭经历详情"><X size={15} /></button></header>{episode.isLoading ? <p>正在读取原始经历…</p> : detail ? <><div className="episode-content">{text(detail, "content")}</div><dl className="property-list"><div><dt>ID</dt><dd>{text(detail, "id")}</dd></div><div><dt>为什么记住</dt><dd>{text(detail, "why")}</dd></div><div><dt>状态</dt><dd>{text(detail, "status")}</dd></div><div><dt>发生方式</dt><dd>{text(detail, "experience_mode")}</dd></div></dl></> : <p>无法读取这段经历。</p>}</section>}
            <section><header><span className="eyebrow">HYPOTHESES</span><h2>尚未确认</h2></header>{(proposals.data?.proposals || []).slice(0, 6).map((item) => <article key={text(item, "id")}><strong>{text(item, "title", "summary", "field")}</strong><small>{date(item)}</small></article>)}</section>
            <section><header><span className="eyebrow">EVOLUTION</span><h2>变化历史</h2></header>{(mutations.data?.mutations || []).slice(0, 8).map((item) => <article key={text(item, "id", "timestamp")}><strong>{text(item, "field", "kind", "action")}</strong><small>{date(item)}</small></article>)}</section>
          </aside>
        </div>
      )}
    </div>
  );
}
