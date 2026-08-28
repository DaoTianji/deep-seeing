import { describe, expect, it } from "vitest";
import { attentionAt, envelopeToEvent, evidenceFromEvents, traceToEvents } from "./turn";
import type { LiveTurn, TurnEvent, TurnTrace } from "./types";

describe("public turn event model", () => {
  it("keeps stable envelope identity and public timing", () => {
    const event = envelopeToEvent({ type: "context_read", turn_id: "t1", seq: 4, turn_offset_ns: 42, data: { source: "episode", id: "ep1", duration_ns: 7 } }, 0);
    expect(event.id).toBe("t1-4");
    expect(event.offset).toBe(42);
    expect(event.state).toBe("read");
    expect(event.objectId).toBe("ep1");
  });

  it("orders historical events by offset instead of storage grouping", () => {
    const trace: TurnTrace = {
      turn_id: "t1", duration_ns: 100,
      context_candidates: [{ source: "episode", result_ids: ["ep1"], turn_offset_ns: 40 }],
      context_reads: [{ source: "episode", id: "ep1", ok: true, turn_offset_ns: 60 }],
      context_uses: [{ source: "episode", id: "ep1", disposition: "used", turn_offset_ns: 80 }],
    };
    expect(traceToEvents(trace).map((event) => event.offset)).toEqual([0, 40, 60, 80, 100]);
  });

  it("uses lifecycle priority without treating unprocessed candidates as dismissed", () => {
    const base = (state: TurnEvent["state"], data: unknown): TurnEvent => ({ id: Math.random().toString(), type: "context", offset: 0, label: "", state, data });
    const evidence = evidenceFromEvents([
      base("candidate", { result_ids: ["ep1", "ep2"] }),
      base("read", { id: "ep1" }),
      base("used", { id: "ep1" }),
    ]);
    expect(evidence.get("ep1")).toBe("used");
    expect(evidence.get("ep2")).toBe("candidate");
  });

  it("replays attention moves and explicit replacement", () => {
    const turn: LiveTurn = {
      id: "t", userText: "", answer: "", status: "complete", startedAt: "",
      attention: { items: [{ source: "workspace", id: "w1", tier: "center" }] },
      events: [{ id: "e", type: "attention_decision", offset: 10, label: "", data: { source: "episode", id: "ep1", to: "center", replaced_source: "workspace", replaced_id: "w1" } }],
    };
    expect(attentionAt(turn, 9)?.items?.map((item) => item.id)).toEqual(["w1"]);
    expect(attentionAt(turn, 10)?.items?.map((item) => item.id)).toEqual(["ep1"]);
  });
});
