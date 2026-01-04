import { Outlet, NavLink, useLocation, useNavigate } from "react-router-dom";
import { Terminal, MessageSquareText, Users, Server } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useRcon } from "@/lib/rcon-context";

type NavItem = { to: string; label: string; icon: React.ReactNode };

const navItems: NavItem[] = [
  { to: "/server", label: "服务器", icon: <Server className="h-4 w-4" /> },
  { to: "/console", label: "控制台", icon: <Terminal className="h-4 w-4" /> },
  { to: "/chat", label: "聊天", icon: <MessageSquareText className="h-4 w-4" /> },
  { to: "/players", label: "玩家", icon: <Users className="h-4 w-4" /> },
];

export default function AppShell() {
  const { address, disconnect } = useRcon();
  const navigate = useNavigate();
  const location = useLocation();

  return (
    <div className="min-h-dvh">
      <div className="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
          <div className="flex items-center gap-3">
            <div className="text-sm font-semibold">WebRcon</div>
            <Separator orientation="vertical" className="h-5" />
            <div className="text-xs text-muted-foreground">{address}</div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              onClick={() => {
                disconnect();
                navigate("/servers");
              }}
            >
              断开连接
            </Button>
          </div>
        </div>
      </div>

      <div className="mx-auto grid max-w-7xl grid-cols-1 gap-4 px-4 py-4 md:grid-cols-[220px_1fr]">
        <aside className="md:sticky md:top-[72px] md:h-[calc(100dvh-88px)]">
          <nav className="flex gap-2 md:flex-col">
            {navItems.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
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

        <main className="min-w-0">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
