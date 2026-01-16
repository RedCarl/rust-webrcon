import * as React from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useRcon } from "@/lib/rcon-context";

type ConsoleMessage = {
  Type?: string;
  Message?: string;
  Time?: string;
  Class?: string;
};

function stripHtml(text?: string) {
  return text ? String(text).replace(/<[^>]+>/gm, "") : "";
}

function getMessageTimestamp(msg: ConsoleMessage): number | null {
  const anyMsg = msg as ConsoleMessage & { DateTime?: unknown; Timestamp?: unknown };
  const raw = anyMsg.Time ?? anyMsg.DateTime ?? anyMsg.Timestamp;
  if (typeof raw === "number") return raw;
  if (typeof raw === "string") {
    const ms = Date.parse(raw);
    return Number.isNaN(ms) ? null : ms;
  }
  return null;
}

export default function ConsolePage() {
  const { request, send, client } = useRcon();
  const [output, setOutput] = React.useState<ConsoleMessage[]>([]);
  const [command, setCommand] = React.useState("");
  const scrollRef = React.useRef<HTMLDivElement | null>(null);

  const append = React.useCallback((msg: ConsoleMessage) => {
    const message = stripHtml(msg.Message);
    if (!message) return;
    if (message.startsWith("[rcon] ")) return;

    setOutput((prev) => [...prev, { ...msg, Message: message, Class: msg.Type }].slice(-2000));
  }, []);

  React.useEffect(() => {
    const handle = (packet: { Type?: string; Message?: string }) => {
      const type = packet.Type;
      if (!type) return;
      if (type === "Generic" || type === "Log" || type === "Error" || type === "Warning") {
        append({ Type: type, Message: packet.Message });
      }
    };
    client.onMessage = handle;
    return () => {
      if (client.onMessage === handle) client.onMessage = null;
    };
  }, [append, client]);

  React.useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const res = await request("console.tail 128");
        const items = JSON.parse(res.Message) as ConsoleMessage[];
        if (!alive) return;
        const firstTs = items.length > 0 ? getMessageTimestamp(items[0]!) : null;
        const lastTs = items.length > 0 ? getMessageTimestamp(items[items.length - 1]!) : null;
        const ordered = firstTs !== null && lastTs !== null ? (firstTs > lastTs ? items.slice().reverse() : items) : items.slice().reverse();
        ordered.forEach((m) => append(m));
      } catch {
        if (!alive) return;
      }
    })();
    return () => {
      alive = false;
    };
  }, [append, request]);

  React.useEffect(() => {
    if (!scrollRef.current) return;
    scrollRef.current.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [output.length]);

  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>控制台</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="rounded-md border">
            <div
              ref={scrollRef}
              className="console-scroll h-[60dvh] overflow-auto p-3 font-mono text-xs leading-5"
            >
              <div className="space-y-1">
                {output.map((m, idx) => (
                  <div key={idx} className="whitespace-pre-wrap break-words">
                    {m.Message}
                  </div>
                ))}
              </div>
            </div>
          </div>
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const c = command.trim();
              if (!c) return;
              append({ Type: "Command", Message: c });
              send(c, 1);
              setCommand("");
            }}
          >
            <Input value={command} onChange={(e) => setCommand(e.target.value)} placeholder="输入命令并回车" />
            <Button type="submit">发送</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
