import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import type { LiveTurn } from "./types";

interface TurnStoreValue {
  turns: Map<string, LiveTurn>;
  putTurn: (turn: LiveTurn) => void;
  getTurn: (id: string) => LiveTurn | undefined;
}

const TurnStore = createContext<TurnStoreValue | null>(null);

export function TurnStoreProvider({ children }: { children: ReactNode }) {
  const [turns, setTurns] = useState<Map<string, LiveTurn>>(() => new Map());
  const value = useMemo<TurnStoreValue>(() => ({
    turns,
    putTurn: (turn) => setTurns((current) => {
      const next = new Map(current);
      next.set(turn.id, turn);
      return next;
    }),
    getTurn: (id) => turns.get(id),
  }), [turns]);
  return <TurnStore.Provider value={value}>{children}</TurnStore.Provider>;
}

export function useTurnStore() {
  const value = useContext(TurnStore);
  if (!value) throw new Error("TurnStoreProvider missing");
  return value;
}
