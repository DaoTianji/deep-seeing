import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive, CirclePause, CirclePlay, Drama, GitFork, Library, LogOut,
  Send, Shield, Sparkles, Upload, UserRoundCog, WandSparkles,
} from "lucide-react";
import { FormEvent, useRef, useState } from "react";
import { api, streamChat } from "../api";
import { MessageContent } from "../components/MessageContent";
import type { RoleDefinition, RoleTranscriptMessage, StreamEnvelope, TheaterChannel } from "../types";

async function fileAsBase64(file: File) {
  const bytes = new Uint8Array(await file.arrayBuffer());
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  }
  return btoa(binary);
}

function roleTypeLabel(role: RoleDefinition) {
  if (role.kind === "professional") return "工作角色";
  return {
    fictional: "虚构人物", deceased: "历史人物",
    living_public: "公众人物", living_private: "私人沙箱",
  }[role.subject_class] || "人物角色";
}

function statusLabel(role: RoleDefinition) {
  return { draft: "草稿", validating: "待确认", ready: "可进入", archived: "已归档" }[role.status] || role.status;
}

function Transcript({
  messages, live, actorName, backstage,
}: {
  messages: RoleTranscriptMessage[]; live?: string; actorName: string; backstage?: boolean;
}) {
  return (
    <div className="theater-transcript">
      {messages.length === 0 && !live && (
        <div className="theater-empty">
          {backstage ? "这里是你与安的私密幕后通道。" : "舞台已经准备好，第一句话会成为这段人生的一部分。"}
        </div>
      )}
      {messages.map((message, index) => (
        <article key={`${message.created_at}-${index}`} className={message.role === "user" ? "user" : "assistant"}>
          <small>{message.role === "user" ? "你" : backstage ? "安 · 导演" : actorName}</small>
          <MessageContent content={message.content} />
        </article>
      ))}
      {live && (
        <article className="assistant live">
          <small>{backstage ? "安 · 导演" : actorName}</small>
          <MessageContent content={live} />
        </article>
      )}
    </div>
  );
}

