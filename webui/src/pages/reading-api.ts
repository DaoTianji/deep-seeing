export type Character = { id: string; name: string; initial: string; description: string; color: string };
export type Scene = { id: number; title: string; time: string; place: string; summary: string; question: string; choice: string; resistance: string; characters: string[]; evidence_ids: string[] };
export type Evidence = { id: string; text: string; note: string; scene: number; origin: string };
export type StoryRecord = {id:string; title:string; scene:string; turns:number; completed:boolean; signature?:string; updated_at:string};
export type BookSummary = {lesson_id?:string; read_only?:boolean; text_scope?:string; reader?:{paragraph:number;furthest:number;finished:boolean}; records?:StoryRecord[]; reading?:Reading;id: string; title: string; author: string; subtitle: string; theme: string; scene_count: number; entry_scene: number};
export type Book = {lesson_id?:string; text_scope?:string;  read_only?: boolean; language?: string; entry_scene?: number; theme?: string; excerpts?: Record<string, {text: string; start_byte: number; end_byte: number}>; id: string; title: string; author: string; subtitle: string; source_url: string; source_note: string; version: string; scenes: Scene[]; characters: Character[]; evidence: Evidence[] };
export type Insight = { scene: number; observation: string; question: string; evidence_ids: string[] };
export type Reading = { status: string; completed: number; insights: Insight[]; error?: string; source_hash: string; model: string };
export type Memory = { id: string; character_id: string; text: string; kind: string; revision: number; turn_id: string };
export type Turn = { id: string; request_id: string; character_id: string; message: string; reply: string; observation: string; evidence_ids: string[]; changes: Memory[]; scene_title: string; revision: number };
export type Ending = { title: string; story: string; resolution: string; new_timeline: string[]; contributions: {turn_id: string; text: string}[]; revision: number; created_at: string };
export type Branch = { signature?:string; signed_at?:string; ending?: Ending; influence?: number; id: string; parent_id?: string; parent_revision?: number; anchor: number; revision: number; scene: Scene; memories: Memory[]; turns: Turn[]; events: string[]; diverged: boolean };
export type Snapshot = { facts: { id: string; text: string; evidence_id: string }[]; memories: Memory[] };
export async function request<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
 const url = new URL(`/api/story${path}`, window.location.origin);
 const book = new URLSearchParams(window.location.search).get("book");
 if (book) url.searchParams.set("book", book);
 const response = await fetch(url.pathname + url.search, { signal, credentials: "same-origin", method: body === undefined ? "GET" : "POST", headers: {...readerHeaders(),...(body === undefined ? {} : { "Content-Type": "application/json" })}, body: body === undefined ? undefined : JSON.stringify(body) });
 const raw = await response.text();
 let data; try { data = JSON.parse(raw); } catch { throw new Error("暂时无法连接阅读服务，请稍后重试。"); }
 if (!response.ok) {if(data.code==="reader_required"||data.code==="reader_changed")checkReaderResponse(response.status);throw new Error(data.error || "本次操作没有完成，请重试。");}
 return data;
}
import {readerHeaders,checkReaderResponse} from "./reading-session";
