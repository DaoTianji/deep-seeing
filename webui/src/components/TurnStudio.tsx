import { useEffect, useMemo, useState } from "react";
import { Activity, Check, CircleDot, Clock3, Eye, Pause, Play, Search, X } from "lucide-react";
import type { AttentionItem, AttentionTier, EvidenceState, LiveTurn, TurnEvent } from "../types";
import { attentionAt, evidenceFromEvents, sourceName } from "../turn";

const lanes: Record<string, { label: string; icon: typeof Search }> = {
  context: { label: "处境", icon: CircleDot },
  recall: { label: "召回", icon: Search },
  evidence: { label: "证据", icon: Check },
  attention: { label: "注意", icon: Eye },
  action: { label: "行动", icon: Activity },
};

function laneFor(event: TurnEvent) {
  if (event.type.includes("attention")) return "attention";
  if (event.type.includes("candidate") || event.type.includes("search") || event.type.includes("read")) return "recall";
  if (event.type.includes("use") || event.type.includes("evidence")) return "evidence";
  if (event.type.includes("context") || event.type === "start") return "context";
  return "action";
}

function seconds(ns = 0) { return `${(ns / 1e9).toFixed(ns >= 1e9 ? 1 : 2)}s`; }

const stateLabel: Record<EvidenceState, string> = {
  candidate: "候选", read: "已读", used: "采用", dismissed: "排除", unknown: "未处理",
};

function AttentionColumn({ tier, items }: { tier: AttentionTier; items: AttentionItem[] }) {
  const title = tier === "center" ? "意识中心" : tier === "support" ? "支撑区" : "外围";
  return (
    <section className={`attention-zone ${tier}`}>
      <header><span>{title}</span><small>{items.length}</small></header>
      <div className="attention-items">
        {items.length === 0 && <span className="zone-empty">此刻安静</span>}
        {items.map((item) => (
          <article key={`${item.source}:${item.id}`}>
            <i className={`source-glyph source-${item.source}`} />
            <span><strong>{item.id}</strong><small>{sourceName(item.source)}{item.idle_turns ? ` · 闲置 ${item.idle_turns} 轮` : ""}</small></span>
          </article>
        ))}
      </div>
    </section>
  );
}

export function TurnStudio({ turn, compact = false }: { turn: LiveTurn; compact?: boolean }) {
  const maximum = Math.max(...turn.events.map((event) => event.offset), 1);
  const [cursor, setCursor] = useState(maximum);
  const [playing, setPlaying] = useState(false);
  useEffect(() => setCursor(maximum), [maximum, turn.id]);
  useEffect(() => {
    if (!playing) return;
    if (cursor >= maximum) setCursor(0);
    const step = Math.max(maximum / 120, 10_000_000);
    const timer = window.setInterval(() => setCursor((value) => {
      if (value + step >= maximum) { setPlaying(false); return maximum; }
      return value + step;
    }), 40);
    return () => window.clearInterval(timer);
  }, [playing, maximum, cursor]);
  const visible = useMemo(() => turn.events.filter((event) => event.offset <= cursor), [turn.events, cursor]);
  const attention = attentionAt(turn, cursor);
  const evidence = evidenceFromEvents(visible);
  const active = visible.at(-1);

  return (
    <div className={`turn-studio ${compact ? "compact" : ""}`}>
      <section className="turn-playback">
        <header className="studio-heading">
          <div><span className="eyebrow">PUBLIC COGNITIVE TRACE</span><h2>这一轮，发生了什么</h2></div>
          <div className="playback-status"><Clock3 size={15} /><span>{seconds(cursor)}</span><small>/ {seconds(maximum)}</small></div>
        </header>
        <div className="playback-control">
          <button onClick={() => setPlaying((value) => !value)} aria-label={playing ? "暂停回放" : "播放回放"}>
            {playing ? <Pause size={15} /> : <Play size={15} />}
          </button>
          <input aria-label="回放时间" type="range" min={0} max={maximum} value={cursor} onChange={(event) => { setPlaying(false); setCursor(Number(event.target.value)); }} />
          <span>{active?.label || "尚未开始"}</span>
        </div>
        <div className="timeline-grid">
          {Object.entries(lanes).map(([lane, meta]) => {
            const Icon = meta.icon;
            const events = turn.events.filter((event) => laneFor(event) === lane);
            return (
              <div className="timeline-lane" key={lane}>
                <div className="lane-label"><Icon size={14} /><span>{meta.label}</span></div>
                <div className="lane-track">
                  <div className="elapsed" style={{ width: `${Math.max(0, Math.min(100, cursor / maximum * 100))}%` }} />
                  {events.map((event) => (
                    <button
                      key={event.id}
                      className={`event-dot ${event.state || ""} ${event.offset <= cursor ? "visible" : "future"}`}
                      style={{ left: `${Math.min(99, event.offset / maximum * 100)}%` }}
                      onClick={() => { setPlaying(false); setCursor(event.offset); }}
                      aria-label={`${event.label}，${seconds(event.offset)}`}
                      title={`${event.label} · ${seconds(event.offset)}`}
                    />
                  ))}
                </div>
              </div>
            );
          })}
        </div>
        {!compact && <div className="event-feed">
          {visible.filter((event) => !["delta", "start"].includes(event.type)).map((event) => (
            <article key={event.id} className={event.id === active?.id ? "active" : ""} onClick={() => setCursor(event.offset)}>
              <time>{seconds(event.offset)}</time><i className={event.state || ""} />
              <span><strong>{event.label}</strong>{event.duration ? <small>耗时 {seconds(event.duration)}</small> : null}</span>
            </article>
          ))}
        </div>}
      </section>

      <section className="attention-theatre">
        <header className="studio-heading">
          <div><span className="eyebrow">ATTENTION FIELD</span><h2>此刻，什么进入意识</h2></div>
          <span className="public-boundary">公开状态 · 非隐藏思维</span>
        </header>
        <div className="attention-stage">
          {(["center", "support", "periphery"] as AttentionTier[]).map((tier) => (
            <AttentionColumn key={tier} tier={tier} items={(attention?.items || []).filter((item) => item.tier === tier)} />
          ))}
        </div>
        <div className="evidence-dock">
          <div className="dock-title"><span>本轮证据</span><small>{evidence.size} 个对象</small></div>
          {evidence.size === 0 && <p>这一轮没有调用长期记忆，这也是正常的。</p>}
          {[...evidence.entries()].map(([id, state]) => (
            <article key={id} className={`evidence-card ${state}`}>
              {state === "used" ? <Check size={14} /> : state === "dismissed" ? <X size={14} /> : <CircleDot size={14} />}
              <span><strong>{id}</strong><small>{stateLabel[state]}</small></span>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}
