import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Calendar as CalendarIcon, ChevronDown, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { format } from "date-fns";
import { cn } from "@/lib/utils";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Calendar } from "@/components/ui/calendar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { apiJson, clearToken } from "@/lib/auth";
import { toast } from "sonner";
import { zhCN } from "date-fns/locale";

type AdminUserDto = {
  id: number;
  username: string;
  created_at: string;
};

type AdminServerDto = {
  id: number;
  user_ids: number[];
  name: string;
  host: string;
  port: number;
  rcon_password: string;
  container_name?: string | null;
  last_wipe_at: string;
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

function formatWipeAtValue(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}T00:00:00`;
}

export default function SettingsServersPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = React.useState(false);
  const [users, setUsers] = React.useState<AdminUserDto[]>([]);
  const [servers, setServers] = React.useState<AdminServerDto[]>([]);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [createUserIds, setCreateUserIds] = React.useState<Set<number>>(new Set());
  const [createName, setCreateName] = React.useState("");
  const [createHost, setCreateHost] = React.useState("");
  const [createPort, setCreatePort] = React.useState("28016");
  const [createPassword, setCreatePassword] = React.useState("");
  const [createContainerName, setCreateContainerName] = React.useState("");

  const [editing, setEditing] = React.useState<AdminServerDto | null>(null);
  const [editUserIds, setEditUserIds] = React.useState<Set<number>>(new Set());
  const [editName, setEditName] = React.useState("");
  const [editHost, setEditHost] = React.useState("");
  const [editPort, setEditPort] = React.useState("28016");
  const [editPassword, setEditPassword] = React.useState("");
  const [editContainerName, setEditContainerName] = React.useState("");
  const [editLastWipeAt, setEditLastWipeAt] = React.useState("");
  const [editLastWipeOpen, setEditLastWipeOpen] = React.useState(false);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const [userList, serverList] = await Promise.all([
        apiJson<AdminUserDto[] | null>("/api/admin/users"),
        apiJson<AdminServerDto[] | null>("/api/admin/servers"),
      ]);
      setUsers(Array.isArray(userList) ? userList : []);
      setServers(Array.isArray(serverList) ? serverList : []);
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
    if (!createOpen) return;
    if (createUserIds.size) return;
    const system = users.find((u) => u.username === "system");
    if (system) setCreateUserIds(new Set([system.id]));
    else if (users[0]) setCreateUserIds(new Set([users[0].id]));
  }, [createOpen, createUserIds, users]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="grid gap-1.5">
          <CardTitle>服务器管理</CardTitle>
          <CardDescription>{loading ? "加载中…" : `共 ${servers.length} 台`}</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="icon" aria-label="新增服务器" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
          </Button>
          <Button variant="secondary" size="icon" aria-label="刷新" disabled={loading} onClick={refresh}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {servers.length === 0 ? (
          <div className="py-10 text-center text-sm text-muted-foreground">暂无服务器</div>
        ) : (
          <div className="table-scroll relative w-full overflow-x-auto pb-1">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>地址</TableHead>
                  <TableHead>端口</TableHead>
                  <TableHead>账号</TableHead>
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
                    <TableCell className="text-xs">
                      {(s.user_ids && s.user_ids.length
                        ? s.user_ids.map((id) => users.find((u) => u.id === id)?.username ?? `#${id}`).join("、")
                        : "-")}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{s.container_name ?? "-"}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {formatDateTime(s.last_wipe_at) || "-"}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="secondary"
                          size="icon"
                          aria-label="编辑"
                          onClick={() => {
                            setEditing(s);
                            setEditUserIds(new Set(s.user_ids ?? []));
                            setEditName(s.name);
                            setEditHost(s.host);
                            setEditPort(String(s.port ?? 0));
                            setEditPassword(s.rcon_password);
                            setEditContainerName(s.container_name ?? "");
                            setEditLastWipeAt(s.last_wipe_at ?? "");
                            setEditLastWipeOpen(false);
                          }}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="destructive"
                          size="icon"
                          aria-label="删除"
                          onClick={async () => {
                            try {
                              await apiJson<void>(`/api/admin/servers/${s.id}`, { method: "DELETE" });
                              await refresh();
                            } catch (e) {
                              toast.error(e instanceof Error ? e.message : "删除失败");
                            }
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
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

      <Dialog
        open={createOpen}
        onOpenChange={(open) => {
          setCreateOpen(open);
        }}
      >
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>新增服务器</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2 md:col-span-2">
              <div className="text-sm text-muted-foreground">
                地址/端口用于 WebSocket RCON（例：127.0.0.1:28016；只有域名时端口可留空）
              </div>
            </div>
            <div className="grid gap-2 md:col-span-2">
              <Label htmlFor="create-user">所属账号</Label>
              <ScrollArea className="h-[220px] rounded-md border">
                <div className="divide-y">
                  {users.map((u) => {
                    const checked = createUserIds.has(u.id);
                    return (
                      <label
                        key={u.id}
                        className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-accent"
                      >
                        <input
                          type="checkbox"
                          className="h-4 w-4"
                          checked={checked}
                          onChange={(e) => {
                            const next = new Set(createUserIds);
                            if (e.target.checked) next.add(u.id);
                            else next.delete(u.id);
                            setCreateUserIds(next);
                          }}
                        />
                        <div className="min-w-0 flex-1 truncate">
                          {u.username} <span className="font-mono text-xs text-muted-foreground">#{u.id}</span>
                        </div>
                      </label>
                    );
                  })}
                  {users.length === 0 ? (
                    <div className="py-10 text-center text-sm text-muted-foreground">暂无账号</div>
                  ) : null}
                </div>
              </ScrollArea>
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
                type="password"
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
                placeholder="rust-server"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                const userIds = Array.from(createUserIds.values());
                const name = createName.trim();
                const host = createHost.trim();
                const portText = createPort.trim();
                const pw = createPassword.trim();
                const port = portText ? Number(portText) : 0;
                if (userIds.length === 0) {
                  toast.error("所属账号不能为空");
                  return;
                }
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
                  await apiJson<AdminServerDto>("/api/admin/servers", {
                    method: "POST",
                    body: JSON.stringify({
                      user_ids: userIds,
                      name,
                      host,
                      port,
                      rcon_password: pw,
                      container_name: createContainerName.trim() ? createContainerName.trim() : null,
                    }),
                  });
                  setCreateUserIds(new Set());
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
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>编辑服务器</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2 md:col-span-2">
              <Label htmlFor="edit-user">所属账号</Label>
              <ScrollArea className="h-[220px] rounded-md border">
                <div className="divide-y">
                  {users.map((u) => {
                    const checked = editUserIds.has(u.id);
                    return (
                      <label
                        key={u.id}
                        className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-accent"
                      >
                        <input
                          type="checkbox"
                          className="h-4 w-4"
                          checked={checked}
                          onChange={(e) => {
                            const next = new Set(editUserIds);
                            if (e.target.checked) next.add(u.id);
                            else next.delete(u.id);
                            setEditUserIds(next);
                          }}
                        />
                        <div className="min-w-0 flex-1 truncate">
                          {u.username} <span className="font-mono text-xs text-muted-foreground">#{u.id}</span>
                        </div>
                      </label>
                    );
                  })}
                  {users.length === 0 ? (
                    <div className="py-10 text-center text-sm text-muted-foreground">暂无账号</div>
                  ) : null}
                </div>
              </ScrollArea>
            </div>
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
              <Input id="edit-port" type="number" value={editPort} onChange={(e) => setEditPort(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-password">RCON 密码</Label>
              <Input
                id="edit-password"
                type="password"
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
                placeholder="rust-server"
              />
            </div>
            <div className="grid gap-2 md:col-span-2">
              <Label htmlFor="edit-last-wipe-at">清档时间（可选）</Label>
              <Popover open={editLastWipeOpen} onOpenChange={setEditLastWipeOpen}>
                <PopoverTrigger asChild>
                  <Button
                    variant="outline"
                    id="edit-last-wipe-at"
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
                      if (date) setEditLastWipeAt(formatWipeAtValue(date));
                      else setEditLastWipeAt("");
                      setEditLastWipeOpen(false);
                    }}
                    initialFocus
                  />
                </PopoverContent>
              </Popover>
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                if (!editing) return;
                const userIds = Array.from(editUserIds.values());
                const name = editName.trim();
                const host = editHost.trim();
                const portText = editPort.trim();
                const pw = editPassword.trim();
                const port = portText ? Number(portText) : 0;
                if (userIds.length === 0) {
                  toast.error("所属账号不能为空");
                  return;
                }
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
                  await apiJson<AdminServerDto>(`/api/admin/servers/${editing.id}`, {
                    method: "PUT",
                    body: JSON.stringify({
                      user_ids: userIds,
                      name,
                      host,
                      port,
                      rcon_password: pw,
                      container_name: editContainerName.trim() ? editContainerName.trim() : "",
                      last_wipe_at: editLastWipeAt.trim(),
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
    </Card>
  );
}
