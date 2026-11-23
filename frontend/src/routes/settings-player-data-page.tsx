import * as React from "react";
import { RefreshCw, Search } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { apiJson } from "@/lib/auth";
import { toast } from "sonner";

type BanRow = {
  server_name: string;
  player_name: string;
  steam_id: string;
  reason: string;
  created_at: string;
  expires_at: string;
  banned_by: string;
  unbanned_at: string;
};

type ReportRow = {
  server_name: string;
  reporter_name: string;
  reporter_steam_id: string;
  target_name: string;
  target_steam_id: string;
  subject: string;
  reason: string;
  type: string;
  created_at: string;
};

type StatsRow = {
  player_name: string;
  steam_id: string;
  server_count: number;
  wipe_count: number;
  total_online_time_seconds: number;
  last_seen_at: string;
};

type QueryRowsResponse<T> = {
  rows: T[];
};

function formatOnlineTime(seconds: number) {
  if (!Number.isFinite(seconds)) return "-";
  const hours = seconds / 3600;
  return `${hours.toFixed(1)} 小时`;
}

export default function SettingsPlayerDataPage() {
  const [banServerName, setBanServerName] = React.useState("");
  const [banRows, setBanRows] = React.useState<BanRow[]>([]);
  const [banLoading, setBanLoading] = React.useState(false);

  const [reportServerName, setReportServerName] = React.useState("");
  const [reportRows, setReportRows] = React.useState<ReportRow[]>([]);
  const [reportLoading, setReportLoading] = React.useState(false);

  const [statsServerName, setStatsServerName] = React.useState("");
  const [statsPlayerName, setStatsPlayerName] = React.useState("");
  const [statsSteamID, setStatsSteamID] = React.useState("");
  const [statsSortBy, setStatsSortBy] = React.useState("total_online_time_seconds");
  const [statsSortOrder, setStatsSortOrder] = React.useState("desc");
  const [statsRows, setStatsRows] = React.useState<StatsRow[]>([]);
  const [statsLoading, setStatsLoading] = React.useState(false);

  const loadBans = React.useCallback(async () => {
    setBanLoading(true);
    try {
      const query = new URLSearchParams();
      if (banServerName.trim()) query.set("server_name", banServerName.trim());
      const data = await apiJson<QueryRowsResponse<BanRow>>(`/api/admin/player-data/bans?${query.toString()}`);
      setBanRows(data.rows ?? []);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "读取封禁列表失败");
    } finally {
      setBanLoading(false);
    }
  }, [banServerName]);

  const loadReports = React.useCallback(async () => {
    setReportLoading(true);
    try {
      const query = new URLSearchParams();
      if (reportServerName.trim()) query.set("server_name", reportServerName.trim());
      const data = await apiJson<QueryRowsResponse<ReportRow>>(`/api/admin/player-data/reports?${query.toString()}`);
      setReportRows(data.rows ?? []);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "读取举报列表失败");
    } finally {
      setReportLoading(false);
    }
  }, [reportServerName]);

  const loadStats = React.useCallback(async () => {
    setStatsLoading(true);
    try {
      const query = new URLSearchParams();
      if (statsServerName.trim()) query.set("server_name", statsServerName.trim());
      if (statsPlayerName.trim()) query.set("player_name", statsPlayerName.trim());
      if (statsSteamID.trim()) query.set("steam_id", statsSteamID.trim());
      query.set("sort_by", statsSortBy);
      query.set("sort_order", statsSortOrder);
      const data = await apiJson<QueryRowsResponse<StatsRow>>(`/api/admin/player-data/stats?${query.toString()}`);
      setStatsRows(data.rows ?? []);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "读取玩家统计失败");
    } finally {
      setStatsLoading(false);
    }
  }, [statsPlayerName, statsServerName, statsSortBy, statsSortOrder, statsSteamID]);

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>玩家数据</CardTitle>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>数据查看</CardTitle>
          <CardDescription>封禁和举报可按 `server_name` 过滤。统计会按玩家聚合所有 `wipe_date` 数据，并支持排序。</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="bans" className="grid gap-4">
            <TabsList className="w-full justify-start">
              <TabsTrigger value="bans">玩家封禁列表</TabsTrigger>
              <TabsTrigger value="reports">玩家举报列表</TabsTrigger>
              <TabsTrigger value="stats">玩家统计信息</TabsTrigger>
            </TabsList>

            <TabsContent value="bans" className="grid gap-4">
              <div className="flex flex-col gap-3 md:flex-row">
                <Input value={banServerName} onChange={(e) => setBanServerName(e.target.value)} placeholder="按 server_name 过滤" />
                <Button disabled={banLoading} onClick={loadBans}>
                  <Search className="mr-2 h-4 w-4" />
                  查询封禁
                </Button>
              </div>
              <div className="table-scroll relative w-full overflow-x-auto pb-1">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>服务器</TableHead>
                      <TableHead>玩家</TableHead>
                      <TableHead>SteamID</TableHead>
                      <TableHead>原因</TableHead>
                      <TableHead>封禁时间</TableHead>
                      <TableHead>到期时间</TableHead>
                      <TableHead>封禁人</TableHead>
                      <TableHead>解封时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {banRows.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} className="text-center text-sm text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      banRows.map((row, index) => (
                        <TableRow key={`${row.steam_id}-${index}`}>
                          <TableCell>{row.server_name || "-"}</TableCell>
                          <TableCell>{row.player_name || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.steam_id || "-"}</TableCell>
                          <TableCell>{row.reason || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.created_at || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.expires_at || "-"}</TableCell>
                          <TableCell>{row.banned_by || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.unbanned_at || "-"}</TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </TabsContent>

            <TabsContent value="reports" className="grid gap-4">
              <div className="flex flex-col gap-3 md:flex-row">
                <Input value={reportServerName} onChange={(e) => setReportServerName(e.target.value)} placeholder="按 server_name 过滤" />
                <Button disabled={reportLoading} onClick={loadReports}>
                  <Search className="mr-2 h-4 w-4" />
                  查询举报
                </Button>
              </div>
              <div className="table-scroll relative w-full overflow-x-auto pb-1">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>服务器</TableHead>
                      <TableHead>举报人</TableHead>
                      <TableHead>举报人 SteamID</TableHead>
                      <TableHead>被举报人</TableHead>
                      <TableHead>被举报人 SteamID</TableHead>
                      <TableHead>主题</TableHead>
                      <TableHead>内容</TableHead>
                      <TableHead>类型</TableHead>
                      <TableHead>时间</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {reportRows.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={9} className="text-center text-sm text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      reportRows.map((row, index) => (
                        <TableRow key={`${row.reporter_steam_id}-${row.target_steam_id}-${index}`}>
                          <TableCell>{row.server_name || "-"}</TableCell>
                          <TableCell>{row.reporter_name || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.reporter_steam_id || "-"}</TableCell>
                          <TableCell>{row.target_name || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.target_steam_id || "-"}</TableCell>
                          <TableCell>{row.subject || "-"}</TableCell>
                          <TableCell>{row.reason || "-"}</TableCell>
                          <TableCell>{row.type || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.created_at || "-"}</TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </TabsContent>

            <TabsContent value="stats" className="grid gap-4">
              <div className="grid gap-3 md:grid-cols-5">
                <Input value={statsServerName} onChange={(e) => setStatsServerName(e.target.value)} placeholder="server_name" />
                <Input value={statsPlayerName} onChange={(e) => setStatsPlayerName(e.target.value)} placeholder="玩家名" />
                <Input value={statsSteamID} onChange={(e) => setStatsSteamID(e.target.value)} placeholder="SteamID" />
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                  value={statsSortBy}
                  onChange={(e) => setStatsSortBy(e.target.value)}
                >
                  <option value="total_online_time_seconds">总在线时长</option>
                  <option value="wipe_count">清档数</option>
                  <option value="server_count">服务器数</option>
                  <option value="player_name">玩家名</option>
                  <option value="last_seen_at">最后活跃时间</option>
                </select>
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
                  value={statsSortOrder}
                  onChange={(e) => setStatsSortOrder(e.target.value)}
                >
                  <option value="desc">降序</option>
                  <option value="asc">升序</option>
                </select>
              </div>
              <div>
                <Button disabled={statsLoading} onClick={loadStats}>
                  <Search className="mr-2 h-4 w-4" />
                  查询统计
                </Button>
              </div>
              <div className="table-scroll relative w-full overflow-x-auto pb-1">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>玩家</TableHead>
                      <TableHead>SteamID</TableHead>
                      <TableHead>累计在线时长</TableHead>
                      <TableHead>清档数</TableHead>
                      <TableHead>服务器数</TableHead>
                      <TableHead>最后活跃</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {statsRows.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} className="text-center text-sm text-muted-foreground">
                          暂无数据
                        </TableCell>
                      </TableRow>
                    ) : (
                      statsRows.map((row, index) => (
                        <TableRow key={`${row.steam_id}-${index}`}>
                          <TableCell>{row.player_name || "-"}</TableCell>
                          <TableCell className="font-mono text-xs">{row.steam_id || "-"}</TableCell>
                          <TableCell>{formatOnlineTime(row.total_online_time_seconds)}</TableCell>
                          <TableCell>{row.wipe_count}</TableCell>
                          <TableCell>{row.server_count}</TableCell>
                          <TableCell className="font-mono text-xs">{row.last_seen_at || "-"}</TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
