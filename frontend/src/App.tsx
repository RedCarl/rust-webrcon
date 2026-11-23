import { Navigate, Route, Routes, useLocation, useParams } from "react-router-dom";
import AppShell from "@/components/app-shell";
import { RconProvider, useRcon } from "@/lib/rcon-context";
import ChatPage from "@/routes/chat-page";
import ConsolePage from "@/routes/console-page";
import LoginPage from "@/routes/login-page";
import PlayerPage from "@/routes/player-page";
import PlayersPage from "@/routes/players-page";
import ServerPage from "@/routes/server-page";
import ServersPage from "@/routes/servers-page";
import SettingsShell from "@/routes/settings-shell";
import SettingsAccountsPage from "@/routes/settings-accounts-page";
import SettingsServersPage from "@/routes/settings-servers-page";
import SettingsWipePage from "@/routes/settings-wipe-page";
import SettingsPlayerDataPage from "@/routes/settings-player-data-page";
import { getToken } from "@/lib/auth";

function RequireAuthed({ children }: { children: React.ReactNode }) {
  const token = getToken();
  if (token) return <>{children}</>;
  return <Navigate to="/login" replace />;
}

function RequireConnected({ children }: { children: React.ReactNode }) {
  const { status } = useRcon();
  const { serverId } = useParams();
  const location = useLocation();
  if (status === "open") return <>{children}</>;
  if (status === "connecting") {
    return (
      <div className="flex min-h-[50dvh] items-center justify-center">
        <div className="text-sm text-muted-foreground">连接中…</div>
      </div>
    );
  }
  if (serverId) {
    return <Navigate to={`/server/${serverId}`} replace state={{ from: location.pathname + location.search }} />;
  }
  return <Navigate to="/servers" replace />;
}

function RedirectWithSavedServerId({ to }: { to: (serverId: string) => string }) {
  let saved: string | null = null;
  try {
    saved = localStorage.getItem("rustpanel-current-server-id");
  } catch {}
  if (!saved || !/^\d+$/.test(saved)) return <Navigate to="/servers" replace />;
  return <Navigate to={to(saved)} replace />;
}

function RedirectPlayerWithSavedServerId() {
  const { steamId } = useParams();
  const encoded = encodeURIComponent(String(steamId ?? ""));
  return <RedirectWithSavedServerId to={(id) => `/server/${id}/players/${encoded}`} />;
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
          path="/console"
          element={
            <RequireAuthed>
              <RedirectWithSavedServerId to={(id) => `/server/${id}/console`} />
            </RequireAuthed>
          }
        />
        <Route
          path="/chat"
          element={
            <RequireAuthed>
              <RedirectWithSavedServerId to={(id) => `/server/${id}/chat`} />
            </RequireAuthed>
          }
        />
        <Route
          path="/players"
          element={
            <RequireAuthed>
              <RedirectWithSavedServerId to={(id) => `/server/${id}/players`} />
            </RequireAuthed>
          }
        />
        <Route
          path="/players/:steamId"
          element={
            <RequireAuthed>
              <RedirectPlayerWithSavedServerId />
            </RequireAuthed>
          }
        />
        <Route
          path="/settings"
          element={
            <RequireAuthed>
              <SettingsShell />
            </RequireAuthed>
          }
        >
          <Route index element={<Navigate to="accounts" replace />} />
          <Route path="accounts" element={<SettingsAccountsPage />} />
          <Route path="servers" element={<SettingsServersPage />} />
          <Route path="container" element={<SettingsWipePage />} />
          <Route path="player-data" element={<SettingsPlayerDataPage />} />
          <Route path="wipe" element={<SettingsWipePage />} />
        </Route>
        <Route
          element={
            <RequireAuthed>
              <AppShell />
            </RequireAuthed>
          }
        >
          <Route index element={<Navigate to="/servers" replace />} />
          <Route path="/server" element={<ServerPage />} />
          <Route path="/server/:serverId" element={<ServerPage />} />
          <Route
            path="/server/:serverId/console"
            element={
              <RequireConnected>
                <ConsolePage />
              </RequireConnected>
            }
          />
          <Route
            path="/server/:serverId/chat"
            element={
              <RequireConnected>
                <ChatPage />
              </RequireConnected>
            }
          />
          <Route
            path="/server/:serverId/players"
            element={
              <RequireConnected>
                <PlayersPage />
              </RequireConnected>
            }
          />
          <Route
            path="/server/:serverId/players/:steamId"
            element={
              <RequireConnected>
                <PlayerPage />
              </RequireConnected>
            }
          />
        </Route>
        <Route path="/connect" element={<Navigate to="/servers" replace />} />
        <Route path="*" element={<Navigate to={token ? "/servers" : "/login"} replace />} />
      </Routes>
    </RconProvider>
  );
}
