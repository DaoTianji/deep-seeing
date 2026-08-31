import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive, CirclePause, CirclePlay, Drama, GitFork, Library, LogOut,
  Send, Shield, Sparkles, Upload, UserRoundCog, WandSparkles,
} from "lucide-react";
import { FormEvent, useRef, useState } from "react";
import { api, streamChat } from "../api";
import { MessageContent } from "../components/MessageContent";
import type { RoleDefinition, RoleInitializationDetail, RoleTranscriptMessage, StreamEnvelope, TheaterChannel } from "../types";

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

function InitializationPanel({
  detail, busy, onAction,
}: {
  detail: RoleInitializationDetail;
  busy: string;
  onAction: (name: string, action: () => Promise<unknown>) => void;
}) {
  const { run, blueprint, critique } = detail;
  const [revision, setRevision] = useState("");
  const [warningReason, setWarningReason] = useState("");
  const [documentFile, setDocumentFile] = useState<File>();
  const [documentAudience, setDocumentAudience] = useState<"actor" | "director">("actor");
  const [documentTier, setDocumentTier] = useState("biography");
  const stages = ["planning", "awaiting_plan_approval", "collecting", "analyzing", "compiling", "blueprinting", "critiquing", "awaiting_final_approval", "completed"];
  const labels: Record<string, string> = { planning: "研究计划", awaiting_plan_approval: "等待确认", collecting: "来源地图", analyzing: "覆盖分析", compiling: "证据编译", blueprinting: "塑造方案", critiquing: "独立审查", awaiting_final_approval: "最终确认", completed: "已上架" };
  const current = Math.max(0, stages.indexOf(run.status));
  const coverageItems = run.coverage?.items || [];
  const hardIssues = critique?.issues?.filter((issue) => issue.severity === "hard" && !issue.resolved) || [];
  const warnings = critique?.issues?.filter((issue) => issue.severity === "warning" && !issue.resolved) || [];
  const sections = blueprint ? [blueprint.self_concept, blueprint.values_and_motives, blueprint.tensions, ...(blueprint.relationships || []), blueprint.reasoning_and_voice, blueprint.unknown_response_policy, blueprint.allowed_inferences, blueprint.forbidden_anachronisms] : [];
  return (
    <section className="initialization-room">
      <header><div><small>CHARACTER ARCHITECT</small><h3>培养室</h3></div><span className={`init-status ${run.status}`}>{labels[run.status] || run.status}</span></header>
      <p className="init-disclaimer">这里展示公开研究轨迹与结构化判断，不展示隐藏思维。</p>
      <div className="init-steps">{stages.map((stage, index) => <span key={stage} className={index < current ? "done" : index === current ? "current" : ""}><i />{labels[stage]}</span>)}</div>
      <div className="init-budget"><span>研究提供者 <strong>{run.search_provider || "未配置"}</strong></span><span>远程额度 <strong>{run.remote_used}/{run.remote_budget}</strong></span><span>checkpoint <strong>{run.checkpoint || "—"}</strong></span></div>
      {run.error_summary && <div className="theater-banner error">{run.error_summary}</div>}
      {run.plan && <details open={run.status === "awaiting_plan_approval"}><summary>研究计划 · {run.plan.target_period}</summary><div className="init-question-list">{(run.plan.questions || []).map((question) => <article key={question.id}><strong>{question.question}</strong><small>{question.priority || "normal"} · {(question.topics || []).join(" / ")}</small></article>)}</div>{run.status === "awaiting_plan_approval" && <button className="primary" onClick={() => onAction("approve-plan", () => api.approveRolePlan(run.id))}>确认计划并开始自主研究</button>}</details>}
      <details open={run.status === "collecting" || run.status === "analyzing"}><summary>来源地图 · {run.assessments?.length || 0} 份</summary><div className="init-source-map">{(run.assessments || []).map((source) => <span key={source.source_id} className={source.audience}><strong>{source.tier}</strong><small>{source.status} · {source.audience === "director" ? "仅安可知" : "角色可知"} · 已读 {source.read_chunk_ids?.length || 0}</small></span>)}</div></details>
      <details open><summary>资料覆盖矩阵</summary><div className="coverage-grid">{coverageItems.map((item) => <span key={item.dimension} className={item.state}><strong>{item.dimension}</strong><small>{item.summary || item.state}</small></span>)}</div></details>
      {(run.conflicts?.length || 0) > 0 && <details open><summary>冲突与未知</summary>{run.conflicts?.map((conflict) => <p key={conflict.id}>{conflict.topic} · {conflict.disposition || "尚未处理"}</p>)}</details>}
      <details><summary>加入长文资料</summary><div className="init-upload"><input type="file" accept=".md,.txt,.pdf,text/plain,text/markdown,application/pdf" onChange={(event) => setDocumentFile(event.target.files?.[0])} /><select value={documentAudience} onChange={(event) => setDocumentAudience(event.target.value as "actor" | "director")}><option value="actor">角色可知</option><option value="director">仅安可知</option></select><select value={documentTier} onChange={(event) => setDocumentTier(event.target.value)}><option value="primary">一手材料</option><option value="contemporary">同时代材料</option><option value="biography">传记</option><option value="scholarship">学术研究</option><option value="posthumous">后世评价</option></select><button disabled={!documentFile || Boolean(busy)} onClick={() => documentFile && onAction("init-document", async () => api.addRoleInitializationDocument(run.id, { title: documentFile.name, mime_type: documentFile.type || "text/plain", content_base64: await fileAsBase64(documentFile), audience: documentAudience, tier: documentTier }))}><Upload size={14} />加入语料库</button></div></details>
      {blueprint && <details open={run.status === "awaiting_final_approval" || run.status === "blueprinting"}><summary>角色塑造方案 v{blueprint.version}</summary><div className="blueprint-sections"><p><strong>时期</strong>{blueprint.target_period} · 截止 {blueprint.knowledge_cutoff}</p>{sections.map((section, index) => <article key={`${section.key}-${index}`}><strong>{section.key}</strong><p>{section.content}</p><small>Claim {section.claim_ids?.length || 0} · Chunk {section.chunk_ids?.length || 0}</small></article>)}</div></details>}
      {critique && <details open><summary>Critic 审查 · {critique.passed ? "通过" : "存在硬错误"}</summary><div className="critique-list">{(critique.issues || []).map((issue, index) => <span key={`${issue.code}-${index}`} className={issue.severity}><strong>{issue.severity === "hard" ? "硬错误" : "警告"} · {issue.code}</strong><small>{issue.message}</small></span>)}{!critique.issues?.length && <span className="passed">没有发现结构性问题。</span>}</div></details>}
      {run.status === "needs_budget" && <button onClick={() => onAction("budget", () => api.grantRoleBudget(run.id))}>追加 12 次研究额度</button>}
      {run.status === "paused" ? <button onClick={() => onAction("resume-init", () => api.resumeRoleInitialization(run.id))}>恢复培养</button> : !["completed", "cancelled", "awaiting_plan_approval", "awaiting_final_approval"].includes(run.status) && <button onClick={() => onAction("pause-init", () => api.pauseRoleInitialization(run.id))}>暂停培养</button>}
      {(run.status === "blueprinting" || run.status === "awaiting_final_approval") && <div className="init-review-actions"><textarea rows={2} value={revision} onChange={(event) => setRevision(event.target.value)} placeholder="告诉安希望如何修订方案…" /><button disabled={!revision.trim()} onClick={() => onAction("revision", () => api.requestRoleBlueprintRevision(run.id, revision))}>要求修订</button></div>}
      {run.status === "awaiting_final_approval" && <div className="init-final"><input value={warningReason} onChange={(event) => setWarningReason(event.target.value)} placeholder={warnings.length ? "有普通警告：填写接受理由后上架" : "无警告时可留空"} /><button className="primary" disabled={hardIssues.length > 0 || (warnings.length > 0 && !warningReason.trim())} onClick={() => onAction("approve-blueprint", () => api.approveRoleBlueprint(run.id, warningReason))}>确认 Blueprint 并上架</button></div>}
    </section>
  );
}

