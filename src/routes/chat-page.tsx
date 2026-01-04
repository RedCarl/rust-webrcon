import * as React from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useRcon } from "@/lib/rcon-context";

type ChatMessage = { Username?: string; Message?: string; Time?: string };

function stripHtml(text?: string) {
  return text ? String(text).replace(/<[^>]+>/gm, "") : "";
}

function toTimeNumber(input?: string) {
  if (!input) return 0;
  const t = Date.parse(input);
  return Number.isFinite(t) ? t : 0;
}

export default function ChatPage() {
  const { request, send, client } = useRcon();
  const [output, setOutput] = React.useState<ChatMessage[]>([]);
  const [text, setText] = React.useState("");
  const scrollRef = React.useRef<HTMLDivElement | null>(null);

  const append = React.useCallback((m: ChatMessage) => {
    setOutput((prev) => [...prev, { ...m, Username: stripHtml(m.Username), Message: stripHtml(m.Message) }].slice(-2000));
  }, []);

  React.useEffect(() => {
    const handle = (packet: { Type?: string; Message?: string }) => {
      if (packet.Type !== "Chat") return;
      try {
        const parsed = JSON.parse(packet.Message ?? "{}") as ChatMessage;
        append(parsed);
      } catch {
        return;
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
        const res = await request("chat.tail 512");
        let items = JSON.parse(res.Message) as ChatMessage[];
        if (!Array.isArray(items)) items = [];
        if (items.length > 1) {
          const first = toTimeNumber(items[0]?.Time);
          const last = toTimeNumber(items[items.length - 1]?.Time);
          if (first && last && first > last) items = items.reverse();
        }
        if (!alive) return;
        items.forEach((m) => append(m));
        setTimeout(() => {
          scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
        }, 50);
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
          <CardTitle>聊天</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <div className="rounded-md border">
            <div ref={scrollRef} className="h-[60dvh] overflow-auto space-y-2 p-3 text-sm">
              {output.map((m, idx) => (
                <div key={idx} className="grid grid-cols-[140px_1fr] gap-3">
                  <div className="truncate text-right text-xs text-muted-foreground">{m.Username ?? "-"}</div>
                  <div className="min-w-0 whitespace-pre-wrap break-words">{m.Message ?? ""}</div>
                </div>
              ))}
            </div>
          </div>

          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const v = text.trim();
              if (!v) return;
              send(`say ${v}`, 1);
              setText("");
            }}
          >
            <Input value={text} onChange={(e) => setText(e.target.value)} placeholder="输入聊天内容并回车" />
            <Button type="submit">发送</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