function TheaterView() {
  const queryClient = useQueryClient();
  const active = useQuery({ queryKey: ["active-role"], queryFn: api.activeRole, refetchInterval: 5000 });
  const session = active.data?.session;
  const definition = active.data?.definition;
  const stage = useQuery({ queryKey: ["role-transcript", "stage", session?.id], queryFn: () => api.roleTranscript("stage"), enabled: Boolean(session) });
  const backstage = useQuery({ queryKey: ["role-transcript", "backstage", session?.id], queryFn: () => api.roleTranscript("backstage"), enabled: Boolean(session) });
  const actions = useQuery({ queryKey: ["role-actions", session?.id], queryFn: api.roleActions, enabled: Boolean(session), refetchInterval: 5000 });
  const [liveStage, setLiveStage] = useState("");
  const [liveBackstage, setLiveBackstage] = useState("");
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");

  const refresh = async () => {
    setLiveStage("");
    setLiveBackstage("");
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["active-role"] }),
      queryClient.invalidateQueries({ queryKey: ["role-transcript"] }),
      queryClient.invalidateQueries({ queryKey: ["role-actions"] }),
      queryClient.invalidateQueries({ queryKey: ["roles"] }),
      queryClient.invalidateQueries({ queryKey: ["graph"] }),
    ]);
  };
  const act = async (name: string, fn: () => Promise<unknown>) => {
    setBusy(name); setError("");
    try { await fn(); await refresh(); } catch (reason) {
      setError(reason instanceof Error ? reason.message : "操作失败");
    } finally { setBusy(""); }
  };

  if (active.isLoading) return <div className="page-state">正在打开剧场…</div>;
  if (!active.data?.active || !definition || !session) return <RoleLibrary />;

  const isPaused = session.status === "paused";
  return (
    <div className="theater-page">
      <header className="theater-hero">
        <div>
          <span className="eyebrow">ROLE THEATER · 模拟角色</span>
          <h1>{definition.display_name}</h1>
          <p>{definition.identity || definition.description || "角色身份正在形成"} · {definition.knowledge_cutoff || "无明确时间截止"}</p>
        </div>
        <div className="theater-controls">
          <span className={active.data.mode === "agent" ? "mode agent" : "mode"}>{active.data.mode === "agent" ? "导演可干预" : "导演观察中"}</span>
          {isPaused ? (
            <button onClick={() => act("resume", api.resumeRole)} disabled={Boolean(busy)}><CirclePlay size={15} />恢复</button>
          ) : (
            <button onClick={() => act("pause", api.pauseRole)} disabled={Boolean(busy)}><CirclePause size={15} />暂停</button>
          )}
          <button onClick={() => act("fork", () => api.forkRole("由你创建的分支"))} disabled={Boolean(busy)}><GitFork size={15} />分叉</button>
          <button className="danger" onClick={() => act("exit", api.exitRole)} disabled={Boolean(busy)}><LogOut size={15} />强制退场</button>
        </div>
      </header>
      {error && <div className="theater-banner error">{error}</div>}
      <div className="theater-meta">
        <span><Drama size={14} />{roleTypeLabel(definition)}</span>
        <span><GitFork size={14} />世界线 {session.worldline_id.slice(-8)}</span>
        <span><Shield size={14} />角色看不见幕后通道</span>
      </div>
      <div className="theater-grid">
        <section className="stage-panel">
          <header><div><small>STAGE</small><h2>台前</h2></div><span>{isPaused ? "已暂停" : "角色在场"}</span></header>
          <Transcript messages={stage.data?.messages || []} live={liveStage} actorName={definition.display_name} />
          {!isPaused && <TheaterChannelInput channel="stage" sessionId={session.id} setLive={setLiveStage} onDone={refresh} />}
        </section>
        <aside className="backstage-panel">
          <header><div><small>BACKSTAGE</small><h2>与安在幕后</h2></div><Shield size={16} /></header>
          <p className="backstage-note">这里发生的事不会进入角色的上下文。安仍是安。</p>
          <Transcript messages={backstage.data?.messages || []} live={liveBackstage} actorName={definition.display_name} backstage />
          <TheaterChannelInput channel="backstage" sessionId={session.id} setLive={setLiveBackstage} onDone={refresh} />
          <div className="director-ledger">
            <header><span>导演记录</span><small>公开工具轨迹，不是隐藏思维</small></header>
            {(actions.data?.actions || []).slice().reverse().slice(0, 8).map((action) => (
              <article key={action.id}>
                <i className={action.type === "no_change" ? "quiet" : action.status} />
                <span><strong>{action.type === "no_change" ? "保持不变" : action.type}</strong><small>{action.reason_code || action.status}</small></span>
                {action.status === "applied" && action.type !== "no_change" && (
                  <button onClick={() => act("revert", () => api.revertDirectorAction(action.id))}>撤销</button>
                )}
              </article>
            ))}
          </div>
        </aside>
      </div>
    </div>
  );
}

function TheaterChannelInput({
  channel, sessionId, setLive, onDone,
}: {
  channel: TheaterChannel; sessionId: string; setLive: (value: string | ((current: string) => string)) => void; onDone: () => void;
}) {
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const text = draft.trim();
    if (!text || busy) return;
    setDraft(""); setLive(""); setBusy(true); setError("");
    const controller = new AbortController();
    try {
      await streamChat(text, controller.signal, (envelope) => {
        if (envelope.type === "delta") setLive((current) => current + String(envelope.data || ""));
        if (envelope.type === "error") {
          const data = envelope.data as { message?: string };
          setError(data.message || "这一轮没有完成");
        }
      }, { channel, roleSessionId: sessionId });
      setLive("");
      onDone();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "连接中断");
    } finally {
      setBusy(false);
    }
  };
  return (
    <form className="theater-composer" onSubmit={submit}>
      <textarea rows={2} value={draft} onChange={(event) => setDraft(event.target.value)}
        placeholder={channel === "backstage" ? "只对安说，角色不会知道…" : "对角色说…"}
        onKeyDown={(event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); event.currentTarget.form?.requestSubmit(); } }}
      />
      <button disabled={!draft.trim() || busy}><Send size={15} />{busy ? "进行中" : "发送"}</button>
      {error && <small className="input-error">{error}</small>}
    </form>
  );
}

