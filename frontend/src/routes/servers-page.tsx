import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Calendar as CalendarIcon, ChevronDownIcon, LogOut, Moon, Settings, Sun } from "lucide-react";
import { format } from "date-fns";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AppFooter, AppHeader } from "@/components/app-shell";
import { apiJson, clearToken } from "@/lib/auth";
import { useRcon } from "@/lib/rcon-context";
import { toast } from "sonner";
import { zhCN } from "date-fns/locale";

type ServerDto = {
  id: number;
  name: string;
  host: string;
  port: number;
  rcon_password: string;
  container_name?: string | null;
  last_wipe_at: string;
  always_use_today_wipe_date: boolean;
};

type ServerActionResponse = {
  ok: boolean;
  partial_success: boolean;
  stage: string;
  message: string;
  logs: string[];
  map_filename?: string;
  map_url?: string;
  web_rcon_error_message?: string;
};

function formatDateTime(s: string) {
  const text = (s ?? "").trim();
  if (!text) return "";
  const d = new Date(text);
  if (Number.isNaN(d.getTime())) return text;
  try {
    return d.toLocaleString("zh-CN", { hour12: false });
  } catch {
    return d.toLocaleString();
  }
}

function parseOptionalDateTime(value: string): Date | undefined {
  const text = (value ?? "").trim();
  if (!text) return undefined;
  const d = new Date(text);
  if (Number.isNaN(d.getTime())) return undefined;
  return d;
}

function formatWipeAtValue(date: Date, timeStr?: string): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  const t = timeStr ? timeStr : "00:00:00";
  return `${y}-${m}-${d}T${t}`;
}

function getWipeTime(value: string): string {
  const date = parseOptionalDateTime(value);
  if (!date) return "00:00:00";
  const h = String(date.getHours()).padStart(2, "0");
  const m = String(date.getMinutes()).padStart(2, "0");
  const s = String(date.getSeconds()).padStart(2, "0");
  return `${h}:${m}:${s}`;
}