function RoleLibrary() {
  const queryClient = useQueryClient();
  const roles = useQuery({ queryKey: ["roles"], queryFn: api.roles });
  const [selected, setSelected] = useState("");
  const roleItems = roles.data?.roles || [];
  const selectedID = selected || roleItems[0]?.id || "";
  const detail = useQuery({ queryKey: ["role", selectedID], queryFn: () => api.role(selectedID), enabled: Boolean(selectedID) });
  const initializations = useQuery({ queryKey: ["role-initializations", selectedID], queryFn: () => api.roleInitializations(selectedID), enabled: Boolean(selectedID), refetchInterval: 3000 });
  const initializationRuns = initializations.data?.runs || [];
  const initializationID = initializationRuns[0]?.id || "";
  const initialization = useQuery({ queryKey: ["role-initialization", initializationID], queryFn: () => api.roleInitialization(initializationID), enabled: Boolean(initializationID), refetchInterval: 2500 });
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [kind, setKind] = useState<"character" | "professional">("character");
  const [subject, setSubject] = useState("fictional");
  const [autonomous, setAutonomous] = useState(true);
  const [objective, setObjective] = useState("");
  const [targetPeriod, setTargetPeriod] = useState("");
  const [knowledgeCutoff, setKnowledgeCutoff] = useState("");
  const [privateConsent, setPrivateConsent] = useState(false);
  const [sourceTitle, setSourceTitle] = useState("");
  const [sourceText, setSourceText] = useState("");
  const [sourceFile, setSourceFile] = useState<File>();
  const [sourceURL, setSourceURL] = useState("");
  const [sourceAudience, setSourceAudience] = useState<"actor" | "director">("actor");

  const refresh = async (id?: string) => {
    await queryClient.invalidateQueries({ queryKey: ["roles"] });
    await queryClient.invalidateQueries({ queryKey: ["active-role"] });
    if (id) await queryClient.invalidateQueries({ queryKey: ["role", id] });
    await queryClient.invalidateQueries({ queryKey: ["role-initializations"] });
    await queryClient.invalidateQueries({ queryKey: ["role-initialization"] });
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
      const result = autonomous ? await api.startRoleInitialization({
        display_name: name, kind, subject_class: subject, objective: objective || `研究并塑造`,
        target_period: targetPeriod, knowledge_cutoff: knowledgeCutoff, private_model_consent: privateConsent,
        description: kind === "professional" ? "由安自主培养的专业角色" : "由 Character Architect 培养的角色",
      }) : await api.createRole({
        display_name: name, kind, subject_class: subject,
        description: kind === "professional" ? "持续工作的专业角色" : "待资料培养的角色",
        allowed_tools: kind === "professional" ? ["list_workspace", "read_workspace", "write_workspace"] : [],
      });
      setName(""); setObjective(""); setTargetPeriod(""); setKnowledgeCutoff(""); setShowCreate(false); setSelected(result.role.id); await refresh(result.role.id);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "创建失败"); }
    finally { setBusy(""); }
  };
  const current = detail.data?.role;
  const sources = detail.data?.sources || [];
  const claims = detail.data?.claims || [];
  const worldlines = detail.data?.worldlines || [];
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
          <label>培养方式<select value={autonomous ? "architect" : "manual"} onChange={(event) => setAutonomous(event.target.value === "architect")}><option value="architect">安自主研究与塑造</option><option value="manual">旧版手动编译</option></select></label>
          <label>角色类型<select value={kind} onChange={(event) => setKind(event.target.value as "character" | "professional")}><option value="character">人物角色</option><option value="professional">工作角色</option></select></label>
          <label>人物性质<select value={subject} onChange={(event) => setSubject(event.target.value)} disabled={kind === "professional"}><option value="fictional">虚构</option><option value="deceased">已故历史人物</option><option value="living_public">在世公众人物</option><option value="living_private">私人个体</option></select></label>
          {kind === "professional" && <span className="role-tool-note">仅授权版本化 Workspace 读写，不含发布、外部通信或删除。</span>}
          {autonomous && <><label>目标时期<input value={targetPeriod} onChange={(event) => setTargetPeriod(event.target.value)} placeholder="例如：成熟期 1920—1937" /></label><label>研究目标<input value={objective} onChange={(event) => setObjective(event.target.value)} placeholder="希望安重点理解什么" /></label><label>知识截止<input value={knowledgeCutoff} onChange={(event) => setKnowledgeCutoff(event.target.value)} placeholder="可由研究计划确定" /></label>{subject === "living_private" && <label className="consent-check"><input type="checkbox" checked={privateConsent} onChange={(event) => setPrivateConsent(event.target.checked)} />允许将私人资料发送给当前模型；不会自动联网</label>}</>}
          <button disabled={!name.trim() || Boolean(busy)}>创建草稿</button>
        </form>
      )}
      <div className="role-library-layout">
        <section className="role-shelf">
          <header><Library size={17} /><span>可选择的角色</span><small>{roleItems.length}</small></header>
          <div className="role-card-grid">
            {roleItems.map((role) => (
              <button key={role.id} className={selectedID === role.id ? "role-card selected" : "role-card"} onClick={() => setSelected(role.id)}>
                <span className="role-avatar">{role.kind === "professional" ? <UserRoundCog /> : <Drama />}</span>
                <span><strong>{role.display_name}</strong><small>{roleTypeLabel(role)}</small></span>
                <em>{statusLabel(role)}</em>
              </button>
            ))}
            {!roles.isLoading && roleItems.length === 0 && <div className="role-empty">还没有角色。先培养第一个完全受控的虚构角色。</div>}
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
              <div><dt>来源</dt><dd>{sources.length} 份</dd></div>
              <div><dt>主张</dt><dd>{claims.length} 条</dd></div>
              <div><dt>世界线</dt><dd>{worldlines.length} 条</dd></div>
            </dl>
            {initialization.data && <InitializationPanel detail={initialization.data} busy={busy} onAction={(name, fn) => act(name, fn, current.id)} />}
            {(sources.length) > 0 && (
              <div className="role-source-list">
                {sources.map((source) => (
                  <span key={source.id}><strong>{source.title}</strong><small>{source.audience === "director" ? "仅安可知 · 后世研究" : "角色可知 · 编入模型"}</small></span>
                ))}
              </div>
            )}
            {current.status !== "ready" && !initialization.data && <>
              <div className="source-editor">
                <label>资料用途<select value={sourceAudience} onChange={(event) => setSourceAudience(event.target.value as "actor" | "director")}><option value="actor">角色可知 · 编入人物模型</option><option value="director">仅安可知 · 后世评价/研究</option></select></label>
                <label>资料标题<input value={sourceTitle} onChange={(event) => setSourceTitle(event.target.value)} placeholder="例如：人物小传第一章" /></label>
                <label>文件（Markdown、纯文本或带文本层 PDF）<input type="file" accept=".md,.txt,.pdf,text/plain,text/markdown,application/pdf" onChange={(event) => { const file = event.target.files?.[0]; setSourceFile(file); if (file && !sourceTitle) setSourceTitle(file.name); }} /></label>
                <label>或者添加公开网页 URL<input type="url" value={sourceURL} onChange={(event) => setSourceURL(event.target.value)} placeholder="https://…" /></label>
                <label>Markdown / 纯文本<textarea rows={6} value={sourceText} onChange={(event) => setSourceText(event.target.value)} placeholder="把你希望安研究的资料放在这里…" /></label>
                <button onClick={() => act("source", async () => api.addRoleSource(current.id, sourceFile ? { title: sourceTitle, kind: "upload", audience: sourceAudience, mime_type: sourceFile.type || "application/octet-stream", content_base64: await fileAsBase64(sourceFile) } : sourceText.trim() ? { title: sourceTitle, kind: "upload", audience: sourceAudience, mime_type: "text/plain", content: sourceText } : { title: sourceTitle || sourceURL, kind: "webpage", audience: sourceAudience, url: sourceURL }), current.id)} disabled={(!sourceTitle.trim() && !sourceURL.trim()) || (!sourceFile && !sourceText.trim() && !sourceURL.trim()) || Boolean(busy)}><Upload size={14} />加入资料</button>
              </div>
              <div className="role-build-actions">
                <button onClick={() => act("compile", () => api.compileRole(current.id), current.id)} disabled={!sources.length || Boolean(busy)}><WandSparkles size={14} />让安编译角色</button>
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
