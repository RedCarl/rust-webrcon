export type RconStatus = "closed" | "connecting" | "open";

export type RconPacket = {
  Identifier: number;
  Message: string;
  Name?: string;
  Type?: string;
};

type Pending = {
  resolve: (packet: RconPacket) => void;
  reject: (error: Error) => void;
};

export class RconClient {
  private socket: WebSocket | null = null;
  private callbacks = new Map<number, Pending>();
  private lastIndex = 1001;

  address: string | null = null;
  status: RconStatus = "closed";
  onStatusChange: ((status: RconStatus) => void) | null = null;
  onMessage: ((packet: RconPacket) => void) | null = null;
  onError: ((error: Error) => void) | null = null;

  connect(address: string, password: string) {
    this.disconnect();

    const addr = address.trim();
    const pw = password.trim();
    const isExplicitWsUrl = /^wss?:\/\//i.test(addr);
    const protocol = window.location.protocol === "https:" ? "wss" : "ws";

    const normalizedAddr = isExplicitWsUrl ? addr : addr.replace(/\/+$/, "");

    this.address = normalizedAddr;
    this.status = "connecting";
    this.onStatusChange?.(this.status);

    const socketUrl = isExplicitWsUrl
      ? pw
        ? `${addr.replace(/\/+$/, "")}/${encodeURIComponent(pw)}`
        : addr
      : `${protocol}://${normalizedAddr}/${encodeURIComponent(pw)}`;

    const socket = new WebSocket(socketUrl);
    this.socket = socket;

    socket.onopen = () => {
      this.status = "open";
      this.onStatusChange?.(this.status);
    };

    socket.onclose = () => {
      this.status = "closed";
      this.onStatusChange?.(this.status);
      const err = new Error("连接已断开");
      for (const pending of this.callbacks.values()) pending.reject(err);
      this.callbacks.clear();
    };

    socket.onerror = () => {
      const err = new Error("连接发生错误");
      this.onError?.(err);
    };

    socket.onmessage = (event) => {
      let data: RconPacket;
      try {
        data = JSON.parse(String(event.data)) as RconPacket;
      } catch {
        return;
      }

      if (data.Identifier > 1000) {
        const pending = this.callbacks.get(data.Identifier);
        if (pending) {
          pending.resolve(data);
          this.callbacks.delete(data.Identifier);
        }
        return;
      }

      this.onMessage?.(data);
    };
  }

  disconnect() {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
    this.callbacks.clear();
    this.status = "closed";
    this.onStatusChange?.(this.status);
    this.address = null;
  }

  isConnected() {
    return this.socket !== null && this.socket.readyState === WebSocket.OPEN;
  }

  send(message: string, identifier = -1) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return;
    const packet: RconPacket = { Identifier: identifier, Message: message, Name: "WebRcon" };
    this.socket.send(JSON.stringify(packet));
  }

  request(message: string) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error("未连接"));
    }

    this.lastIndex += 1;
    const id = this.lastIndex;

    return new Promise<RconPacket>((resolve, reject) => {
      this.callbacks.set(id, { resolve, reject });
      this.send(message, id);
    });
  }
}
