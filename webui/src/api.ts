import type {
  ActiveRole, DirectorActionView, GraphView, Message, RoleClaim, RoleDefinition,
  RoleInstance, RoleSession, RoleSource, RoleTranscriptMessage, RoleWorldline, RoleInitializationRun, RoleInitializationDetail,
  RuntimeSnapshot, StreamEnvelope, TheaterChannel, TurnTrace,
} from "./types";
import type { ReflectionLiveState, ReflectionRun, ReflectionSeed } from "./reflection-types";

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error((payload as { error?: string }).error || `请求失败 (${response.status})`);
  return payload as T;
}

const post = <T>(path: string, body: unknown = {}) => json<T>(path, {
  method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
});

export const api = {
  runtime: () => json<{ runtime: RuntimeSnapshot; graph_label?: string }>("/api/runtime"),
  bootstrap: () => json<{ runtime: RuntimeSnapshot; graph_label?: string; attention?: unknown; turns?: TurnTrace[] }>("/api/bootstrap"),
  history: () => json<{ messages: Message[] }>("/api/history"),
  graph: () => json<GraphView>("/api/graph?limit=500"),
  episodes: (cursor = "") => json<{ episodes: Record<string, unknown>[]; next_cursor?: string }>(`/api/episodes?limit=50${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`),
  episode: (id: string) => json<Record<string, unknown>>(`/api/episode/${encodeURIComponent(id)}`),
  proposals: () => json<{ proposals: Record<string, unknown>[] }>("/api/proposals?limit=100"),
  mutations: () => json<{ mutations: Record<string, unknown>[] }>("/api/mutations?limit=150"),
  reflections: () => json<{ reflections: ReflectionSeed[] }>("/api/reflections?limit=100"),
  reflectionRuns: () => json<{ runs: ReflectionRun[] }>("/api/reflection-runs?limit=60"),
  reflectionLive: () => json<ReflectionLiveState>("/api/reflection-live"),
  self: () => json<{ artifacts: Record<string, unknown>[] }>("/api/self?limit=100"),
  workspace: () => json<{ documents: Record<string, unknown>[] }>("/api/workspace?limit=100"),
  intents: () => json<{ intents: Record<string, unknown>[] }>("/api/intents?limit=100"),
  wakes: () => json<{ wakes: Record<string, unknown>[] }>("/api/wakes?limit=100"),
  agency: () => json<Record<string, unknown>>("/api/agency"),
  sources: () => json<{ sources: Record<string, unknown>[] }>("/api/sources?limit=100"),
  turns: () => json<{ turns: TurnTrace[]; next_cursor?: string }>("/api/turns?limit=60"),
  turn: (id: string) => json<{ turn: TurnTrace }>(`/api/turns/${encodeURIComponent(id)}`),
  review: () => post<Record<string, unknown>>("/api/review"),
  dream: () => post<Record<string, unknown>>("/api/dream"),
  generativeDream: () => post<Record<string, unknown>>("/api/dream/generative"),
  revertMutation: (id: string) => post<Record<string, unknown>>(`/api/mutations/${encodeURIComponent(id)}/revert`, { reason: "用户请求撤销这次认识变化" }),

  roleInitializations: (roleId = "") => json<{ runs: RoleInitializationRun[]; mode: string; coverage_limited: boolean }>(`/api/role-initializations${roleId ? `?role_id=${encodeURIComponent(roleId)}` : ""}`),
  roleInitialization: (id: string) => json<RoleInitializationDetail>(`/api/role-initializations/${encodeURIComponent(id)}`),
  startRoleInitialization: (input: Record<string, unknown>) => post<{ run: RoleInitializationRun; role: RoleDefinition }>("/api/role-initializations", input),
  approveRolePlan: (id: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/approve-plan`),
  grantRoleBudget: (id: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/budget`),
  pauseRoleInitialization: (id: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/pause`),
  resumeRoleInitialization: (id: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/resume`),
  requestRoleBlueprintRevision: (id: string, reason: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/revision`, { reason }),
  approveRoleBlueprint: (id: string, warningAcceptanceReason: string) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/approve`, { warning_acceptance_reason: warningAcceptanceReason }),
  addRoleInitializationDocument: (id: string, input: Record<string, unknown>) => post<Record<string, unknown>>(`/api/role-initializations/${encodeURIComponent(id)}/documents`, input),
  roles: () => json<{ roles: RoleDefinition[]; active?: { definition: RoleDefinition; instance: RoleInstance; session: RoleSession }; mode: string }>("/api/roles"),
  role: (id: string) => json<{ role: RoleDefinition; sources: RoleSource[]; claims: RoleClaim[]; instance?: RoleInstance; worldlines?: RoleWorldline[]; sessions: RoleSession[] }>(`/api/roles/${encodeURIComponent(id)}`),
  createRole: (input: Record<string, unknown>) => post<{ role: RoleDefinition }>("/api/roles", input),
  addRoleSource: (id: string, input: Record<string, unknown>) => post<{ source: RoleSource }>(`/api/roles/${encodeURIComponent(id)}/sources`, input),
  compileRole: (id: string) => post<Record<string, unknown>>(`/api/roles/${encodeURIComponent(id)}/compile`),
  publishRole: (id: string) => post<{ role: RoleDefinition }>(`/api/roles/${encodeURIComponent(id)}/publish`),
  enterRole: (id: string) => post<{ definition: RoleDefinition; instance: RoleInstance; session: RoleSession }>(`/api/roles/${encodeURIComponent(id)}/enter`),
  activeRole: () => json<ActiveRole>("/api/role/active"),
  pauseRole: () => post<{ session: RoleSession }>("/api/role/pause"),
  resumeRole: () => post<{ session: RoleSession }>("/api/role/resume"),
  exitRole: () => post<{ session: RoleSession }>("/api/role/exit", { reason: "user_exit" }),
  forkRole: (label: string) => post<{ worldline: RoleWorldline; instance: RoleInstance }>("/api/role/fork", { label }),
  roleTranscript: (channel: TheaterChannel) => json<{ messages: RoleTranscriptMessage[] }>(`/api/role/transcripts?channel=${channel}&limit=200`),
  roleActions: () => json<{ actions: DirectorActionView[] }>("/api/role/actions"),
  revertDirectorAction: (id: string) => post<{ action: DirectorActionView }>(`/api/director-actions/${encodeURIComponent(id)}/revert`),
};

export interface StreamChatOptions {
  channel?: TheaterChannel;
  roleSessionId?: string;
}

export async function streamChat(
  message: string,
  signal: AbortSignal,
  onEvent: (event: StreamEnvelope) => void,
  options: StreamChatOptions = {},
): Promise<void> {
  const response = await fetch("/api/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      message,
      channel: options.channel,
      role_session_id: options.roleSessionId,
    }),
    signal,
  });
  if (!response.ok || !response.body) {
    const payload = await response.json().catch(() => ({}));
    throw new Error((payload as { error?: string }).error || `发送失败 (${response.status})`);
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const lines = buffer.split("\n");
    buffer = lines.pop() || "";
    for (const line of lines) {
      if (line.trim()) onEvent(JSON.parse(line) as StreamEnvelope);
    }
    if (done) break;
  }
  if (buffer.trim()) onEvent(JSON.parse(buffer) as StreamEnvelope);
}
