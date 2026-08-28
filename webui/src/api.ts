import type { GraphView, Message, RuntimeSnapshot, StreamEnvelope, TurnTrace } from "./types";

async function json<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error((payload as { error?: string }).error || `请求失败 (${response.status})`);
  return payload as T;
}

export const api = {
  runtime: () => json<{ runtime: RuntimeSnapshot; graph_label?: string }>("/api/runtime"),
  bootstrap: () => json<{ runtime: RuntimeSnapshot; graph_label?: string; attention?: unknown; turns?: TurnTrace[] }>("/api/bootstrap"),
  history: () => json<{ messages: Message[] }>("/api/history"),
  graph: () => json<GraphView>("/api/graph?limit=500"),
  episodes: (cursor = "") => json<{ episodes: Record<string, unknown>[]; next_cursor?: string }>(`/api/episodes?limit=50${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`),
  episode: (id: string) => json<Record<string, unknown>>(`/api/episode/${encodeURIComponent(id)}`),
  proposals: () => json<{ proposals: Record<string, unknown>[] }>("/api/proposals?limit=100"),
  mutations: () => json<{ mutations: Record<string, unknown>[] }>("/api/mutations?limit=150"),
  self: () => json<{ artifacts: Record<string, unknown>[] }>("/api/self?limit=100"),
  workspace: () => json<{ documents: Record<string, unknown>[] }>("/api/workspace?limit=100"),
  intents: () => json<{ intents: Record<string, unknown>[] }>("/api/intents?limit=100"),
  wakes: () => json<{ wakes: Record<string, unknown>[] }>("/api/wakes?limit=100"),
  agency: () => json<Record<string, unknown>>("/api/agency"),
  sources: () => json<{ sources: Record<string, unknown>[] }>("/api/sources?limit=100"),
  turns: () => json<{ turns: TurnTrace[]; next_cursor?: string }>("/api/turns?limit=60"),
  turn: (id: string) => json<{ turn: TurnTrace }>(`/api/turns/${encodeURIComponent(id)}`),
};

export async function streamChat(
  message: string,
  signal: AbortSignal,
  onEvent: (event: StreamEnvelope) => void,
): Promise<void> {
  const response = await fetch("/api/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message }),
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
