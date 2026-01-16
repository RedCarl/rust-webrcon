import React from "react";
import { RconClient, type RconPacket, type RconStatus } from "@/lib/rcon-client";

type PreviousConnection = { Address: string; Password: string; date: string };
type PendingConnection = { address: string; password: string; date: string };

type RconContextValue = {
  client: RconClient;
  status: RconStatus;
  address: string | null;
  lastError: string | null;
  pendingConnection: PendingConnection | null;
  setPendingConnection: (address: string, password: string) => void;
  clearPendingConnection: () => void;
  connect: (address: string, password: string) => void;
  disconnect: () => void;
  send: (message: string, identifier?: number) => void;
  request: (message: string) => Promise<RconPacket>;
  previousConnections: PreviousConnection[];
  savePreviousConnection: (entry: PreviousConnection) => void;
  removePreviousConnection: (address: string) => void;
};

const STORAGE_KEY = "previousConnections";
const PENDING_KEY = "pendingRconConnection";

function loadPreviousConnections(): PreviousConnection[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as PreviousConnection[];
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((x) => typeof x?.Address === "string" && typeof x?.Password === "string");
  } catch {
    return [];
  }
}

function savePreviousConnections(list: PreviousConnection[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list));
}

function loadPendingConnection(): PendingConnection | null {
  try {
    const raw = localStorage.getItem(PENDING_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as PendingConnection;
    if (!parsed || typeof parsed !== "object") return null;
    if (typeof (parsed as any).address !== "string") return null;
    if (typeof (parsed as any).password !== "string") return null;
    if (typeof (parsed as any).date !== "string") return null;
    const address = String((parsed as any).address).trim();
    const password = String((parsed as any).password);
    const date = String((parsed as any).date);
    if (!address) return null;
    return { address, password, date };
  } catch {
    return null;
  }
}

function savePendingConnection(conn: PendingConnection | null) {
  if (!conn) {
    localStorage.removeItem(PENDING_KEY);
    return;
  }
  localStorage.setItem(PENDING_KEY, JSON.stringify(conn));
}

const RconContext = React.createContext<RconContextValue | null>(null);

export function RconProvider({ children }: { children: React.ReactNode }) {
  const [bootstrap] = React.useState(() => {
    const previous = loadPreviousConnections();
    return { previous };
  });

  const [client] = React.useState(() => new RconClient());
  const [status, setStatus] = React.useState<RconStatus>("closed");
  const [address, setAddress] = React.useState<string | null>(null);
  const [lastError, setLastError] = React.useState<string | null>(null);
  const [previousConnections, setPreviousConnections] = React.useState<PreviousConnection[]>(bootstrap.previous);
  const [pendingConnection, setPendingConnectionState] = React.useState<PendingConnection | null>(() =>
    loadPendingConnection(),
  );

  React.useEffect(() => {
    client.onStatusChange = (s: RconStatus) => {
      setStatus(s);
      setAddress(client.address);
      if (s === "open") setLastError(null);
      if (s === "open") {
        setPendingConnectionState((prev) => {
          if (!prev) return prev;
          savePendingConnection(null);
          return null;
        });
      }
    };
    client.onError = (err: Error) => setLastError(err.message);

    return () => {
      client.onStatusChange = null;
      client.onMessage = null;
      client.onError = null;
    };
  }, [client]);

  const savePreviousConnection = React.useCallback(
    (entry: PreviousConnection) => {
      const next = [
        ...previousConnections.filter((x) => x.Address !== entry.Address || x.Password !== entry.Password),
        entry,
      ].slice(-20);
      setPreviousConnections(next);
      savePreviousConnections(next);
    },
    [previousConnections],
  );

  const removePreviousConnection = React.useCallback(
    (addr: string) => {
      const next = previousConnections.filter((x) => x.Address !== addr);
      setPreviousConnections(next);
      savePreviousConnections(next);
    },
    [previousConnections],
  );

  const value: RconContextValue = {
    client,
    status,
    address,
    lastError,
    pendingConnection,
    setPendingConnection: (addr, pw) => {
      const next: PendingConnection = { address: addr.trim(), password: pw, date: new Date().toISOString() };
      setPendingConnectionState(next);
      savePendingConnection(next);
    },
    clearPendingConnection: () => {
      setPendingConnectionState(null);
      savePendingConnection(null);
    },
    connect: (addr, pw) => {
      setLastError(null);
      client.connect(addr, pw);
    },
    disconnect: () => {
      client.disconnect();
      setLastError(null);
    },
    send: (msg, identifier) => client.send(msg, identifier),
    request: (msg) => client.request(msg),
    previousConnections,
    savePreviousConnection,
    removePreviousConnection,
  };

  return <RconContext.Provider value={value}>{children}</RconContext.Provider>;
}

export function useRcon() {
  const ctx = React.useContext(RconContext);
  if (!ctx) throw new Error("RconProvider 缺失");
  return ctx;
}
