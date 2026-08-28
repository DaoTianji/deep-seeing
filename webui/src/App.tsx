import { lazy, Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ConversationPage } from "./pages/ConversationPage";
import { TurnStoreProvider } from "./store";

const TurnPage = lazy(() => import("./pages/TurnPage").then((module) => ({ default: module.TurnPage })));
const MemoryPage = lazy(() => import("./pages/MemoryPage").then((module) => ({ default: module.MemoryPage })));
const MindPage = lazy(() => import("./pages/MindPage").then((module) => ({ default: module.MindPage })));

export function App() {
  return (
    <TurnStoreProvider>
      <AppShell>
        <Suspense fallback={<div className="page-state">正在打开这个心智空间…</div>}>
          <Routes>
            <Route path="/" element={<ConversationPage />} />
            <Route path="/turn/:turnId" element={<TurnPage />} />
            <Route path="/memory" element={<MemoryPage />} />
            <Route path="/mind" element={<MindPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </AppShell>
    </TurnStoreProvider>
  );
}
