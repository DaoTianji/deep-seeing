import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Brain, Check, CircleDot, CloudMoon, History, RotateCcw, Search, Sparkles } from "lucide-react";
import { api } from "../api";
import type { ReflectionDecision, ReflectionEvidence, ReflectionRun, ReflectionSeed } from "../reflection-types";
import "../reflection.css";

function compactId(id?: string) {
  if (!id) return "—";
  return id.length > 18 ? `${id.slice(0, 9)}…${id.slice(-5)}` : id;
}

function actionLabel(action?: string) {
  const labels: Record<string, string> = {
    no_change: "保持原样", defer: "等待更多经历", confirm: "得到确认", revise: "修订认识",
    supersede: "替代旧认识", open_tension: "保留张力", resolve_tension: "化解张力", reject_seed: "排除误判",
  };
  return labels[action || ""] || action || "尚未决定";
}

function evidenceLabel(item: ReflectionEvidence) {
  const labels: Record<string, string> = {
    support: "支持", conflict: "冲突", context: "补充背景", duplicate: "重复",
    stale: "已经过时", insufficient: "证据不足", superseded: "已被取代",
  };
  return labels[item.state] || item.state;
}

function RunTimeline({ run }: { run?: ReflectionRun }) {
  if (!run) return <div className="reflection-empty">还没有发生可回放的反思。</div>;
  const steps = [
    { icon: CircleDot, label: "触发", value: run.trigger || "opportunity", tone: "seed" },
    { icon: Brain, label: "选择问题", value: `${run.seed_ids?.length || 0} 个 Seed`, tone: "seed" },
    { icon: Search, label: "激活候选", value: `${run.candidate_ids?.length || 0} 条记忆`, tone: "candidate" },
    { icon: History, label: "阅读全文", value: `${run.read_ids?.length || 0} 条证据`, tone: "read" },
    { icon: Check, label: "形成结论", value: run.no_change ? "没有改变" : `${run.decisions?.length || 0} 个决定`, tone: "used" },
  ];
  return <div className="reflection-timeline">
    {steps.map(({ icon: Icon, label, value, tone }) => <article className={`reflection-step ${tone}`} key={label}><span><Icon size={14} /></span><small>{label}</small><strong>{value}</strong></article>)}
  </div>;
}

function EvidenceList({ evidence = [] }: { evidence?: ReflectionEvidence[] }) {
  if (evidence.length === 0) return <p className="reflection-empty">这次没有采用历史证据。</p>;
  return <div className="reflection-evidence">{evidence.map((item, index) => <article className={`evidence-${item.state}`} key={`${item.seed_id}-${item.episode_id}-${index}`}>
    <i /><span><strong>{evidenceLabel(item)}</strong><small>{compactId(item.episode_id)} · {item.epistemic === "derived_inference" ? "推断来源" : item.experience_mode || "经历"}</small></span>
  </article>)}</div>;
}

function DecisionList({ decisions = [] }: { decisions?: ReflectionDecision[] }) {
  if (decisions.length === 0) return <p className="reflection-empty">没有形成需要执行的改变。</p>;
  return <div className="reflection-decisions">{decisions.map((item, index) => <article key={`${item.seed_id}-${index}`}><header><strong>{actionLabel(item.action)}</strong><small>{item.kind || "reflection"}</small></header><p>{item.suggested_text || item.reason_summary || "保持未决"}</p>{item.mutation_id && <em>已写入 · {compactId(item.mutation_id)}</em>}{item.proposal_id && !item.mutation_id && <em>观察中 · 尚未写入</em>}</article>)}</div>;
}

