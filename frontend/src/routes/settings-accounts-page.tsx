import * as React from "react";
import { useNavigate } from "react-router-dom";
import { KeyRound, Link2, Plus, RefreshCw } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { apiJson, clearToken } from "@/lib/auth";
import { toast } from "sonner";

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
  container_name?: string | null;
};

function formatDateTime(s: string) {
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return s;
  try {
    return d.toLocaleString("zh-CN", { hour12: false });
  } catch {
    return d.toLocaleString();
  }
}

export default function SettingsAccountsPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = React.useState(false);
  const [users, setUsers] = React.useState<AdminUserDto[]>([]);
  const [servers, setServers] = React.useState<AdminServerDto[]>([]);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [createUsername, setCreateUsername] = React.useState("");
  const [createPassword, setCreatePassword] = React.useState("");

  const [passwordUser, setPasswordUser] = React.useState<AdminUserDto | null>(null);
  const [newPassword, setNewPassword] = React.useState("");

  const [assignUser, setAssignUser] = React.useState<AdminUserDto | null>(null);
  const [assigned, setAssigned] = React.useState<Set<number>>(new Set());

  const serverCountByUserId = React.useMemo(() => {
    const m = new Map<number, number>();
    for (const s of servers) {
      for (const uid of s.user_ids ?? []) {
        m.set(uid, (m.get(uid) ?? 0) + 1);
      }
    }
    return m;
  }, [servers]);

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

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="grid gap-1.5">
          <CardTitle>账号管理</CardTitle>
          <CardDescription>{loading ? "加载中…" : `共 ${users.length} 个账号`}</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="icon" aria-label="添加账号" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" />
          </Button>
          <Button variant="secondary" size="icon" aria-label="刷新" disabled={loading} onClick={refresh}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {users.length === 0 ? (
          <div className="py-10 text-center text-sm text-muted-foreground">暂无账号</div>
        ) : (
          <div className="relative w-full overflow-x-auto pb-1">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户名</TableHead>
                  <TableHead>ID</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead>服务器数</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">{u.username}</TableCell>
                    <TableCell className="font-mono text-xs">{u.id}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{formatDateTime(u.created_at)}</TableCell>
                    <TableCell className="font-mono text-xs">{serverCountByUserId.get(u.id) ?? 0}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="secondary"
                          size="icon"
                          aria-label="改密码"
                          onClick={() => {
                            setPasswordUser(u);
                            setNewPassword("");
                          }}
                        >
                          <KeyRound className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="secondary"
                          size="icon"
                          aria-label="分配服务器"
                          onClick={() => {
                            setAssignUser(u);
                            setAssigned(
                              new Set(servers.filter((s) => (s.user_ids ?? []).includes(u.id)).map((s) => s.id)),
                            );
                          }}
                        >
                          <Link2 className="h-4 w-4" />
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加账号</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="create-username">用户名</Label>
              <Input id="create-username" value={createUsername} onChange={(e) => setCreateUsername(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="create-password">密码</Label>
              <Input
                id="create-password"
                type="password"
                value={createPassword}
                onChange={(e) => setCreatePassword(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                const username = createUsername.trim();
                const password = createPassword.trim();
                if (!username) {
                  toast.error("用户名不能为空");
                  return;
                }
                if (password.length < 6) {
                  toast.error("密码至少 6 位");
                  return;
                }
                try {
                  await apiJson<AdminUserDto>("/api/admin/users", {
                    method: "POST",
                    body: JSON.stringify({ username, password }),
                  });
                  setCreateUsername("");
                  setCreatePassword("");
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
        open={passwordUser !== null}
        onOpenChange={(open) => {
          if (!open) setPasswordUser(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>修改密码</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="text-sm text-muted-foreground">账号：{passwordUser?.username ?? "-"}</div>
            <div className="grid gap-2">
              <Label htmlFor="new-password">新密码</Label>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                if (!passwordUser) return;
                const password = newPassword.trim();
                if (password.length < 6) {
                  toast.error("密码至少 6 位");
                  return;
                }
                try {
                  await apiJson<void>(`/api/admin/users/${passwordUser.id}/password`, {
                    method: "PUT",
                    body: JSON.stringify({ password }),
                  });
                  setPasswordUser(null);
                  setNewPassword("");
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "修改失败");
                }
              }}
            >
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={assignUser !== null}
        onOpenChange={(open) => {
          if (!open) setAssignUser(null);
        }}
      >
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>分配服务器</DialogTitle>
          </DialogHeader>
          <div className="grid gap-4">
            <div className="text-sm text-muted-foreground">账号：{assignUser?.username ?? "-"}</div>
            <ScrollArea className="h-[360px] rounded-md border">
              <div className="divide-y">
                {servers.map((s) => {
                  const checked = assigned.has(s.id);
                  return (
                    <label
                      key={s.id}
                      className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-accent"
                    >
                      <input
                        type="checkbox"
                        className="h-4 w-4"
                        checked={checked}
                        onChange={(e) => {
                          const next = new Set(assigned);
                          if (e.target.checked) next.add(s.id);
                          else next.delete(s.id);
                          setAssigned(next);
                        }}
                      />
                      <div className="min-w-0 flex-1">
                        <div className="truncate font-medium">
                          {s.name} <span className="font-mono text-xs text-muted-foreground">#{s.id}</span>
                        </div>
                        <div className="truncate font-mono text-xs text-muted-foreground">
                          {s.host}
                          {s.port ? `:${s.port}` : ""}
                        </div>
                      </div>
                      <div className="shrink-0 font-mono text-xs text-muted-foreground">
                        <span>
                          当前：
                          {(s.user_ids && s.user_ids.length
                            ? s.user_ids
                                .map((id) => users.find((u) => u.id === id)?.username ?? `#${id}`)
                                .join("、")
                            : "无账号")}
                        </span>
                      </div>
                    </label>
                  );
                })}
                {servers.length === 0 ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">暂无服务器</div>
                ) : null}
              </div>
            </ScrollArea>
          </div>
          <DialogFooter>
            <Button
              onClick={async () => {
                if (!assignUser) return;
                try {
                  await apiJson<void>(`/api/admin/users/${assignUser.id}/servers`, {
                    method: "PUT",
                    body: JSON.stringify({ server_ids: Array.from(assigned.values()) }),
                  });
                  setAssignUser(null);
                  await refresh();
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "分配失败");
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
