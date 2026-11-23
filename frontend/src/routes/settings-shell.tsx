import * as React from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { ArrowLeft, Database, HardDrive, LogOut, Moon, Settings, Sun, Users, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AppFooter, AppHeader } from "@/components/app-shell";
import { cn } from "@/lib/utils";
import { clearToken } from "@/lib/auth";

type NavItem = { to: string; label: string; icon: React.ReactNode; end?: boolean };

export default function SettingsShell() {
  const navigate = useNavigate();
  const location = useLocation();
  const [theme, setTheme] = React.useState<"light" | "dark">(() =>
    document.documentElement.classList.contains("dark") ? "dark" : "light",
  );

  const navItems: NavItem[] = [
    { to: "/settings/accounts", label: "账号管理", icon: <Users className="h-4 w-4" /> },
    { to: "/settings/servers", label: "服务器管理", icon: <Server className="h-4 w-4" /> },
    { to: "/settings/container", label: "容器配置", icon: <HardDrive className="h-4 w-4" /> },
    { to: "/settings/player-data", label: "玩家数据", icon: <Database className="h-4 w-4" /> },
  ];

  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <AppHeader
        subtitle={
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <Settings className="h-3.5 w-3.5" />
            <span>系统设置</span>
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
                navigate("/servers");
              }}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <Button
              variant="secondary"
              size="icon"
              aria-label="退出登录"
              onClick={() => {
                clearToken();
                navigate("/login", { replace: true });
              }}
            >
              <LogOut className="h-4 w-4" />
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
                    "inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground",
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
