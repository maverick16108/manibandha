// Переподключающийся WebSocket мессенджера — платформо-независимо через глобальный WebSocket
// (есть и в браузере, и в React Native). Приём апдейтов + «печатает…».
export interface ChatSocket {
  connect(): void;
  close(): void;
  sendTyping(chatId: number): void;
}

export interface ChatSocketOptions {
  /** Функция, строящая ws-URL по токену, напр. (t) => `wss://host/api/ws/chat?token=${t}`. */
  url: (token: string) => string;
  getToken(): string | null | undefined;
  onMessage(evt: unknown): void;
  onReconnect(): void;
  onStatus(s: 'online' | 'offline'): void;
}

export function createReconnectingSocket(opts: ChatSocketOptions): ChatSocket {
  let ws: WebSocket | null = null;
  let closed = false;
  let retry = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let everConnected = false;

  function open(): void {
    if (closed) return;
    const token = opts.getToken() || '';
    let socket: WebSocket;
    try {
      socket = new WebSocket(opts.url(token));
    } catch {
      scheduleReconnect();
      return;
    }
    ws = socket;
    socket.onopen = () => {
      retry = 0;
      opts.onStatus('online');
      if (everConnected) opts.onReconnect(); // догон + дослать очередь на реконнекте
      everConnected = true;
    };
    socket.onmessage = (e: MessageEvent) => {
      let data: unknown;
      try {
        data = JSON.parse(typeof e.data === 'string' ? e.data : '');
      } catch {
        return;
      }
      opts.onMessage(data);
    };
    socket.onclose = () => {
      opts.onStatus('offline');
      scheduleReconnect();
    };
    socket.onerror = () => {
      try {
        socket.close();
      } catch {
        /* ignore */
      }
    };
  }

  function scheduleReconnect(): void {
    if (closed || reconnectTimer) return;
    retry = Math.min(retry + 1, 6);
    const delay = Math.min(1000 * 2 ** (retry - 1), 15000);
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      open();
    }, delay);
  }

  function sendRaw(obj: unknown): void {
    if (ws && ws.readyState === WebSocket.OPEN) {
      try {
        ws.send(JSON.stringify(obj));
      } catch {
        /* ignore */
      }
    }
  }

  return {
    connect() {
      closed = false;
      open();
    },
    close() {
      closed = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      try {
        ws?.close();
      } catch {
        /* ignore */
      }
      ws = null;
    },
    sendTyping(chatId: number) {
      sendRaw({ type: 'typing', chat_id: chatId });
    },
  };
}
