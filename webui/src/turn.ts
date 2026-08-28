import type { AttentionSnapshot, ContextEvent, EvidenceState, LiveTurn, Source, StreamEnvelope, TurnEvent, TurnTrace } from "./types";

const sourceNames: Record<Source, string> = {
  bond: "关系常模",
  scene_norm: "场景经验",
  workspace: "当前任务",
  intent: "未来意图",
  proposal: "未确认认识",
  episode: "过往经历",
};

export function sourceName(source?: Source): string {
  return source ? sourceNames[source] || source : "系统";
}

function contextData(raw: unknown): ContextEvent {
  return (raw || {}) as ContextEvent;
}

export function eventLabel(type: string, raw: unknown): string {
  const data = contextData(raw);
  const name = sourceName(data.source);
  switch (type) {
    case "start": return "开始理解这次提问";
    case "tool": return `使用工具 · ${String((raw as { name?: string })?.name || "未知")}`;
    case "context_source": return `${name} · ${String((raw as { state?: string })?.state || "可用")}`;
    case "context_candidate":
    case "recall_search": return `寻找${name}${data.result_ids?.length ? ` · ${data.result_ids.length} 个候选` : ""}`;
    case "context_read":
    case "recall_read": return `读取${name}${data.id || (raw as { episode_id?: string })?.episode_id ? ` · ${data.id || (raw as { episode_id?: string }).episode_id}` : ""}`;
    case "context_use":
    case "recall_evidence": {
      const disposition = data.disposition || (raw as { status?: string })?.status;
      return `${disposition === "used" ? "采用" : disposition === "dismissed" ? "排除" : "聚焦"}${name}`;
    }
    case "attention_snapshot": return "看见当前注意力空间";
    case "attention_decision": return `移动注意力 · ${data.id || "对象"}`;
    case "task_context": return "理解当前任务处境";
    case "task_context_expand": return "展开任务背景";
    case "task_context_focus": return "确认当前任务焦点";
    case "health": return "完成运行检查";
    case "done": return "回答完成";
    case "error": return "本轮发生异常";
    default: return type.replaceAll("_", " ");
  }
}

function eventState(type: string, raw: unknown): EvidenceState | undefined {
  const data = contextData(raw);
  if (type.includes("candidate") || type === "recall_search") return "candidate";
  if (type.includes("read")) return "read";
  const disposition = data.disposition || (raw as { status?: string })?.status;
  if (disposition === "used") return "used";
  if (disposition === "dismissed") return "dismissed";
  return undefined;
}

export function envelopeToEvent(envelope: StreamEnvelope, index: number): TurnEvent {
  const data = contextData(envelope.data);
  return {
    id: `${envelope.turn_id || "turn"}-${envelope.seq ?? index}`,
    type: envelope.type,
    offset: envelope.turn_offset_ns ?? data.turn_offset_ns ?? index * 1_000_000,
    duration: data.duration_ns,
    label: eventLabel(envelope.type, envelope.data),
    source: data.source || (envelope.type.startsWith("recall_") ? "episode" : undefined),
    objectId: data.id || (envelope.data as { episode_id?: string })?.episode_id,
    state: eventState(envelope.type, envelope.data),
    data: envelope.data,
  };
}

export function traceToEvents(trace: TurnTrace): TurnEvent[] {
  const events: TurnEvent[] = [];
  let index = 0;
  const add = (type: string, data: unknown, offset?: number) => {
    events.push(envelopeToEvent({ type, turn_id: trace.turn_id, seq: index++, turn_offset_ns: offset, data }, index));
  };
  add("start", { at: trace.started_at || trace.timestamp }, 0);
  trace.context_sources?.forEach((event) => add("context_source", event, event.turn_offset_ns));
  trace.context_candidates?.forEach((event) => add("context_candidate", event, event.turn_offset_ns));
  trace.context_reads?.forEach((event) => add("context_read", event, event.turn_offset_ns));
  trace.context_uses?.forEach((event) => add("context_use", event, event.turn_offset_ns));
  if (trace.attention) add("attention_snapshot", trace.attention, trace.attention.turn_offset_ns || 1);
  trace.attention_decisions?.forEach((event) => add("attention_decision", event, event.turn_offset_ns));
  if (trace.attention_final) add("attention_snapshot", trace.attention_final, trace.attention_final.turn_offset_ns || trace.duration_ns);
  if (trace.health) add("health", trace.health, trace.duration_ns);
  add("done", { answer: trace.answer_preview }, trace.duration_ns);
  return events.sort((a, b) => a.offset - b.offset);
}

export function traceToLiveTurn(trace: TurnTrace): LiveTurn {
  return {
    id: trace.turn_id || trace.timestamp || "unknown",
    userText: trace.user_text || "",
    answer: trace.answer_preview || "",
    status: trace.errors?.length ? "error" : "complete",
    events: traceToEvents(trace),
    attention: trace.attention_final || trace.attention,
    health: trace.health,
    startedAt: trace.started_at || trace.timestamp || new Date().toISOString(),
  };
}

export function evidenceFromEvents(events: TurnEvent[]): Map<string, EvidenceState> {
  const rank: Record<EvidenceState, number> = { unknown: 0, candidate: 1, read: 2, dismissed: 3, used: 4 };
  const result = new Map<string, EvidenceState>();
  for (const event of events) {
    const raw = event.data as ContextEvent & { episode_id?: string; result_ids?: string[] };
    const objectId = event.objectId || raw.id || raw.episode_id;
    const ids = raw.result_ids || (objectId ? [objectId] : []);
    for (const id of ids) {
      const next = event.state || "candidate";
      if (rank[next] >= rank[result.get(id) || "unknown"]) result.set(id, next);
    }
  }
  return result;
}

export function attentionAt(turn: LiveTurn, offset: number): AttentionSnapshot | undefined {
  const hasRecordedSnapshot = turn.events.some((event) => event.type === "attention_snapshot");
  let snapshot = hasRecordedSnapshot ? undefined : turn.attention ? { ...turn.attention, items: turn.attention.items?.map((item) => ({ ...item })) } : undefined;
  for (const event of turn.events) {
    if (event.offset > offset) break;
    if (event.type === "attention_snapshot") {
      const recorded = event.data as AttentionSnapshot;
      snapshot = { ...recorded, items: recorded.items?.map((item) => ({ ...item })) };
    }
    if (event.type === "attention_decision" && snapshot) {
      const decision = event.data as { source: Source; id: string; to: "center" | "support" | "periphery"; replaced_source?: Source; replaced_id?: string };
      const items = (snapshot.items || []).map((item) => ({ ...item })).filter((item) => !(item.source === decision.replaced_source && item.id === decision.replaced_id));
      const found = items.find((item) => item.source === decision.source && item.id === decision.id);
      if (found) found.tier = decision.to;
      else items.push({ source: decision.source, id: decision.id, tier: decision.to });
      snapshot = { ...snapshot, items };
    }
  }
  return snapshot;
}
