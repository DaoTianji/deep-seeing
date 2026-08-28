import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowUp, BrainCircuit, ChevronRight, CircleStop, RotateCcw, Sparkles } from "lucide-react";
import { FormEvent, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api, streamChat } from "../api";
import { TurnStudio } from "../components/TurnStudio";
import { MessageContent } from "../components/MessageContent";
import { useTurnStore } from "../store";
import { envelopeToEvent } from "../turn";
import type { AttentionSnapshot, HealthTrace, LiveTurn, Message, StreamEnvelope } from "../types";

function statusText(turn?: LiveTurn) {
  const event = turn?.events.filter((item) => item.type !== "delta").at(-1);
  return event?.label || (turn?.status === "running" ? "正在理解" : "此刻安静");
}

function signature(turn?: LiveTurn) {
  if (!turn) return null;
  const candidates = turn.events.filter((event) => event.state === "candidate").length;
  const reads = turn.events.filter((event) => event.state === "read").length;
  const used = turn.events.filter((event) => event.state === "used").length;
  const attention = turn.events.filter((event) => event.type === "attention_decision").length;
  return { candidates, reads, used, attention };
}

export function ConversationPage() {
  const history = useQuery({ queryKey: ["history"], queryFn: api.history });
  const queryClient = useQueryClient();
  const { turns, putTurn } = useTurnStore();
  const [localMessages, setLocalMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState("");
  const [activeTurn, setActiveTurn] = useState<LiveTurn>();
  const [showNow, setShowNow] = useState(true);
  const controller = useRef<AbortController | undefined>(undefined);
  const endRef = useRef<HTMLDivElement>(null);
  const messages = useMemo(() => [...(history.data?.messages || []).filter((message) => message.role !== "system"), ...localMessages], [history.data, localMessages]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const text = draft.trim();
    if (!text || activeTurn?.status === "running") return;
    setDraft("");
    const tempId = `pending-${Date.now()}`;
    let current: LiveTurn = { id: tempId, userText: text, answer: "", status: "running", events: [], startedAt: new Date().toISOString() };
    setActiveTurn(current);
    setLocalMessages((list) => [...list, { role: "user", content: text, turn_id: tempId }, { role: "assistant", content: "", turn_id: tempId }]);
    controller.current = new AbortController();
    const apply = (envelope: StreamEnvelope) => {
      const id = envelope.turn_id || current.id;
      if (id !== current.id) {
        setLocalMessages((list) => list.map((message) => message.turn_id === current.id ? { ...message, turn_id: id } : message));
        current = { ...current, id };
      }
      if (envelope.type === "delta") {
        current = { ...current, answer: current.answer + String(envelope.data || "") };
        setLocalMessages((list) => list.map((message, index) => index === list.length - 1 ? { ...message, content: current.answer, turn_id: id } : message));
      } else {
        const nextEvent = envelopeToEvent(envelope, current.events.length);
        current = { ...current, events: [...current.events, nextEvent] };
        if (envelope.type === "attention_snapshot") current.attention = envelope.data as AttentionSnapshot;
        if (envelope.type === "health") current.health = envelope.data as HealthTrace;
        if (envelope.type === "done") current.status = "complete";
        if (envelope.type === "error") current.status = "error";
      }
      setActiveTurn({ ...current });
      putTurn({ ...current });
      requestAnimationFrame(() => endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" }));
    };
    try {
      await streamChat(text, controller.current.signal, apply);
      if (current.status === "running") current = { ...current, status: "complete" };
    } catch (error) {
      current = { ...current, status: controller.current.signal.aborted ? "stopped" : "error" };
      if (!controller.current.signal.aborted) {
        current.events = [...current.events, envelopeToEvent({ type: "error", turn_id: current.id, data: { message: error instanceof Error ? error.message : "连接中断" } }, current.events.length)];
      }
    } finally {
      setActiveTurn({ ...current });
      putTurn({ ...current });
      void queryClient.invalidateQueries({ queryKey: ["turns"] });
    }
  };

  return (
    <div className={`conversation-layout ${showNow ? "with-now" : ""}`}>
      <section className="conversation-space">
        <header className="page-heading conversation-heading">
          <div><span className="eyebrow">CONVERSATION</span><h1>隔桌而谈</h1><p>不急着得出答案，让理解在时间里长出来。</p></div>
          <button className={`soft-button ${showNow ? "active" : ""}`} onClick={() => setShowNow((value) => !value)}><BrainCircuit size={16} />此刻</button>
        </header>
        <div className="message-stream" aria-live="polite">
          {history.isLoading && <div className="quiet-empty">正在把这间屋子重新点亮…</div>}
          {!history.isLoading && messages.length === 0 && (
            <div className="welcome-state"><span className="welcome-orbit"><Sparkles /></span><h2>屋子已经亮起</h2><p>写下第一句话。需要过去时，ta 会自己去寻找。</p></div>
          )}
          {messages.map((message, index) => {
            const turn = message.turn_id ? turns.get(message.turn_id) : undefined;
            const stats = signature(turn);
            return (
              <article key={`${message.turn_id || "history"}-${index}`} className={`message ${message.role}`}>
                <div className="message-role">{message.role === "user" ? "你" : message.role === "assistant" ? "Deep Seeing" : "会话折叠"}</div>
                <div className="message-body">{message.content ? <MessageContent content={message.content} /> : (turn?.status === "running" ? <span className="thinking-dots">正在形成回答<i /><i /><i /></span> : "")}</div>
                {message.role === "assistant" && message.turn_id && turn?.status !== "running" && (
                  <Link className="mind-signature" to={`/turn/${encodeURIComponent(message.turn_id)}`}>
                    <span><BrainCircuit size={14} />本轮心智</span>
                    <small>{stats ? <>{stats.candidates ? `${stats.candidates} 次寻找` : "未调用记忆"}{stats.used ? ` · ${stats.used} 条证据` : ""}{stats.attention ? ` · ${stats.attention} 次注意变化` : ""}</> : "查看这次回答的公开轨迹"}</small>
                    <ChevronRight size={14} />
                  </Link>
                )}
              </article>
            );
          })}
          <div ref={endRef} />
        </div>
        <form className="composer" onSubmit={submit}>
          {activeTurn?.status === "error" && <div className="composer-alert"><span>这一轮没有完整结束。</span><button type="button" onClick={() => setDraft(activeTurn.userText)}><RotateCcw size={12} />重新填写</button></div>}
          <textarea value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="想说些什么…" rows={1} onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit(); }
          }} />
          <div className="composer-actions">
            <span>{activeTurn?.status === "running" ? statusText(activeTurn) : "Enter 发送 · Shift + Enter 换行"}</span>
            {activeTurn?.status === "running" ? (
              <button type="button" className="send-button stop" onClick={() => controller.current?.abort()}><CircleStop size={18} /><span>停止</span></button>
            ) : (
              <button type="submit" className="send-button" disabled={!draft.trim()}><ArrowUp size={18} /><span>发送</span></button>
            )}
          </div>
        </form>
      </section>
      {showNow && (
        <aside className="now-panel">
          <header><div><span className="eyebrow">NOW</span><h2>此刻</h2></div><span className={`presence ${activeTurn?.status === "running" ? "active" : ""}`} /></header>
          {!activeTurn && <div className="now-empty"><BrainCircuit size={27} /><strong>还没有正在发生的回合</strong><p>当一次提问开始，这里会出现公开的召回与注意力活动。</p></div>}
          {activeTurn && <>
            <div className="now-question"><small>当前问题</small><p>{activeTurn.userText}</p></div>
            <TurnStudio turn={activeTurn} compact />
            {activeTurn.status !== "running" && <Link className="primary-link" to={`/turn/${encodeURIComponent(activeTurn.id)}`}>展开完整本轮 <ChevronRight size={15} /></Link>}
          </>}
        </aside>
      )}
    </div>
  );
}