export function ReflectionPanel({ seeds, mutations }: { seeds: ReflectionSeed[]; mutations: Record<string, unknown>[] }) {
  const queryClient = useQueryClient();
  const runtime = useQuery({ queryKey: ["runtime"], queryFn: api.runtime });
  const runs = useQuery({ queryKey: ["reflection-runs"], queryFn: api.reflectionRuns, refetchInterval: 12_000 });
  const live = useQuery({ queryKey: ["reflection-live"], queryFn: api.reflectionLive, refetchInterval: 750 });
  const latest = live.data?.running && live.data.run ? live.data.run : runs.data?.runs?.[0];
  const refresh = async () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ["reflections"] }),
    queryClient.invalidateQueries({ queryKey: ["reflection-runs"] }),
    queryClient.invalidateQueries({ queryKey: ["reflection-live"] }),
    queryClient.invalidateQueries({ queryKey: ["proposals"] }),
    queryClient.invalidateQueries({ queryKey: ["mutations"] }),
    queryClient.invalidateQueries({ queryKey: ["self"] }),
  ]);
  const review = useMutation({ mutationFn: api.review, onSuccess: refresh });
  const dream = useMutation({ mutationFn: api.dream, onSuccess: refresh });
  const generative = useMutation({ mutationFn: api.generativeDream, onSuccess: refresh });
  const rollback = useMutation({ mutationFn: (id: string) => api.revertMutation(id), onSuccess: refresh });
  const busy = Boolean(live.data?.running) || review.isPending || dream.isPending || generative.isPending || rollback.isPending;
  const mode = runtime.data?.runtime?.reflection_mode || "observe";

  return <section className="reflection-lab">
    <header><div><span className="eyebrow">REFLECTION & DREAM</span><h2>安如何重新理解经历</h2><p>这是可公开核对的证据轨迹，不是隐藏思维。紫色内容只是想象假设，不会被当成真实记忆。</p></div><span className={`reflection-mode mode-${mode}`}>{mode === "agent" ? "自主巩固" : mode === "observe" ? "观察模式" : "旧版回退"}</span></header>
    <div className="reflection-actions">
      <button className="soft-button" disabled={busy} onClick={() => review.mutate()}><Brain size={14} />回看当前会话</button>
      <button className="soft-button" disabled={busy} onClick={() => dream.mutate()}><Sparkles size={14} />证据型反思</button>
      <button className="soft-button dream-button" disabled={busy} onClick={() => generative.mutate()}><CloudMoon size={14} />生成式梦境</button>
      {busy && <small>安正在安静地整理 · {live.data?.phase || "preparing"}</small>}
    </div>
    <RunTimeline run={latest} />
    <div className="reflection-columns">
      <section><header><span className="eyebrow">OPEN QUESTIONS</span><h3>仍在心里的问题</h3><small>{seeds.length}</small></header><div className="seed-list">{seeds.slice(0, 8).map((seed) => <article className={seed.generated ? "generated" : ""} key={seed.id}><i /><span><strong>{seed.statement}</strong><small>{seed.generated ? "想象假设 · 尚待真实经历验证" : `${seed.scope} · ${seed.status}`}</small></span></article>)}{seeds.length === 0 && <p className="reflection-empty">此刻没有等待重新审视的问题。</p>}</div></section>
      <section><header><span className="eyebrow">EVIDENCE</span><h3>这次激活的证据</h3></header><EvidenceList evidence={latest?.evidence} /></section>
      <section><header><span className="eyebrow">DECISION</span><h3>安决定如何改变</h3></header><DecisionList decisions={latest?.decisions} /></section>
    </div>
    {latest?.generative_note && <blockquote className="dream-fragment"><CloudMoon size={17} /><span><small>想象片段，不是事实</small>{latest.generative_note}</span></blockquote>}
      {!!latest?.generated_seeds?.length && <div className="seed-list generated-seed-trace">{latest.generated_seeds.map(seed => <article className="generated" key={seed.id}><i /><span><strong>{seed.statement}</strong><small>想象假设 · {seed.scope} · 尚待真实经历验证</small></span></article>)}</div>}
    <div className="rollback-list"><header><span className="eyebrow">REVERSIBLE HISTORY</span><h3>最近的可撤销变化</h3></header>{mutations.slice(0, 5).map((item) => {
      const id = String(item.mutation_id || ""); const reverted = Boolean(item.reverts_mutation_id); const reversible = !reverted && ["bond_patch", "self_artifact"].includes(String(item.kind || ""));
      return <article key={id}><span><strong>{String(item.reason_summary || item.field || item.kind || "一次认识变化")}</strong><small>{compactId(id)} · {String(item.kind || "mutation")}</small></span>{reversible && <button disabled={busy} onClick={() => rollback.mutate(id)} title="以补偿记录撤销，不删除历史"><RotateCcw size={13} />撤销</button>}</article>;
    })}</div>
  </section>;
}
