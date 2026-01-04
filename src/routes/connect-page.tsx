import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { useRcon } from "@/lib/rcon-context";

export default function ConnectPage() {
  const { connect, status, lastError, previousConnections, savePreviousConnection, removePreviousConnection } = useRcon();
  const navigate = useNavigate();
  const latest = previousConnections.length > 0 ? previousConnections[previousConnections.length - 1] : null;
  const [address, setAddress] = React.useState(() => latest?.Address ?? "");
  const [password, setPassword] = React.useState(() => latest?.Password ?? "");
  const [save, setSave] = React.useState(true);
  const isHttps = window.location.protocol === "https:";
  const isWsUrl = address.trim().toLowerCase().startsWith("ws://");
  const incompatibleWs = isHttps && isWsUrl;

  React.useEffect(() => {
    if (status === "open") navigate("/server", { replace: true });
  }, [navigate, status]);

  return (
    <div className="mx-auto flex min-h-dvh max-w-3xl items-center px-4 py-10">
      <div className="grid w-full gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>连接服务器</CardTitle>
            <CardDescription>使用 WebSocket RCON 连接到游戏服务器</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="address">地址</Label>
              <Input
                id="address"
                placeholder="127.0.0.1:28016 或 wss://example.com/ws"
                value={address}
                onChange={(e) => setAddress(e.target.value)}
                autoComplete="off"
              />
              {incompatibleWs ? (
                <div className="text-xs text-destructive">
                  当前页面是 HTTPS，浏览器会阻止 ws://（混合内容）。请使用 wss:// 或通过 Nginx 反向代理把 ws 转成 wss。
                </div>
              ) : null}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="off"
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
              记住连接信息
            </label>
            {lastError ? <div className="text-sm text-destructive">{lastError}</div> : null}
          </CardContent>
          <CardFooter className="gap-2">
            <Button
              className="w-full"
              disabled={status === "connecting" || incompatibleWs}
              onClick={() => {
                const addr = address.trim();
                const pw = password.trim();
                connect(addr, pw);
                if (save && addr) {
                  savePreviousConnection({ Address: addr, Password: pw, date: new Date().toISOString() });
                }
              }}
            >
              {status === "connecting" ? "连接中…" : "连接"}
            </Button>
          </CardFooter>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>历史连接</CardTitle>
            <CardDescription>点击即可快速连接</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3">
            {previousConnections.length === 0 ? (
              <div className="text-sm text-muted-foreground">暂无记录</div>
            ) : (
              previousConnections
                .slice()
                .reverse()
                .slice(0, 8)
                .map((c) => (
                  <div
                    key={`${c.Address}-${c.date}`}
                    className={cn(
                      "w-full rounded-md border p-3 text-left hover:bg-accent",
                      status === "connecting" ? "cursor-not-allowed opacity-50" : "cursor-pointer",
                    )}
                    role="button"
                    tabIndex={status === "connecting" ? -1 : 0}
                    aria-disabled={status === "connecting"}
                    onClick={() => {
                      if (status === "connecting") return;
                      setSave(false);
                      connect(c.Address, c.Password);
                    }}
                    onKeyDown={(e) => {
                      if (status === "connecting") return;
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        setSave(false);
                        connect(c.Address, c.Password);
                      }
                    }}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium">{c.Address}</div>
                        <div className="truncate text-xs text-muted-foreground">{new Date(c.date).toLocaleString()}</div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            removePreviousConnection(c.Address);
                          }}
                        >
                          删除
                        </Button>
                      </div>
                    </div>
                  </div>
                ))
            )}
          </CardContent>
          <CardFooter className="flex flex-col items-start gap-2 text-xs text-muted-foreground">
            <Separator />
            <div>提示：保存密码到本地存在风险，仅建议在可信环境使用。</div>
          </CardFooter>
        </Card>
      </div>
    </div>
  );
}
