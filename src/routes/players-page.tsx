import * as React from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useRcon } from "@/lib/rcon-context";

type Player = {
  SteamID: string;
  DisplayName?: string;
  ConnectedSeconds?: number;
  Ping?: number;
  Address?: string;
};

function secondsToDuration(input?: number) {
  if (input === undefined || input === null || Number.isNaN(input)) return "-";
  const seconds = Math.max(0, Math.floor(input));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds / 60) % 60;
  const sec = seconds % 60;
  const parts: string[] = [];
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0 || hours > 0) parts.push(`${minutes}m`);
  parts.push(`${sec}s`);
  return parts.join("");
}

export default function PlayersPage() {
  const { request, send } = useRcon();
  const [players, setPlayers] = React.useState<Player[]>([]);
  const [query, setQuery] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [banOpen, setBanOpen] = React.useState(false);
  const [banTarget, setBanTarget] = React.useState<Player | null>(null);
  const [banDuration, setBanDuration] = React.useState("1d");
  const [banReason, setBanReason] = React.useState("");
  const [banError, setBanError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await request("playerlist");
      const parsed = JSON.parse(res.Message) as Player[];
      setPlayers(Array.isArray(parsed) ? parsed : []);
    } finally {
      setLoading(false);
    }
  }, [request]);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  const openBan = React.useCallback((p: Player) => {
    setBanTarget(p);
    setBanDuration("1d");
    setBanReason("");
    setBanError(null);
    setBanOpen(true);
  }, []);

  const filtered = players
    .filter((p) => {
      const q = query.trim().toLowerCase();
      if (!q) return true;
      return String(p.DisplayName ?? "").toLowerCase().includes(q) || String(p.SteamID ?? "").includes(q);
    })
    .slice()
    .sort((a, b) => (b.ConnectedSeconds ?? 0) - (a.ConnectedSeconds ?? 0));

  return (
    <div className="grid gap-4">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0">
          <div className="text-lg font-semibold">玩家列表</div>
          <div className="text-sm text-muted-foreground">支持搜索、踢出与封禁</div>
        </div>
        <div className="flex items-center gap-2">
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索昵称或 SteamID" />
          <Button variant="secondary" onClick={refresh} disabled={loading}>
            刷新
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>在线玩家（{filtered.length}）</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>昵称</TableHead>
                <TableHead>SteamID</TableHead>
                <TableHead>在线时长</TableHead>
                <TableHead>Ping</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((p) => (
                <TableRow key={p.SteamID}>
                  <TableCell className="font-medium">
                    <Link className="hover:underline" to={`/players/${encodeURIComponent(p.SteamID)}`}>
                      {p.DisplayName ?? "-"}
                    </Link>
                  </TableCell>
                  <TableCell className="font-mono text-xs">{p.SteamID}</TableCell>
                  <TableCell>{secondsToDuration(p.ConnectedSeconds)}</TableCell>
                  <TableCell>{p.Ping ?? "-"}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-2">
                      <Button variant="secondary" size="sm" onClick={() => openBan(p)}>
                        封禁
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => {
                          send(`kick ${p.SteamID}`);
                          setTimeout(() => refresh(), 300);
                        }}
                      >
                        踢出
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-sm text-muted-foreground">
                    暂无数据
                  </TableCell>
                </TableRow>
              ) : null}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog
        open={banOpen}
        onOpenChange={(open: boolean) => {
          setBanOpen(open);
          if (!open) {
            setBanTarget(null);
            setBanError(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>封禁玩家</DialogTitle>
            <DialogDescription>
              {banTarget ? `${banTarget.DisplayName ?? "-"}（${banTarget.SteamID}）` : "选择一个玩家"}
            </DialogDescription>
          </DialogHeader>

          <form
            className="grid gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (!banTarget) return;

              const duration = banDuration.trim();
              const reason = banReason.trim();

              if (!reason) {
                setBanError("请输入封禁理由");
                return;
              }

              setBanError(null);
              if (duration) {
                send(`ban ${banTarget.SteamID} ${duration} ${reason}`);
              } else {
                send(`ban ${banTarget.SteamID} ${reason}`);
              }
              setBanOpen(false);
              setTimeout(() => refresh(), 300);
            }}
          >
            <div className="grid gap-2">
              <Label htmlFor="banDuration">封禁时间</Label>
              <select
                id="banDuration"
                value={banDuration}
                onChange={(e) => setBanDuration(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
              >
                <option value="1d">1 天</option>
                <option value="3d">3 天</option>
                <option value="7d">7 天</option>
                <option value="14d">14 天</option>
                <option value="30d">30 天</option>
                <option value="">永久</option>
              </select>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="banReason">封禁理由</Label>
              <Textarea
                id="banReason"
                value={banReason}
                onChange={(e) => setBanReason(e.target.value)}
                placeholder="请输入封禁理由"
              />
            </div>

            {banError ? <div className="text-sm text-destructive">{banError}</div> : null}

            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setBanOpen(false)}>
                取消
              </Button>
              <Button type="submit" variant="destructive" disabled={!banTarget}>
                确认封禁
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
