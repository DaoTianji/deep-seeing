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

export type RoleMode = "off" | "observe" | "agent";
export type RoleKind = "character" | "professional";
export type SubjectClass = "fictional" | "deceased" | "living_public" | "living_private";
export type RoleStatus = "draft" | "validating" | "ready" | "archived";
export type RoleSessionStatus = "active" | "paused" | "completed" | "aborted";
export type TheaterChannel = "stage" | "backstage";

export interface RoleDefinition {
  id: string;
  display_name: string;
  kind: RoleKind;
  subject_class: SubjectClass;
  description?: string;
  identity?: string;
  voice?: string;
  knowledge_cutoff?: string;
  private_sandbox?: boolean;
  main_instance_id?: string;
  status: RoleStatus;
  version: number;
  validation?: { passed: boolean; issues?: Array<{ code: string; severity: string; message: string }> };
}

export interface RoleInstance {
  id: string;
  role_id: string;
  main_worldline_id: string;
  current_worldline_id: string;
  status: string;
  scene?: string;
  state?: Record<string, string>;
  version: number;
}

export interface RoleSession {
  id: string;
  role_id: string;
  role_instance_id: string;
  worldline_id: string;
  status: RoleSessionStatus;
  exit_reason?: string;
  started_at: string;
}

export interface RoleSource {
  id: string;
  role_id: string;
  title: string;
  kind: string;
  audience: "actor" | "director";
  url?: string;
  mime_type?: string;
}

export interface RoleClaim {
  id: string;
  role_id: string;
  kind: string;
  statement: string;
  source_ids?: string[];
  confidence?: number;
}

export interface RoleWorldline {
  id: string;
  parent_worldline_id?: string;
  label: string;
  state?: Record<string, string>;
  version: number;
}

export interface DirectorActionView {
  id: string;
  type: string;
  status: string;
  reason_code?: string;
  before?: Record<string, string>;
  after?: Record<string, string>;
  reverts_action_id?: string;
  created_at: string;
}

export interface RoleTranscriptMessage {
  turn_id?: string;
  channel: TheaterChannel;
  role: string;
  content: string;
  created_at: string;
}

export interface ActiveRole {
  active: boolean;
  mode: RoleMode;
  definition?: RoleDefinition;
  instance?: RoleInstance;
  session?: RoleSession;
}

export type RoleInitializationMode = "off" | "observe" | "agent";
export type RoleInitializationStatus = "draft" | "planning" | "awaiting_plan_approval" | "collecting" | "analyzing" | "compiling" | "blueprinting" | "critiquing" | "awaiting_final_approval" | "completed" | "paused" | "needs_budget" | "failed" | "cancelled";
export interface RoleResearchPlan {
  target_period: string;
  knowledge_cutoff?: string;
  questions: Array<{ id: string; question: string; topics?: string[]; search_terms?: string[]; priority?: string }>;
  completion_criteria?: string[];
  approved_at?: string;
}
export interface RoleCoverageItem { dimension: string; state: "missing" | "partial" | "sufficient" | "contested"; summary?: string; source_ids?: string[]; chunk_ids?: string[] }
export interface RoleBlueprintSection { key: string; content: string; claim_ids?: string[]; chunk_ids?: string[] }
export interface RoleBlueprint {
  id: string; run_id: string; role_id: string; version: number; target_period: string; knowledge_cutoff: string;
  self_concept: RoleBlueprintSection; values_and_motives: RoleBlueprintSection; tensions: RoleBlueprintSection;
  relationships?: RoleBlueprintSection[]; reasoning_and_voice: RoleBlueprintSection; unknown_response_policy: RoleBlueprintSection;
  allowed_inferences: RoleBlueprintSection; forbidden_anachronisms: RoleBlueprintSection; change_summary?: string;
}
export interface RoleCritiqueIssue { code: string; severity: "hard" | "warning"; message: string; section?: string; resolved?: boolean }
export interface RoleCritique { id: string; passed: boolean; issues?: RoleCritiqueIssue[]; warning_acceptance_reason?: string }
export interface RoleInitializationRun {
  id: string; role_id: string; status: RoleInitializationStatus; current_step?: string; checkpoint?: string; objective?: string;
  plan?: RoleResearchPlan; coverage: { items: RoleCoverageItem[]; updated_at: string }; conflicts?: Array<{ id: string; topic: string; disposition?: string }>;
  assessments?: Array<{ source_id: string; tier: string; audience: "actor" | "director"; status: string; reliable?: string; read_chunk_ids?: string[] }>;
  blueprint_id?: string; critique_id?: string; remote_budget: number; remote_used: number; search_provider?: string; error_summary?: string; revision_request?: string;
  readiness_research_attempts?: number; research_focus?: RoleResearchPlan["questions"];
}
export interface RoleInitializationDetail { run: RoleInitializationRun; role: RoleDefinition; blueprint?: RoleBlueprint; critique?: RoleCritique; mode: RoleInitializationMode }