function RoleLibrary() {
  const queryClient = useQueryClient();
  const roles = useQuery({ queryKey: ["roles"], queryFn: api.roles });
  const [selected, setSelected] = useState("");
  const selectedID = selected || roles.data?.roles[0]?.id || "";
  const detail = useQuery({ queryKey: ["role", selectedID], queryFn: () => api.role(selectedID), enabled: Boolean(selectedID) });
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState<"character" | "professional">("character");
  const [subject, setSubject] = useState("fictional");
  const [sourceTitle, setSourceTitle] = useState("");
  const [sourceText, setSourceText] = useState("");
  const [sourceFile, setSourceFile] = useState<File>();
  const [sourceURL, setSourceURL] = useState("");
  const [sourceAudience, setSourceAudience] = useState<"actor" | "director">("actor");

  const refresh = async (id?: string) => {
    await queryClient.invalidateQueries({ queryKey: ["roles"] });
    await queryClient.invalidateQueries({ queryKey: ["active-role"] });
    if (id) await queryClient.invalidateQueries({ queryKey: ["role", id] });
  };
  const act = async (name: string, fn: () => Promise<unknown>, id?: string) => {
    setBusy(name); setError("");
    try { await fn(); await refresh(id); } catch (reason) {
      setError(reason instanceof Error ? reason.message : "操作失败");
    } finally { setBusy(""); }
  };
  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) return;
    setBusy("create");
    try {
      const result = await api.createRole({
        display_name: name, kind, subject_class: subject,
        description: kind === "professional" ? "持续工作的专业角色" : "待资料培养的角色",
        allowed_tools: kind === "professional" ? ["list_workspace", "read_workspace", "write_workspace"] : [],
      });
      setName(""); setShowCreate(false); setSelected(result.role.id); await refresh(result.role.id);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "创建失败"); }
    finally { setBusy(""); }
  };
  const current = detail.data?.role;
  const canPublish = current?.status === "validating" && current.validation?.passed;

  return (
    <div className="role-library-page">
      <header className="page-heading wide-heading role-heading">
        <div><span className="eyebrow">ROLE LIBRARY</span><h1>角色人生剧场</h1><p>每个角色都有自己的资料、世界线和记忆。安在幕后守住边界。</p></div>
        <button className="soft-button active" onClick={() => setShowCreate((value) => !value)}><Sparkles size={15} />培养新角色</button>
      </header>
      {roles.data?.mode === "off" && <div className="theater-banner"><Shield size={15} />角色剧场仍处于 off；可以培养角色，但进入前需启用 observe 或 agent。</div>}
      {error && <div className="theater-banner error">{error}</div>}
      {showCreate && (
        <form className="role-create-card" onSubmit={create}>
          <label>角色名字<input value={name} onChange={(event) => setName(event.target.value)} placeholder="例如：林舟、弗洛伊德、编辑" /></label>
          <label>角色类型<select value={kind} onChange={(event) => setKind(event.target.value as "character" | "professional")}><option value="character">人物角色</option><option value="professional">工作角色</option></select></label>
          <label>人物性质<select value={subject} onChange={(event) => setSubject(event.target.value)} disabled={kind === "professional"}><option value="fictional">虚构</option><option value="deceased">已故历史人物</option><option value="living_public">在世公众人物</option><option value="living_private">私人个体</option></select></label>
          {kind === "professional" && <span className="role-tool-note">仅授权版本化 Workspace 读写，不含发布、外部通信或删除。</span>}
          <button disabled={!name.trim() || Boolean(busy)}>创建草稿</button>
        </form>
      )}
      <div className="role-library-layout">
        <section className="role-shelf">
          <header><Library size={17} /><span>可选择的角色</span><small>{roles.data?.roles.length || 0}</small></header>
          <div className="role-card-grid">
            {(roles.data?.roles || []).map((role) => (
              <button key={role.id} className={selectedID === role.id ? "role-card selected" : "role-card"} onClick={() => setSelected(role.id)}>
                <span className="role-avatar">{role.kind === "professional" ? <UserRoundCog /> : <Drama />}</span>
                <span><strong>{role.display_name}</strong><small>{roleTypeLabel(role)}</small></span>
                <em>{statusLabel(role)}</em>
              </button>
            ))}
            {!roles.isLoading && roles.data?.roles.length === 0 && <div className="role-empty">还没有角色。先培养第一个完全受控的虚构角色。</div>}
          </div>
        </section>
        <aside className="role-workbench">
          {!current ? <div className="role-empty">选择一个角色查看培养进度。</div> : <>
            <header>
              <div><small>{roleTypeLabel(current)} · v{current.version}</small><h2>{current.display_name}</h2></div>
              <span className={current.status}>{statusLabel(current)}</span>
            </header>
            {current.private_sandbox && <div className="private-sandbox"><Shield size={14} />私人沙箱：不联网、不分享、不对外冒充。</div>}
            <p>{current.identity || current.description || "还没有编译出稳定的角色身份。"}</p>
            <dl className="role-facts">
              <div><dt>知识截止</dt><dd>{current.knowledge_cutoff || "待资料确定"}</dd></div>
              <div><dt>来源</dt><dd>{detail.data?.sources.length || 0} 份</dd></div>
              <div><dt>主张</dt><dd>{detail.data?.claims.length || 0} 条</dd></div>
              <div><dt>世界线</dt><dd>{detail.data?.worldlines?.length || 0} 条</dd></div>
            </dl>
            {(detail.data?.sources.length || 0) > 0 && (
              <div className="role-source-list">
                {detail.data?.sources.map((source) => (
                  <span key={source.id}><strong>{source.title}</strong><small>{source.audience === "director" ? "仅安可知 · 后世研究" : "角色可知 · 编入模型"}</small></span>
                ))}
              </div>
            )}
            {current.status !== "ready" && <>
              <div className="source-editor">
                <label>资料用途<select value={sourceAudience} onChange={(event) => setSourceAudience(event.target.value as "actor" | "director")}><option value="actor">角色可知 · 编入人物模型</option><option value="director">仅安可知 · 后世评价/研究</option></select></label>
                <label>资料标题<input value={sourceTitle} onChange={(event) => setSourceTitle(event.target.value)} placeholder="例如：人物小传第一章" /></label>
                <label>文件（Markdown、纯文本或带文本层 PDF）<input type="file" accept=".md,.txt,.pdf,text/plain,text/markdown,application/pdf" onChange={(event) => { const file = event.target.files?.[0]; setSourceFile(file); if (file && !sourceTitle) setSourceTitle(file.name); }} /></label>
                <label>或者添加公开网页 URL<input type="url" value={sourceURL} onChange={(event) => setSourceURL(event.target.value)} placeholder="https://…" /></label>
                <label>Markdown / 纯文本<textarea rows={6} value={sourceText} onChange={(event) => setSourceText(event.target.value)} placeholder="把你希望安研究的资料放在这里…" /></label>
                <button onClick={() => act("source", async () => api.addRoleSource(current.id, sourceFile ? { title: sourceTitle, kind: "upload", audience: sourceAudience, mime_type: sourceFile.type || "application/octet-stream", content_base64: await fileAsBase64(sourceFile) } : sourceText.trim() ? { title: sourceTitle, kind: "upload", audience: sourceAudience, mime_type: "text/plain", content: sourceText } : { title: sourceTitle || sourceURL, kind: "webpage", audience: sourceAudience, url: sourceURL }), current.id)} disabled={(!sourceTitle.trim() && !sourceURL.trim()) || (!sourceFile && !sourceText.trim() && !sourceURL.trim()) || Boolean(busy)}><Upload size={14} />加入资料</button>
              </div>
              <div className="role-build-actions">
                <button onClick={() => act("compile", () => api.compileRole(current.id), current.id)} disabled={!detail.data?.sources.length || Boolean(busy)}><WandSparkles size={14} />让安编译角色</button>
                <button className="primary" onClick={() => act("publish", () => api.publishRole(current.id), current.id)} disabled={!canPublish || Boolean(busy)}><Archive size={14} />确认上架</button>
              </div>
            </>}
            {current.validation && !current.validation.passed && (
              <div className="validation-list">{current.validation.issues?.map((issue) => <span key={issue.code}>{issue.message}</span>)}</div>
            )}
            {current.status === "ready" && (
              <button className="enter-role" onClick={() => act("enter", () => api.enterRole(current.id), current.id)} disabled={roles.data?.mode === "off" || Boolean(busy)}><Drama size={16} />进入这个角色</button>
            )}
          </>}
        </aside>
      </div>
    </div>
  );
}

export function RolePage() {
  return <TheaterView />;
}
