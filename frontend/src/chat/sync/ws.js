// Переподключающийся WebSocket мессенджера (/api/ws/chat). Приём апдейтов + «печатает…».
// Отправка сообщений идёт через REST (идемпотентно) — сюда шлём только typing.
export class ChatSocket {
  constructor({ getToken, onMessage, onReconnect, onStatus }) {
    this._getToken = getToken;
    this._onMessage = onMessage || (() => {});
    this._onReconnect = onReconnect || (() => {});
    this._onStatus = onStatus || (() => {});
    this.ws = null;
    this._closed = false;
    this._retry = 0;
    this._reconnectTimer = null;
    this._everConnected = false;
  }

  connect() {
    this._closed = false;
    this._open();
  }

  _url() {
    const token = this._getToken();
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    return `${proto}://${location.host}/api/ws/chat?token=${encodeURIComponent(token || '')}`;
  }

  _open() {
    if (this._closed) return;
    let ws;
    try {
      ws = new WebSocket(this._url());
    } catch {
      this._scheduleReconnect();
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      this._retry = 0;
      this._onStatus('online');
      // на каждом (пере)подключении — догон пропущенного + дослать очередь
      if (this._everConnected) this._onReconnect();
      this._everConnected = true;
    };
    ws.onmessage = (e) => {
      let data;
      try { data = JSON.parse(e.data); } catch { return; }
      this._onMessage(data);
    };
    ws.onclose = () => {
      this._onStatus('offline');
      this._scheduleReconnect();
    };
    ws.onerror = () => { try { ws.close(); } catch { /* ignore */ } };
  }

  _scheduleReconnect() {
    if (this._closed || this._reconnectTimer) return;
    this._retry = Math.min(this._retry + 1, 6);
    const delay = Math.min(1000 * 2 ** (this._retry - 1), 15000);
    this._reconnectTimer = setTimeout(() => {
      this._reconnectTimer = null;
      this._open();
    }, delay);
  }

  send(obj) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      try { this.ws.send(JSON.stringify(obj)); } catch { /* ignore */ }
    }
  }

  sendTyping(chatId) {
    this.send({ type: 'typing', chat_id: chatId });
  }

  close() {
    this._closed = true;
    if (this._reconnectTimer) { clearTimeout(this._reconnectTimer); this._reconnectTimer = null; }
    try { this.ws?.close(); } catch { /* ignore */ }
    this.ws = null;
  }
}