export default function ServersPage() {
  const navigate = useNavigate();
  const { status, disconnect, clearPendingConnection, setPendingConnection } = useRcon();
  const [theme, setTheme] = React.useState<"light" | "dark">(() =>
    document.documentElement.classList.contains("dark") ? "dark" : "light",
  );
  const [servers, setServers] = React.useState<ServerDto[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [actionBusyId, setActionBusyId] = React.useState<number | null>(null);
  const [actionTitle, setActionTitle] = React.useState<string | null>(null);
  const [actionOutput, setActionOutput] = React.useState<string | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [createName, setCreateName] = React.useState("");
  const [createHost, setCreateHost] = React.useState("");
  const [createPort, setCreatePort] = React.useState("28016");
  const [createPassword, setCreatePassword] = React.useState("");
  const [createContainerName, setCreateContainerName] = React.useState("");

  const [editing, setEditing] = React.useState<ServerDto | null>(null);
  const [editName, setEditName] = React.useState("");
  const [editHost, setEditHost] = React.useState("");
  const [editPort, setEditPort] = React.useState("28016");
  const [editPassword, setEditPassword] = React.useState("");
  const [editContainerName, setEditContainerName] = React.useState("");
  const [editLastWipeAt, setEditLastWipeAt] = React.useState("");
  const [editLastWipeOpen, setEditLastWipeOpen] = React.useState(false);
  const [editAlwaysUseTodayWipeDate, setEditAlwaysUseTodayWipeDate] = React.useState(false);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const list = await apiJson<ServerDto[] | null>("/api/servers");
      setServers(Array.isArray(list) ? list : []);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "加载失败";
      toast.error(msg);
      if (msg.includes("未登录")) {
        clearToken();
        navigate("/login", { replace: true });
      }
    } finally {
      setLoading(false);
    }
  }, [navigate]);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  React.useEffect(() => {
    if (status !== "closed") disconnect();
    clearPendingConnection();
  }, [clearPendingConnection, disconnect, status]);

  return (
    <div className="flex h-dvh flex-col overflow-hidden">
      <AppHeader
        subtitle={<div className="text-xs text-muted-foreground">v0.1 beta</div>}
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
              aria-label="系统设置"
              onClick={() => {
                navigate("/settings");
              }}
            >
              <Settings className="h-4 w-4" />
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

      <main className="panel-scroll mx-auto flex w-full max-w-7xl flex-1 flex-col gap-4 overflow-y-auto px-4 py-4">
        <Card>
          <CardHeader className="flex flex-row items-start justify-between space-y-0">
            <div className="grid gap-1.5">
              <CardTitle>我的服务器</CardTitle>
              <CardDescription>{loading ? "加载中…" : `共 ${servers.length} 台`}</CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="secondary" onClick={() => setCreateOpen(true)}>
                新增服务器
              </Button>
              <Button variant="secondary" disabled={loading} onClick={refresh}>
                刷新
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {servers.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-10 text-center text-sm text-muted-foreground">
                <div>暂无服务器</div>
                <Button onClick={() => setCreateOpen(true)}>新增服务器</Button>
              </div>
            ) : (
              <div className="table-scroll relative w-full overflow-x-auto pb-1">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>名称</TableHead>
                      <TableHead>地址</TableHead>
                      <TableHead>端口</TableHead>
                      <TableHead>容器</TableHead>
                      <TableHead>清档时间</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {servers.map((s) => (
                        <TableRow key={s.id}>
                          <TableCell className="font-medium">{s.name}</TableCell>
                          <TableCell className="font-mono text-xs">{s.host}</TableCell>
                          <TableCell className="font-mono text-xs">{s.port ? s.port : "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{s.container_name ?? "-"}</TableCell>
                          <TableCell className="font-mono text-xs text-muted-foreground">
                            {formatDateTime(s.last_wipe_at) || "-"}
                          </TableCell>
                          <TableCell className="text-right">
                            <div className="flex justify-end gap-2">
                              <Button
                                size="sm"
                                onClick={() => {
                                  try {
                                    localStorage.setItem("rustpanel-current-server-id", String(s.id));
                                  } catch {}
                                  const addr = s.port ? `${s.host}:${s.port}` : s.host;
                                  setPendingConnection(addr, s.rcon_password);
                                  navigate(`/server/${s.id}`);
                                }}
                              >
                                管理
                              </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => {
                            setEditing(s);
                            setEditName(s.name);
                            setEditHost(s.host);
                            setEditPort(s.port ? String(s.port) : "");
                            setEditPassword(s.rcon_password);
                            setEditContainerName(s.container_name ?? "");
                            setEditLastWipeAt(s.last_wipe_at ?? "");
                            setEditAlwaysUseTodayWipeDate(Boolean(s.always_use_today_wipe_date));
                            setEditLastWipeOpen(false);
                          }}
                        >
                          编辑
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={actionBusyId === s.id || !s.container_name}
                          onClick={async () => {
                            if (!s.container_name) return;
                            if (!window.confirm(`确定对 ${s.name} 执行更新？`)) return;
                            setActionBusyId(s.id);
                            try {
                              const res = await apiJson<ServerActionResponse>(`/api/servers/${s.id}/actions/update`, {
                                method: "POST",
                              });
                              const output = [`exit_code: ${res.exit_code}`, "", res.stdout, res.stderr]
                                .filter((x) => x !== null && x !== undefined)
                                .join("\n");
                              setActionTitle(`更新：${s.name}`);
                              setActionOutput(output.trim() ? output : "无输出");
                            } catch (e) {
                              toast.error(e instanceof Error ? e.message : "执行失败");
                            } finally {
                              setActionBusyId(null);
                            }
                          }}
                        >
                          更新
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          disabled={actionBusyId === s.id}
                          onClick={async () => {
                            if (!window.confirm(`确定对 ${s.name} 执行清档？`)) return;
                            setActionBusyId(s.id);
                            try {
                              const res = await apiJson<ServerActionResponse>(`/api/servers/${s.id}/wipes`, {
                                method: "POST",
                              });
                              const output = [
                                `结果：${res.message}`,
                                res.map_filename ? `地图文件：${res.map_filename}` : "",
                                res.map_url ? `地图地址：${res.map_url}` : "",
                                res.web_rcon_error_message ? `WebRCON：${res.web_rcon_error_message}` : "",
                                "",
                                ...(res.logs ?? []),
                              ]
                                .filter(Boolean)
                                .join("\n");
                              setActionTitle(`自动清档：${s.name}`);
                              setActionOutput(output.trim() ? output : "无输出");
                              await refresh();
                            } catch (e) {
                              toast.error(e instanceof Error ? e.message : "执行失败");
                            } finally {
                              setActionBusyId(null);
                            }
                          }}
                        >
                          清档
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={async () => {
                            if (!window.confirm(`确定删除服务器：${s.name}？`)) return;
                            try {
                              await apiJson<void>(`/api/servers/${s.id}`, { method: "DELETE" });
                              await refresh();
                            } catch (e) {
                              toast.error(e instanceof Error ? e.message : "删除失败");
                            }
                          }}
                        >
                          删除
                        </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </main>

      <AppFooter />

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加服务器</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2 md:col-span-2">
              <div className="text-sm text-muted-foreground">
                地址/端口用于 WebSocket RCON（例：127.0.0.1:28016；只有域名时端口可留空）
              </div>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-name">名称</Label>
              <Input id="create-name" value={createName} onChange={(e) => setCreateName(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-host">地址</Label>
              <Input id="create-host" value={createHost} onChange={(e) => setCreateHost(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-port">端口</Label>
              <Input
                id="create-port"
                type="number"
                value={createPort}
                onChange={(e) => setCreatePort(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-password">RCON 密码</Label>
              <Input
                id="create-password"
                value={createPassword}
                onChange={(e) => setCreatePassword(e.target.value)}
              />
            </div>
            <div className="grid gap-2 md:col-span-2">
              <Label htmlFor="create-container">容器名称（可选）</Label>
              <Input
                id="create-container"
                value={createContainerName}
                onChange={(e) => setCreateContainerName(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                setCreateOpen(false);
              }}
            >
              取消
            </Button>
            <Button
              disabled={loading}
              onClick={async () => {
                const name = createName.trim();
                const host = createHost.trim();
                const pw = createPassword.trim();
                const portText = createPort.trim();
                const port = portText ? Number(portText) : 0;
                if (!name) {
                  toast.error("服务器名称不能为空");
                  return;
                }
                if (!host) {
                  toast.error("服务器地址不能为空");
                  return;
                }
                if (!pw) {
                  toast.error("RCON 密码不能为空");
                  return;
                }
                if (portText && (!Number.isFinite(port) || !Number.isInteger(port) || port < 0 || port > 65535)) {
                  toast.error("端口不合法");
                  return;
                }
                try {
                  await apiJson<ServerDto>("/api/servers", {
                    method: "POST",
                    body: JSON.stringify({
                      name,
                      host,
                      port,
                      rcon_password: pw,
                      container_name: createContainerName.trim() ? createContainerName.trim() : null,
                    }),
                  });
                  setCreateName("");
                  setCreateHost("");
                  setCreatePort("28016");
                  setCreatePassword("");
                  setCreateContainerName("");
                  setCreateOpen(false);
                  await refresh();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "添加失败");
                }
              }}
            >
              添加
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={editing !== null}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑服务器</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="edit-name">名称</Label>
              <Input id="edit-name" value={editName} onChange={(e) => setEditName(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-host">地址</Label>
              <Input id="edit-host" value={editHost} onChange={(e) => setEditHost(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-port">端口</Label>
              <Input
                id="edit-port"
                type="number"
                value={editPort}
                onChange={(e) => setEditPort(e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-password">RCON 密码</Label>
              <Input
                id="edit-password"
                value={editPassword}
                onChange={(e) => setEditPassword(e.target.value)}
              />
            </div>
            <div className="grid gap-2 md:col-span-2">
              <Label htmlFor="edit-container">容器名称（可选）</Label>
              <Input
                id="edit-container"
                value={editContainerName}
                onChange={(e) => setEditContainerName(e.target.value)}
              />
            </div>
            <label className="flex items-center gap-3 rounded-md border px-3 py-2 text-sm md:col-span-2">
              <input
                type="checkbox"
                className="h-4 w-4"
                checked={editAlwaysUseTodayWipeDate}
                onChange={(e) => setEditAlwaysUseTodayWipeDate(e.target.checked)}
              />
              <div className="grid gap-1">
                <span>始终以今天作为清档日期</span>
                <span className="text-xs text-muted-foreground">开启后，接口每次返回的清档日期都会自动变成当天。</span>
              </div>
            </label>
            <div className="flex gap-4 md:col-span-2">
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-last-wipe-at">清档日期（可选）</Label>
                <Popover open={editLastWipeOpen} onOpenChange={setEditLastWipeOpen}>
                  <PopoverTrigger asChild>
                    <Button
                      variant="outline"
                      id="edit-last-wipe-at"
                      disabled={editAlwaysUseTodayWipeDate}
                      className={cn(
                        "w-[240px] justify-start text-left font-normal",
                        !editLastWipeAt && "text-muted-foreground",
                      )}
                    >
                      <CalendarIcon className="mr-2 h-4 w-4" />
                      {parseOptionalDateTime(editLastWipeAt)
                        ? format(parseOptionalDateTime(editLastWipeAt)!, "PPP", { locale: zhCN })
                        : "选择日期"}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-0" align="start">
                    <Calendar
                      mode="single"
                      locale={zhCN}
                      selected={parseOptionalDateTime(editLastWipeAt)}
                      onSelect={(date) => {
                        if (date) {
                          const time = getWipeTime(editLastWipeAt);
                          setEditLastWipeAt(formatWipeAtValue(date, time));
                        } else {
                          setEditLastWipeAt("");
                        }
                        setEditLastWipeOpen(false);
                      }}
                      initialFocus
                    />
                  </PopoverContent>
                </Popover>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="edit-last-wipe-time">清档时间</Label>
                <Input
                  type="time"
                  step="1"
                  value={getWipeTime(editLastWipeAt)}
                  onChange={(e) => {
                    const date = parseOptionalDateTime(editLastWipeAt) || new Date();
                    setEditLastWipeAt(formatWipeAtValue(date, e.target.value));
                  }}
                  disabled={editAlwaysUseTodayWipeDate}
                  className="w-32"
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                setEditing(null);
              }}
            >
              取消
            </Button>
            <Button
              onClick={async () => {
                if (!editing) return;
                const portText = editPort.trim();
                const port = portText ? Number(portText) : 0;
                if (portText && (!Number.isFinite(port) || !Number.isInteger(port) || port < 0 || port > 65535)) {
                  toast.error("端口不合法");
                  return;
                }
                try {
                  await apiJson<ServerDto>(`/api/servers/${editing.id}`, {
                    method: "PUT",
                    body: JSON.stringify({
                      name: editName,
                      host: editHost,
                      port,
                      rcon_password: editPassword,
                      container_name: editContainerName.trim() ? editContainerName.trim() : null,
                      last_wipe_at: editLastWipeAt.trim(),
                      always_use_today_wipe_date: editAlwaysUseTodayWipeDate,
                    }),
                  });
                  setEditing(null);
                  await refresh();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "更新失败");
                }
              }}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={actionOutput !== null}
        onOpenChange={(open) => {
          if (!open) {
            setActionTitle(null);
            setActionOutput(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{actionTitle ?? "执行结果"}</DialogTitle>
          </DialogHeader>
          <div className="max-h-[60dvh] overflow-auto rounded-md border bg-muted p-3 font-mono text-xs whitespace-pre-wrap break-words">
            {actionOutput}
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                setActionTitle(null);
                setActionOutput(null);
              }}
            >
              关闭
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
