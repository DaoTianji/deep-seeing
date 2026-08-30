import { useQuery } from "@tanstack/react-query";
import { Activity, Compass, Eye, HeartHandshake, Orbit, Sparkles } from "lucide-react";
import { api } from "../api";
import { ReflectionPanel } from "../components/ReflectionPanel";

function title(item: Record<string, unknown>) {
  return String(item.title || item.summary || item.field || item.query || item.url || item.id || "未命名");
}
function meta(item: Record<string, unknown>) {
  return String(item.status || item.kind || item.type || item.decision || "");
}

function Collection({ label, heading, items, empty }: { label: string; heading: string; items: Record<string, unknown>[]; empty: string }) {
  return <section className="mind-collection"><header><span className="eyebrow">{label}</span><h2>{heading}</h2><small>{items.length}</small></header><div>{items.slice(0, 8).map((item, index) => <article key={String(item.id || index)}><i /><span><strong>{title(item)}</strong><small>{meta(item)}</small></span></article>)}{items.length === 0 && <p className="collection-empty">{empty}</p>}</div></section>;
}

export function MindPage() {
  const workspace = useQuery({ queryKey: ["workspace"], queryFn: api.workspace });
  const intents = useQuery({ queryKey: ["intents"], queryFn: api.intents });
  const self = useQuery({ queryKey: ["self"], queryFn: api.self });
  const wakes = useQuery({ queryKey: ["wakes"], queryFn: api.wakes });
  const sources = useQuery({ queryKey: ["sources"], queryFn: api.sources });
  const proposals = useQuery({ queryKey: ["proposals"], queryFn: api.proposals });
  const mutations = useQuery({ queryKey: ["mutations"], queryFn: api.mutations });
	const reflections = useQuery({ queryKey: ["reflections"], queryFn: api.reflections, refetchInterval: 12_000 });
  const agency = useQuery({ queryKey: ["agency"], queryFn: api.agency });
  const documents = workspace.data?.documents || [];
  const intentItems = intents.data?.intents || [];
  const artifacts = self.data?.artifacts || [];
  return (
    <div className="mind-page">
      <header className="page-heading wide-heading"><div><span className="eyebrow">LIVING MIND</span><h1>ta 正在成为谁</h1><p>长期的理解、此刻的牵挂、未来的约定，以及主动伸向世界的触角。</p></div><div className="mind-presence"><span /><span /><span /><strong>持续生长</strong></div></header>
      <section className="mind-overview">
        <article><HeartHandshake /><span><small>关系基础</small><strong>Bond</strong><em>作为每轮理解的底色</em></span></article>
        <article><Sparkles /><span><small>正在生长</small><strong>{documents.length}</strong><em>Workspace</em></span></article>
        <article><Compass /><span><small>未来约定</small><strong>{intentItems.length}</strong><em>Intent</em></span></article>
        <article><Eye /><span><small>自我理解</small><strong>{artifacts.length}</strong><em>Self</em></span></article>
      </section>
      <section className="mind-section-heading"><span className="eyebrow">PRESENT</span><h2>此刻仍在心上的事</h2><p>任务是正在处理的现实，意图是面向未来的计划；两者不会被混为已经发生。</p></section>
      <div className="mind-grid two">
        <Collection label="WORKSPACE" heading="正在进行" items={documents} empty="此刻没有持续占据工作空间的任务。" />
        <Collection label="INTENT" heading="未来约定" items={intentItems} empty="此刻没有等待履行的未来约定。" />
      </div>
      <section className="mind-section-heading"><span className="eyebrow">IDENTITY</span><h2>缓慢形成的自己</h2><p>自我认识可以修订；未确认的认识仍保持为假设。</p></section>
      <div className="mind-grid two">
        <Collection label="SELF" heading="关于自己的理解" items={artifacts} empty="还没有形成可展示的自我产物。" />
        <Collection label="PROPOSAL" heading="尚未确认" items={proposals.data?.proposals || []} empty="此刻没有等待确认的假设。" />
      </div>
		<section className="mind-section-heading"><span className="eyebrow">REFLECTION</span><h2>经历如何改变了安</h2><p>问题可以长期保持未决；证据、冲突、想象与正式改变不会混在一起。</p></section>
		<ReflectionPanel seeds={reflections.data?.reflections || []} mutations={mutations.data?.mutations || []} />
      <section className="mind-section-heading"><span className="eyebrow">AGENCY & WORLD</span><h2>主动生活与外界接触</h2><p>这里只展示可追溯的行动和来源，不把外界资料自动当成信念。</p></section>
      <div className="mind-grid three">
        <Collection label="WAKE" heading="自主唤醒" items={wakes.data?.wakes || []} empty="没有近期自主唤醒。" />
        <Collection label="WORLD" heading="外界来源" items={sources.data?.sources || []} empty="还没有留下可追溯的外界来源。" />
        <Collection label="LEDGER" heading="变化历史" items={mutations.data?.mutations || []} empty="还没有新的结构变化。" />
      </div>
      {agency.isError && <div className="panel-warning"><Activity size={16} />自主系统暂时不可读取，但不会影响其他心智区域。</div>}
    </div>
  );
}
