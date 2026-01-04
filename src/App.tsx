import { Navigate, Route, Routes } from "react-router-dom";
import AppShell from "@/components/app-shell";
import { RconProvider, useRcon } from "@/lib/rcon-context";
import ChatPage from "@/routes/chat-page";
import ConsolePage from "@/routes/console-page";
import LoginPage from "@/routes/login-page";
import PlayerPage from "@/routes/player-page";
import PlayersPage from "@/routes/players-page";
import ServerPage from "@/routes/server-page";
import ServersPage from "@/routes/servers-page";
import { getToken } from "@/lib/auth";

function RequireAuthed({ children }: { children: React.ReactNode }) {
  const token = getToken();
  if (token) return <>{children}</>;
  return <Navigate to="/login" replace />;
}

function RequireConnected({ children }: { children: React.ReactNode }) {
  const { status, autoReconnectInProgress } = useRcon();
  if (status === "open") return <>{children}</>;
  if (status === "connecting" || autoReconnectInProgress) {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <div className="text-sm text-muted-foreground">正在重新连接…</div>
      </div>
    );
  }
  return <Navigate to="/servers" replace />;
}

export default function App() {
  const token = getToken();
  return (
    <RconProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/servers"
          element={
            <RequireAuthed>
              <ServersPage />
            </RequireAuthed>
          }
        />
        <Route
          element={
            <RequireConnected>
              <AppShell />
            </RequireConnected>
          }
        >
          <Route index element={<Navigate to="/server" replace />} />
          <Route path="/server" element={<ServerPage />} />
          <Route path="/console" element={<ConsolePage />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/players" element={<PlayersPage />} />
          <Route path="/players/:steamId" element={<PlayerPage />} />
        </Route>
        <Route path="/connect" element={<Navigate to="/servers" replace />} />
        <Route path="*" element={<Navigate to={token ? "/servers" : "/login"} replace />} />
      </Routes>
    </RconProvider>
  );
}
