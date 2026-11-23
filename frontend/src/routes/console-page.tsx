import * as React from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useRcon } from "@/lib/rcon-context";

const COMMAND_HISTORY_KEY = "rustpanel-console-command-history";
const MAX_COMMAND_HISTORY = 50;

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

function loadCommandHistory(): string[] {
  try {
    const raw = localStorage.getItem(COMMAND_HISTORY_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
  } catch {
    return [];
  }
}

function saveCommandHistory(history: string[]) {
  localStorage.setItem(COMMAND_HISTORY_KEY, JSON.stringify(history));
}

export default function ConsolePage() {
  const { request, send, client } = useRcon();
  const [output, setOutput] = React.useState<ConsoleMessage[]>([]);
  const [command, setCommand] = React.useState("");
  const [history, setHistory] = React.useState<string[]>(() => loadCommandHistory());
  const [historyIndex, setHistoryIndex] = React.useState<number | null>(null);
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const draftCommandRef = React.useRef("");

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

  const handleSubmit = React.useCallback(() => {
    const nextCommand = command.trim();
    if (!nextCommand) return;

    append({ Type: "Command", Message: nextCommand });
    send(nextCommand, 1);

    const nextHistory = [...history.filter((item) => item !== nextCommand), nextCommand].slice(-MAX_COMMAND_HISTORY);
    setHistory(nextHistory);
    saveCommandHistory(nextHistory);
    setCommand("");
    setHistoryIndex(null);
    draftCommandRef.current = "";
  }, [append, command, history, send]);

  const handleHistoryKeyDown = React.useCallback(
    (event: React.KeyboardEvent<HTMLInputElement>) => {
      if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
      if (history.length === 0) return;

      event.preventDefault();

      if (event.key === "ArrowUp") {
        if (historyIndex === null) {
          draftCommandRef.current = command;
          const nextIndex = history.length - 1;
          setHistoryIndex(nextIndex);
          setCommand(history[nextIndex] ?? "");
          return;
        }

        const nextIndex = Math.max(0, historyIndex - 1);
        setHistoryIndex(nextIndex);
        setCommand(history[nextIndex] ?? "");
        return;
      }

      if (historyIndex === null) return;

      const nextIndex = historyIndex + 1;
      if (nextIndex >= history.length) {
        setHistoryIndex(null);
        setCommand(draftCommandRef.current);
        return;
      }

      setHistoryIndex(nextIndex);
      setCommand(history[nextIndex] ?? "");
    },
    [command, history, historyIndex],
  );

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
              handleSubmit();
            }}
          >
            <Input
              value={command}
              onChange={(e) => {
                const nextValue = e.target.value;
                setCommand(nextValue);
                if (historyIndex === null) {
                  draftCommandRef.current = nextValue;
                }
              }}
              onKeyDown={handleHistoryKeyDown}
              placeholder="输入命令并回车，可用上下键切换历史命令"
            />
            <Button type="submit">发送</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
