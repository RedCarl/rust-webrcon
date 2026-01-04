import React from "react";
import { RconClient, type RconPacket, type RconStatus } from "@/lib/rcon-client";

type PreviousConnection = { Address: string; Password: string; date: string };

type RconContextValue = {
  client: RconClient;
  status: RconStatus;
  address: string | null;
  lastError: string | null;
  autoReconnectTried: boolean;
  autoReconnectInProgress: boolean;
  connect: (address: string, password: string) => void;
  disconnect: () => void;
  send: (message: string, identifier?: number) => void;
  request: (message: string) => Promise<RconPacket>;
  previousConnections: PreviousConnection[];
  savePreviousConnection: (entry: PreviousConnection) => void;
  removePreviousConnection: (address: string) => void;
};

const STORAGE_KEY = "previousConnections";

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

const RconContext = React.createContext<RconContextValue | null>(null);

export function RconProvider({ children }: { children: React.ReactNode }) {
  const [bootstrap] = React.useState(() => {
    const previous = loadPreviousConnections();
    const candidate = previous.length > 0 ? previous[previous.length - 1]! : null;
    return { previous, candidate };
  });

  const [client] = React.useState(() => new RconClient());
  const [status, setStatus] = React.useState<RconStatus>(bootstrap.candidate ? "connecting" : "closed");
  const [address, setAddress] = React.useState<string | null>(bootstrap.candidate?.Address ?? null);
  const [lastError, setLastError] = React.useState<string | null>(null);
  const [previousConnections, setPreviousConnections] = React.useState<PreviousConnection[]>(bootstrap.previous);
  const [autoReconnectTried] = React.useState<boolean>(bootstrap.candidate !== null);
  const [autoReconnectInProgress, setAutoReconnectInProgress] = React.useState<boolean>(bootstrap.candidate !== null);

  React.useEffect(() => {
    client.onStatusChange = (s: RconStatus) => {
      setStatus(s);
      setAddress(client.address);
      if (s === "open") setLastError(null);
      if (bootstrap.candidate) {
        if (s === "open" || s === "closed") setAutoReconnectInProgress(false);
      }
    };
    client.onError = (err: Error) => setLastError(err.message);

    if (bootstrap.candidate) {
      client.connect(bootstrap.candidate.Address, bootstrap.candidate.Password);
    }

    return () => {
      client.onStatusChange = null;
      client.onMessage = null;
      client.onError = null;
    };
  }, [bootstrap.candidate, client]);

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
    autoReconnectTried,
    autoReconnectInProgress,
    connect: (addr, pw) => {
      setLastError(null);
      setAutoReconnectInProgress(false);
      client.connect(addr, pw);
    },
    disconnect: () => {
      client.disconnect();
      setLastError(null);
      setAutoReconnectInProgress(false);
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
