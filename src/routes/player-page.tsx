import * as React from "react";
import { useParams, Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableRow } from "@/components/ui/table";
import { useRcon } from "@/lib/rcon-context";

type Player = Record<string, unknown> & { SteamID: string; DisplayName?: string };

export default function PlayerPage() {
  const { steamId } = useParams();
  const id = decodeURIComponent(String(steamId ?? ""));
  const { request } = useRcon();
  const [player, setPlayer] = React.useState<Player | null>(null);

  const refresh = React.useCallback(async () => {
    const res = await request("playerlist");
    const parsed = JSON.parse(res.Message) as Player[];
    const found = Array.isArray(parsed) ? parsed.find((p) => p?.SteamID === id) : null;
    if (!found) {
      setPlayer(null);
      return;
    }
    const cleaned: Player = { ...found };
    if ("CurrentLevel" in cleaned) delete cleaned.CurrentLevel;
    if ("UnspentXp" in cleaned) delete cleaned.UnspentXp;
    if ("VoiationLevel" in cleaned) {
      const v = cleaned.VoiationLevel;
      delete cleaned.VoiationLevel;
      (cleaned as any).ViolationLevel = v;
    }
    setPlayer(cleaned);
  }, [id, request]);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-lg font-semibold">{player?.DisplayName ?? id}</div>
          <div className="text-sm text-muted-foreground">SteamID：{id}</div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" asChild>
            <Link to="/players">返回列表</Link>
          </Button>
          <Button variant="secondary" onClick={refresh}>
            刷新
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>玩家信息</CardTitle>
        </CardHeader>
        <CardContent>
          {player ? (
            <Table>
              <TableBody>
                {Object.entries(player).map(([k, v]) => (
                  <TableRow key={k}>
                    <TableCell className="w-[200px] text-sm text-muted-foreground">{k}</TableCell>
                    <TableCell className="font-mono text-xs whitespace-pre-wrap break-words">
                      {typeof v === "string" ? v : JSON.stringify(v)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <div className="py-10 text-center text-sm text-muted-foreground">玩家不在线或不存在</div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

