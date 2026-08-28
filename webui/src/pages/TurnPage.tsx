import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, Clock3, Cpu, HeartPulse } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { api } from "../api";
import { TurnStudio } from "../components/TurnStudio";
import { useTurnStore } from "../store";
import { traceToLiveTurn } from "../turn";

export function TurnPage() {
  const { turnId = "" } = useParams();
  const { getTurn } = useTurnStore();
  const local = getTurn(turnId);
  const query = useQuery({ queryKey: ["turn", turnId], queryFn: () => api.turn(turnId), enabled: !local && !!turnId, retry: false });
  const turn = local || (query.data?.turn ? traceToLiveTurn(query.data.turn) : undefined);
  if (query.isLoading && !turn) return <div className="page-state">正在找到这一轮留下的痕迹…</div>;
  if (!turn) return <div className="page-state error"><h1>没有找到这一轮</h1><p>它可能来自旧版本，尚未拥有稳定的 Turn ID。</p><Link to="/">回到对话</Link></div>;
  const total = Math.max(...turn.events.map((event) => event.offset), 0);
  return (
    <div className="turn-page">
      <header className="turn-hero">
        <Link to="/" className="back-link"><ArrowLeft size={16} />回到对话</Link>
        <div><span className="eyebrow">TURN · {turn.id.slice(0, 12)}</span><h1>{turn.userText || "一次没有标题的交谈"}</h1><p>从公开事件中回看：哪些背景被看见，哪些记忆成为证据，注意力如何移动。</p></div>
        <div className="turn-metrics">
          <article><Clock3 /><span><strong>{(total / 1e9).toFixed(1)}s</strong><small>公开过程</small></span></article>
          <article><Cpu /><span><strong>{turn.events.length}</strong><small>结构事件</small></span></article>
          <article><HeartPulse /><span><strong>{turn.health?.status || "完成"}</strong><small>运行状态</small></span></article>
        </div>
      </header>
      <TurnStudio turn={turn} />
      <section className="turn-answer"><span className="eyebrow">ANSWER</span><h2>最终回答</h2><div>{turn.answer || "回答正文没有写入公开轨迹。请从当前会话中查看。"}</div></section>
    </div>
  );
}
