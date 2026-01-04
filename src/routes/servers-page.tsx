import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiJson, clearToken } from "@/lib/auth";
import { useRcon } from "@/lib/rcon-context";

type ServerDto = {
  id: number;
  name: string;
  host: string;
  port: number;
  rcon_password: string;
  container_name?: string | null;
};

type ServerActionResponse = {
  stdout: string;
  stderr: string;
  exit_code: number;
};

export default function ServersPage() {
  const navigate = useNavigate();
  const { connect, status, lastError } = useRcon();
  const [servers, setServers] = React.useState<ServerDto[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [actionBusyId, setActionBusyId] = React.useState<number | null>(null);
  const [actionTitle, setActionTitle] = React.useState<string | null>(null);
  const [actionOutput, setActionOutput] = React.useState<string | null>(null);

  const [createName, setCreateName] = React.useState("");
  const [createHost, setCreateHost] = React.useState("");
  const [createPort, setCreatePort] = React.useState(28016);
  const [createPassword, setCreatePassword] = React.useState("");
  const [createContainerName, setCreateContainerName] = React.useState("");

  const [editing, setEditing] = React.useState<ServerDto | null>(null);
  const [editName, setEditName] = React.useState("");
  const [editHost, setEditHost] = React.useState("");
  const [editPort, setEditPort] = React.useState(28016);
  const [editPassword, setEditPassword] = React.useState("");
  const [editContainerName, setEditContainerName] = React.useState("");

  const refresh = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await apiJson<ServerDto[]>("/api/servers");
      setServers(list);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "加载失败";
      setError(msg);
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
    if (status === "open") navigate("/server", { replace: true });
  }, [navigate, status]);

  return (
    <div className="mx-auto grid min-h-dvh max-w-5xl gap-4 px-4 py-6">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-lg font-semibold">服务器列表</div>
          <div className="text-sm text-muted-foreground">选择服务器后会直接建立 RCON 连接</div>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            onClick={() => {
              clearToken();
              navigate("/login", { replace: true });
            }}
          >
            退出登录
          </Button>
        </div>
      </div>

      {error ? <div className="text-sm text-destructive">{error}</div> : null}
      {lastError ? <div className="text-sm text-destructive">{lastError}</div> : null}

      <Card>
        <CardHeader>
          <CardTitle>添加服务器</CardTitle>
          <CardDescription>地址/端口用于 WebSocket RCON（例：127.0.0.1:28016）</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2">
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
              onChange={(e) => setCreatePort(Number(e.target.value))}
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
            />
          </div>
        </CardContent>
        <CardFooter className="gap-2">
          <Button
            disabled={loading}
            onClick={async () => {
              setError(null);
              try {
                await apiJson<ServerDto>("/api/servers", {
                  method: "POST",
                  body: JSON.stringify({
                    name: createName,
                    host: createHost,
                    port: createPort,
                    rcon_password: createPassword,
                    container_name: createContainerName.trim() ? createContainerName.trim() : null,
                  }),
                });
                setCreateName("");
                setCreateHost("");
                setCreatePort(28016);
                setCreatePassword("");
                setCreateContainerName("");
                await refresh();
              } catch (e) {
                setError(e instanceof Error ? e.message : "添加失败");
              }
            }}
          >
            添加
          </Button>
          <Button variant="secondary" disabled={loading} onClick={refresh}>
            刷新
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>我的服务器</CardTitle>
          <CardDescription>{loading ? "加载中…" : `共 ${servers.length} 台`}</CardDescription>
        </CardHeader>
        <CardContent>
          {servers.length === 0 ? (
            <div className="py-10 text-center text-sm text-muted-foreground">暂无服务器</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>地址</TableHead>
                  <TableHead>端口</TableHead>
                  <TableHead>容器</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {servers.map((s) => (
                  <TableRow key={s.id}>
                    <TableCell className="font-medium">{s.name}</TableCell>
                    <TableCell className="font-mono text-xs">{s.host}</TableCell>
                    <TableCell className="font-mono text-xs">{s.port}</TableCell>
                    <TableCell className="font-mono text-xs">{s.container_name ?? "-"}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          disabled={status === "connecting"}
                          onClick={() => {
                            connect(`${s.host}:${s.port}`, s.rcon_password);
                          }}
                        >
                          {status === "connecting" ? "连接中…" : "连接"}
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={status === "connecting"}
                          onClick={() => {
                            setEditing(s);
                            setEditName(s.name);
                            setEditHost(s.host);
                            setEditPort(s.port);
                            setEditPassword(s.rcon_password);
                            setEditContainerName(s.container_name ?? "");
                          }}
                        >
                          编辑
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={status === "connecting" || actionBusyId === s.id || !s.container_name}
                          onClick={async () => {
                            if (!s.container_name) return;
                            if (!window.confirm(`确定对 ${s.name} 执行更新？`)) return;
                            setError(null);
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
                              setError(e instanceof Error ? e.message : "执行失败");
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
                          disabled={status === "connecting" || actionBusyId === s.id || !s.container_name}
                          onClick={async () => {
                            if (!s.container_name) return;
                            if (!window.confirm(`确定对 ${s.name} 执行清档？`)) return;
                            setError(null);
                            setActionBusyId(s.id);
                            try {
                              const res = await apiJson<ServerActionResponse>(`/api/servers/${s.id}/actions/wipe`, {
                                method: "POST",
                              });
                              const output = [`exit_code: ${res.exit_code}`, "", res.stdout, res.stderr]
                                .filter((x) => x !== null && x !== undefined)
                                .join("\n");
                              setActionTitle(`清档：${s.name}`);
                              setActionOutput(output.trim() ? output : "无输出");
                            } catch (e) {
                              setError(e instanceof Error ? e.message : "执行失败");
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
                          disabled={status === "connecting"}
                          onClick={async () => {
                            if (!window.confirm(`确定删除服务器：${s.name}？`)) return;
                            setError(null);
                            try {
                              await apiJson<void>(`/api/servers/${s.id}`, { method: "DELETE" });
                              await refresh();
                            } catch (e) {
                              setError(e instanceof Error ? e.message : "删除失败");
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
          )}
        </CardContent>
      </Card>

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
                onChange={(e) => setEditPort(Number(e.target.value))}
              />
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
              />
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
                setError(null);
                try {
                  await apiJson<ServerDto>(`/api/servers/${editing.id}`, {
                    method: "PUT",
                    body: JSON.stringify({
                      name: editName,
                      host: editHost,
                      port: editPort,
                      rcon_password: editPassword,
                      container_name: editContainerName.trim() ? editContainerName.trim() : null,
                    }),
                  });
                  setEditing(null);
                  await refresh();
                } catch (e) {
                  setError(e instanceof Error ? e.message : "更新失败");
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
