export type Source = "bond" | "scene_norm" | "workspace" | "intent" | "proposal" | "episode";
export type EvidenceState = "candidate" | "read" | "used" | "dismissed" | "unknown";
export type AttentionTier = "center" | "support" | "periphery";

export interface Message {
  role: "user" | "assistant" | "summary" | "system";
  content: string;
  turn_id?: string;
}

export interface RuntimeSnapshot {
  available?: boolean;
  recall_mode?: string;
  model?: string;
  person_id?: string;
  session_id?: string;
  graph_available?: boolean;
  [key: string]: unknown;
}

export interface AttentionItem {
  source: Source;
  id: string;
  role?: string;
  tier: AttentionTier;
  idle_turns?: number;
}

export interface AttentionSnapshot {
  turn_offset_ns?: number;
  version?: string;
  items?: AttentionItem[];
  capacity?: Record<AttentionTier, number>;
}

export interface ContextEvent {
  source: Source;
  id?: string;
  result_ids?: string[];
  query?: string;
  operation?: string;
  role?: string;
  disposition?: string;
  reason_code?: string;
  ok?: boolean;
  error?: string;
  duration_ns?: number;
  turn_offset_ns?: number;
}

export interface AttentionDecision {
  source: Source;
  id: string;
  from?: AttentionTier;
  to: AttentionTier;
  replaced_source?: Source;
  replaced_id?: string;
  turn_offset_ns?: number;
}

export interface HealthTrace {
  status?: string;
  codes?: string[];
  total_duration_ns?: number;
  retrieval_duration_ns?: number;
  answer_duration_ns?: number;
  [key: string]: unknown;
}

export interface TurnTrace {
  turn_id?: string;
  timestamp?: string;
  started_at?: string;
  completed_at?: string;
  duration_ns?: number;
  session_id?: string;
  user_text?: string;
  answer_preview?: string;
  recall_mode?: string;
  context_sources?: ContextEvent[];
  context_candidates?: ContextEvent[];
  context_reads?: ContextEvent[];
  context_uses?: ContextEvent[];
  attention?: AttentionSnapshot;
  attention_final?: AttentionSnapshot;
  attention_decisions?: AttentionDecision[];
  tool_starts?: string[];
  errors?: string[];
  health?: HealthTrace;
  token_usage?: Record<string, number>;
  [key: string]: unknown;
}

export interface StreamEnvelope {
  type: string;
  turn_id?: string;
  seq?: number;
  turn_offset_ns?: number;
  data: unknown;
}

export interface GraphNode {
  id: string;
  label?: string;
  type?: string;
  kind?: string;
  title?: string;
  [key: string]: unknown;
}

export interface GraphEdge {
  id?: string;
  source: string;
  target: string;
  type?: string;
  label?: string;
  [key: string]: unknown;
}

export interface GraphView {
  available: boolean;
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface TurnEvent {
  id: string;
  type: string;
  offset: number;
  label: string;
  source?: Source;
  objectId?: string;
  state?: EvidenceState;
  duration?: number;
  data: unknown;
}

export interface LiveTurn {
  id: string;
  userText: string;
  answer: string;
  status: "running" | "complete" | "error" | "stopped";
  events: TurnEvent[];
  attention?: AttentionSnapshot;
  health?: HealthTrace;
  startedAt: string;
}
