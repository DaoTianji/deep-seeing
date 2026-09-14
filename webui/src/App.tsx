import { lazy, Suspense, useEffect } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { ConversationPage } from "./pages/ConversationPage";
import { TurnStoreProvider } from "./store";
import { ReadingAccess } from "./pages/ReadingAccess";
import { ReadingWelcome } from "./pages/ReadingWelcome";

const TurnPage = lazy(() => import("./pages/TurnPage").then((module) => ({ default: module.TurnPage })));
const MemoryPage = lazy(() => import("./pages/MemoryPage").then((module) => ({ default: module.MemoryPage })));
const MindPage = lazy(() => import("./pages/MindPage").then((module) => ({ default: module.MindPage })));
const RolePage = lazy(() => import("./pages/RolePage").then((module) => ({ default: module.RolePage })));
const ReadingPage = lazy(() => import("./pages/ReadingPage"));

export function App() {
  const location = useLocation();
  const isReading = location.pathname.startsWith("/reading");
  useEffect(() => {
    if (!isReading) return;
    const previousTitle = document.title;
    document.title = "书中见";
    return () => { document.title = previousTitle; };
  }, [isReading]);
  if (isReading && !new URLSearchParams(location.search).has("book") && !new URLSearchParams(location.search).has("view")) return <ReadingWelcome/>;
  if (location.pathname.startsWith("/reading")) return <ReadingAccess><Suspense fallback={<div className="page-state">正在加载书中见…</div>}><ReadingPage /></Suspense></ReadingAccess>;
  return (
    <TurnStoreProvider>
      <AppShell>
        <Suspense fallback={<div className="page-state">正在打开这个心智空间…</div>}>
          <Routes>
            <Route path="/" element={<ConversationPage />} />
            <Route path="/turn/:turnId" element={<TurnPage />} />
            <Route path="/memory" element={<MemoryPage />} />
            <Route path="/mind" element={<MindPage />} />
            <Route path="/roles" element={<RolePage />} />
            <Route path="/theater" element={<RolePage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </AppShell>
    </TurnStoreProvider>
  );
}
