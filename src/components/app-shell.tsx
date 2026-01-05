import { NavLink, Outlet, useLocation, useNavigate, useParams } from "react-router-dom";
import { Terminal, MessageSquareText, Users, Server, Moon, Sun, ArrowLeft } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useRcon } from "@/lib/rcon-context";

type NavItem = { to: string; label: string; icon: React.ReactNode; end?: boolean };

export function AppHeader({
  subtitle,
  right,
}: {
  subtitle: React.ReactNode;
  right: React.ReactNode;
}) {
  return (
    <div className="z-10 shrink-0 border-b bg-background/80 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-3 px-4 py-4">
        <div className="flex items-center gap-3">
          <div className="min-w-0">
            <div className="truncate text-lg font-semibold">RustPanel</div>
            {subtitle}
          </div>
        </div>
        <div className="flex items-center gap-2">{right}</div>
      </div>
    </div>
  );
}

export function AppFooter() {
  return (
    <footer className="shrink-0 border-t">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-3 px-4 py-4 text-xs text-muted-foreground">
        <div>RustPanel v0.1 beta</div>
        <div>© {new Date().getFullYear()} CarlJoy</div>
      </div>
    </footer>
  );
}

export default function AppShell() {
  const { address, disconnect } = useRcon();
  const navigate = useNavigate();
  const location = useLocation();
  const { serverId } = useParams();
  const [theme, setTheme] = React.useState<"light" | "dark">(() =>
    document.documentElement.classList.contains("dark") ? "dark" : "light",
  );

  let effectiveServerId = serverId ?? null;
  if (!effectiveServerId) {
    try {
      const saved = localStorage.getItem("rustpanel-current-server-id");
      if (saved && /^\d+$/.test(saved)) effectiveServerId = saved;
    } catch {}
  }

  const navItems: NavItem[] = [
    { to: effectiveServerId ? `/server/${effectiveServerId}` : "/server", label: "服务器", icon: <Server className="h-4 w-4" />, end: true },
    { to: effectiveServerId ? `/server/${effectiveServerId}/console` : "/console", label: "控制台", icon: <Terminal className="h-4 w-4" /> },
    { to: effectiveServerId ? `/server/${effectiveServerId}/chat` : "/chat", label: "聊天", icon: <MessageSquareText className="h-4 w-4" /> },
    { to: effectiveServerId ? `/server/${effectiveServerId}/players` : "/players", label: "玩家", icon: <Users className="h-4 w-4" /> },
  ];

  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <AppHeader
        subtitle={
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <span>v0.1 beta</span>
            {address ? (
              <>
                <Separator orientation="vertical" className="h-4" />
                <span className="truncate">{address}</span>
              </>
            ) : null}
          </div>
        }
        right={
          <>
            <Button
              variant="secondary"
              size="icon"
              aria-label={theme === "dark" ? "切换到浅色模式" : "切换到深色模式"}
              onClick={() => {
                const next = theme === "dark" ? "light" : "dark";
                document.documentElement.classList.toggle("dark", next === "dark");
                setTheme(next);
                try {
                  localStorage.setItem("rustpanel-theme", next);
                } catch {}
              }}
            >
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
            <Button
              variant="secondary"
              size="icon"
              aria-label="返回服务器列表"
              onClick={() => {
                disconnect();
                navigate("/servers");
              }}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
          </>
        }
      />

      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-4 overflow-hidden px-4 py-4 md:flex-row">
        <aside className="shrink-0 md:w-[220px]">
          <nav className="flex gap-2 overflow-x-auto md:flex-col md:overflow-visible">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    "inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground",
                    isActive || location.pathname === item.to ? "bg-accent text-accent-foreground" : "",
                  )
                }
              >
                {item.icon}
                {item.label}
              </NavLink>
            ))}
          </nav>
        </aside>

        <main className="panel-scroll min-w-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>

      <AppFooter />
    </div>
  );
}
