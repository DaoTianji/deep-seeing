import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TurnStudio } from "./TurnStudio";
import type { LiveTurn } from "../types";

const turn: LiveTurn = {
  id: "turn-1", userText: "我们之前约定了什么？", answer: "采用一条证据", status: "complete", startedAt: "2026-08-28T10:00:00Z",
  events: [
    { id: "start", type: "start", offset: 0, label: "开始理解", data: {} },
    { id: "candidate", type: "context_candidate", offset: 10, label: "找到候选", source: "episode", state: "candidate", data: { source: "episode", result_ids: ["ep-1"] } },
    { id: "read", type: "context_read", offset: 20, label: "读取经历", source: "episode", objectId: "ep-1", state: "read", data: { source: "episode", id: "ep-1" } },
    { id: "used", type: "context_use", offset: 30, label: "采用经历", source: "episode", objectId: "ep-1", state: "used", data: { source: "episode", id: "ep-1", disposition: "used" } },
  ],
  attention: { items: [{ source: "episode", id: "ep-1", tier: "center" }] },
};

describe("TurnStudio", () => {
  it("renders synchronized public timeline, attention and evidence", () => {
    render(<TurnStudio turn={turn} />);
    expect(screen.getByText("这一轮，发生了什么")).toBeInTheDocument();
    expect(screen.getByText("此刻，什么进入意识")).toBeInTheDocument();
    expect(screen.getAllByText("ep-1").length).toBeGreaterThan(0);
    expect(screen.getByText("采用")).toBeInTheDocument();
    expect(screen.getByText(/非隐藏思维/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("回放时间"), { target: { value: "10" } });
    expect(screen.getAllByText("找到候选").length).toBeGreaterThan(0);
  });
});
