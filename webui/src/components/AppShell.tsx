import { BrainCircuit, Database, Drama, MessageCircleMore, Sparkles } from "lucide-react";
import { NavLink } from "react-router-dom";
import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { ReactNode } from "react";
import { useTurnStore } from "../store";
import { traceToLiveTurn } from "../turn";

const nav = [
  { to: "/", label: "对话", icon: MessageCircleMore, end: true },
  { to: "/memory", label: "记忆", icon: Database },
  { to: "/mind", label: "心智", icon: BrainCircuit },
  { to: "/roles", label: "角色", icon: Drama },
];

export function AppShell({ children }: { children: ReactNode }) {
  const runtime = useQuery({ queryKey: ["bootstrap"], queryFn: api.bootstrap, refetchInterval: 30_000 });
  const { putTurn } = useTurnStore();
  useEffect(() => {
    runtime.data?.turns?.forEach((trace) => {
      if (trace.turn_id) putTurn(traceToLiveTurn(trace));
    });
  }, [runtime.data]);
  const mode = runtime.data?.runtime?.recall_mode;
  return (
    <div className="app-shell">
      <div className="ambient-field" aria-hidden="true"><i /><i /><i /></div>
      <header className="app-header">
        <NavLink to="/" className="brand" aria-label="Deep Seeing 首页">
          <span className="living-orbit"><Sparkles size={15} /></span>
          <span><strong>Deep Seeing</strong><small>让时间留下理解</small></span>
        </NavLink>
        <nav className="primary-nav" aria-label="主要空间">
          {nav.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={to} to={to} end={end} className={({ isActive }) => isActive ? "active" : ""}>
              <Icon size={17} /><span>{label}</span>
            </NavLink>
          ))}
        </nav>
        <div className="runtime-pill" title={runtime.error instanceof Error ? runtime.error.message : "当前运行状态"}>
          <i className={runtime.isError ? "danger" : runtime.isFetching ? "working" : "alive"} />
          <span>{runtime.isError ? "部分离线" : mode === "agent" ? "自主召回" : "稳定模式"}</span>
        </div>
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
}
