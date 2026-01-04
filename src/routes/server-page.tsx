import * as React from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { Cpu, HardDrive, Network, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";
import { useRcon } from "@/lib/rcon-context";
import { apiJson, clearToken } from "@/lib/auth";
import { toast } from "sonner";

type ServerInfo = Record<string, unknown> & {
  Hostname?: string;
  Map?: string;
  Players?: number;
  MaxPlayers?: number;
  Queued?: number;
  Joining?: number;
  Framerate?: number;
  EntityCount?: number;
  NetworkIn?: number;
  NetworkOut?: number;
  Uptime?: number;
  Memory?: number;
  MemoryUsageSystem?: number;
};

type ServerDto = {
  id: number;
  name: string;
  host: string;
  port: number;
  rcon_password: string;
};

function formatBytes(bytes?: number) {
  if (bytes === undefined || bytes === null) return "-";
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)}kB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)}MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)}GB`;
}

function formatMemory(input?: number) {
  if (input === undefined || input === null || Number.isNaN(input)) return "-";
  if (input > 1024 * 1024) return formatBytes(input);
  return `${Math.round(input)}MB`;
}

function formatDurationSeconds(input?: number) {
  if (input === undefined || input === null || Number.isNaN(input)) return "-";
  const seconds = Math.max(0, Math.floor(input));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor(seconds / 3600) % 24;
  const minutes = Math.floor(seconds / 60) % 60;
  const sec = seconds % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  parts.push(`${hours}h`);
  parts.push(`${minutes}m`);
  parts.push(`${sec}s`);
  return parts.join(" ");
}

function formatValue(key: string, value: unknown) {
  if (value === null || value === undefined) return "-";
  if (key === "NetworkIn" || key === "NetworkOut") {
    if (typeof value === "number") return `${value} (${formatBytes(value)})`;
  }
  if (key === "Uptime") {
    if (typeof value === "number") return `${value} (${formatDurationSeconds(value)})`;
  }
  if (typeof value === "string") return value;
  if (typeof value === "number") return Number.isFinite(value) ? String(value) : "-";
  if (typeof value === "boolean") return value ? "true" : "false";
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function labelForKey(key: string) {
  const map: Record<string, string> = {
    Hostname: "服务器名称",
    Map: "地图",
    Players: "在线玩家",
    MaxPlayers: "最大玩家",
    Queued: "排队人数",
    Joining: "加入中",
    ReservedSlots: "预留位",
    EntityCount: "实体数量",
    GameTime: "游戏时间",
    Uptime: "运行时长",
    Framerate: "帧率",
    Memory: "内存(MB)",
    MemoryUsageSystem: "系统内存占用",
    Collections: "GC次数",
    NetworkIn: "网络入",
    NetworkOut: "网络出",
    Restarting: "正在重启",
    SaveCreatedTime: "存档时间",
    Version: "版本",
    Protocol: "协议",
  };
  return map[key] ?? key;
}

export default function ServerPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { serverId } = useParams();
  const { status, address, pendingConnection, connect, request, lastError } = useRcon();
  const [info, setInfo] = React.useState<ServerInfo | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const lastNotifiedErrorRef = React.useRef<string | null>(null);
  const connectStartedRef = React.useRef(false);
  const bootstrappedIdRef = React.useRef(false);

  React.useEffect(() => {
    if (!error) return;
    if (lastNotifiedErrorRef.current === error) return;
    lastNotifiedErrorRef.current = error;
    toast.error(error);
  }, [error]);

  React.useEffect(() => {
    if (!lastError) return;
    if (lastNotifiedErrorRef.current === lastError) return;
    lastNotifiedErrorRef.current = lastError;
    toast.error(lastError);
  }, [lastError]);

  React.useEffect(() => {
    if (status !== "open") return;
    const from = (location.state as any)?.from;
    if (typeof from !== "string") return;
    if (!from.startsWith("/server/")) return;
    if (from === location.pathname + location.search) return;
    navigate(from, { replace: true, state: null });
  }, [location.pathname, location.search, location.state, navigate, status]);

  React.useEffect(() => {
    if (status !== "closed") return;
    if (connectStartedRef.current) return;

    if (!serverId && !bootstrappedIdRef.current) {
      bootstrappedIdRef.current = true;
      try {
        const saved = localStorage.getItem("rustpanel-current-server-id");
        if (saved && /^\d+$/.test(saved)) {
          navigate(`/server/${saved}`, { replace: true });
          return;
        }
      } catch {}
    }

    if (pendingConnection) {
      connectStartedRef.current = true;
      connect(pendingConnection.address, pendingConnection.password);
      return;
    }

    const id = serverId ? Number(serverId) : NaN;
    if (!Number.isFinite(id) || id <= 0) return;

    connectStartedRef.current = true;
    (async () => {
      try {
        const list = await apiJson<ServerDto[] | null>("/api/servers");
        const server = Array.isArray(list) ? list.find((s) => s.id === id) : null;
        if (!server) throw new Error("服务器不存在或已被删除");
        try {
          localStorage.setItem("rustpanel-current-server-id", String(server.id));
        } catch {}
        connect(`${server.host}:${server.port}`, server.rcon_password);
      } catch (e) {
        const msg = e instanceof Error ? e.message : "加载服务器失败";
        toast.error(msg);
        if (msg.includes("未登录")) {
          clearToken();
          navigate("/login", { replace: true });
          return;
        }
        connectStartedRef.current = false;
      }
    })();
  }, [connect, navigate, pendingConnection, serverId, status]);

  React.useEffect(() => {
    let alive = true;
    if (status !== "open") {
      setInfo(null);
      setError(null);
      return () => {
        alive = false;
      };
    }

    const refresh = async () => {
      try {
        const packet = await request("serverinfo");
        const parsed = JSON.parse(packet.Message) as ServerInfo;
        if (!alive) return;
        setInfo(parsed);
        setError(null);
      } catch (e) {
        if (!alive) return;
        setError(e instanceof Error ? e.message : "刷新失败");
      }
    };

    refresh();
    const timer = window.setInterval(refresh, 1000);
    return () => {
      alive = false;
      window.clearInterval(timer);
    };
  }, [request, status]);

  if (status !== "open") {
    return (
      <div className="grid gap-4">
        <div className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="truncate text-lg font-semibold">服务器</div>
            <div className="text-sm text-muted-foreground">
              {status === "connecting"
                ? `连接中：${address ?? pendingConnection?.address ?? "-"}`
                : "请从服务器列表选择服务器后进入管理"}
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="secondary">{status === "connecting" ? "连接中" : "未连接"}</Badge>
            <Button variant="secondary" onClick={() => navigate("/servers")}>
              返回列表
            </Button>
          </div>
        </div>

        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {status === "connecting" ? "正在建立连接…" : "暂无连接"}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-lg font-semibold">{info?.Hostname ?? "服务器"}</div>
          <div className="text-sm text-muted-foreground">{info?.Map ? `地图：${info.Map}` : null}</div>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="secondary">实时</Badge>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Users className="h-4 w-4 text-muted-foreground" />
              在线
            </CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">
            {info?.Players ?? "-"} / {info?.MaxPlayers ?? "-"}
            <div className="mt-2 text-sm font-normal text-muted-foreground">
              排队：{info?.Queued ?? "-"}，加入中：{info?.Joining ?? "-"}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Cpu className="h-4 w-4 text-muted-foreground" />
              性能
            </CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">
            {info?.Framerate ?? "-"} FPS
            <div className="mt-2 text-sm font-normal text-muted-foreground">实体：{info?.EntityCount ?? "-"}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Network className="h-4 w-4 text-muted-foreground" />
              网络
            </CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">
            {formatBytes(info?.NetworkIn)} / {formatBytes(info?.NetworkOut)}
            <div className="mt-2 text-sm font-normal text-muted-foreground">入 / 出</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <HardDrive className="h-4 w-4 text-muted-foreground" />
              内存
            </CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-semibold">
            {formatMemory(info?.Memory ?? info?.MemoryUsageSystem)}
            <div className="mt-2 text-sm font-normal text-muted-foreground">进程 / 系统</div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>全部数据</CardTitle>
        </CardHeader>
        <CardContent>
          {info ? (
            <div className="relative w-full overflow-x-auto">
              <Table>
                <TableBody>
                  {Object.entries(info)
                    .slice()
                    .sort(([a], [b]) => a.localeCompare(b))
                    .filter(([k]) => {
                      const shown = new Set([
                        "Hostname",
                        "Map",
                        "Players",
                        "MaxPlayers",
                        "Queued",
                        "Joining",
                        "Framerate",
                        "EntityCount",
                        "NetworkIn",
                        "NetworkOut",
                      ]);
                      return !shown.has(k);
                    })
                    .map(([k, v]) => (
                      <TableRow key={k}>
                        <TableCell className="w-[220px] text-sm text-muted-foreground">{labelForKey(k)}</TableCell>
                        <TableCell className="font-mono text-xs whitespace-pre-wrap break-words">{formatValue(k, v)}</TableCell>
                      </TableRow>
                    ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <div className="py-10 text-center text-sm text-muted-foreground">暂无数据</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
