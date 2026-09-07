import { useEffect, useState } from 'react';

type UpdateMessage = { type: 'update'; topic: string; data: unknown };
type Listener = (data: unknown) => void;

// wsURL builds the connection URL even with no token — a public page (e.g.
// /markets) has no login at all, and its topics (market-stocks,
// protocol-stats, reserves, agent-task-reasoning) are still allowed for an
// anonymous connection, matching their already-unauthenticated REST
// equivalents (backend authorization_service.go). Only topics that actually
// need auth get silently dropped server-side for an anonymous connection.
function wsURL(): string | null {
  if (typeof window === 'undefined') return null;
  const backend = process.env.NEXT_PUBLIC_BACKEND_URL;
  if (!backend) return null;
  const scheme = backend.startsWith('https') ? 'wss' : 'ws';
  const host = backend.replace(/^https?:\/\//, '');
  const token = localStorage.getItem('access_token');
  const query = token ? `?token=${encodeURIComponent(token)}` : '';
  return `${scheme}://${host}/api/v1/realtime/ws${query}`;
}

// RealtimeSocket is a single WebSocket connection shared by the whole app,
// multiplexing every topic a hook subscribes to (docs/plans/realtime-websocket-updates.md
// §2/§4) — one handshake regardless of how many hooks/components want
// updates, mirroring how a managed pub/sub service (e.g. Ably) hands a
// client one connection and many channels on top of it, not one connection
// per channel.
class RealtimeSocket {
  private ws: WebSocket | null = null;
  private listeners = new Map<string, Set<Listener>>();
  private reconnectDelayMs = 1_000;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private connected = false;
  private connectionListeners = new Set<(connected: boolean) => void>();

  connect() {
    if (this.ws) return;
    const url = wsURL();
    if (!url) return;

    const socket = new WebSocket(url);
    this.ws = socket;

    socket.onopen = () => {
      this.reconnectDelayMs = 1_000;
      this.connected = true;
      this.connectionListeners.forEach((cb) => cb(true));
      const topics = Array.from(this.listeners.keys());
      if (topics.length > 0) socket.send(JSON.stringify({ type: 'subscribe', topics }));
    };

    socket.onmessage = (event) => {
      let msg: UpdateMessage;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }
      if (msg.type === 'update') {
        this.listeners.get(msg.topic)?.forEach((cb) => cb(msg.data));
      }
    };

    socket.onclose = () => {
      // Guards against a superseded socket's close event landing after
      // reconnect() has already installed a newer one (see reconnect()
      // below) — without this check, that stale event would null out the
      // new socket's own reference.
      if (this.ws !== socket) return;
      this.ws = null;
      this.connected = false;
      this.connectionListeners.forEach((cb) => cb(false));
      // Reconnect with backoff (1s, 2s, 4s, ... capped at 30s) rather than
      // hammering immediately or giving up — see plan doc §5.
      this.reconnectTimer = setTimeout(() => this.connect(), this.reconnectDelayMs);
      this.reconnectDelayMs = Math.min(this.reconnectDelayMs * 2, 30_000);
    };

    socket.onerror = () => socket.close();
  }

  // reconnect drops the current connection and immediately opens a fresh
  // one under the (possibly now different) token in localStorage —
  // SiweAuthContext calls this on both sign-in and sign-out. Without it, a
  // socket that connected anonymously before login never picks up the new
  // identity (private topics stay silently dropped by the backend's
  // isTopicAllowed even after the user logs in), and a socket that was
  // authenticated before logout keeps flowing already-granted private
  // topics until it happens to drop on its own — a real stale-auth gap, not
  // just a UX one. Existing topic subscriptions (this.listeners) are
  // untouched, so they get re-requested automatically once the new
  // connection's onopen fires.
  reconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.reconnectDelayMs = 1_000;
    this.connect();
  }

  isConnected() {
    return this.connected;
  }

  onConnectionChange(cb: (connected: boolean) => void) {
    this.connectionListeners.add(cb);
    return () => {
      this.connectionListeners.delete(cb);
    };
  }

  subscribe(topic: string, listener: Listener) {
    let set = this.listeners.get(topic);
    const isNewTopic = !set;
    if (!set) {
      set = new Set();
      this.listeners.set(topic, set);
    }
    set.add(listener);
    if (isNewTopic && this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: 'subscribe', topics: [topic] }));
    }
    return () => {
      set!.delete(listener);
      if (set!.size === 0) {
        this.listeners.delete(topic);
        if (this.ws?.readyState === WebSocket.OPEN) {
          this.ws.send(JSON.stringify({ type: 'unsubscribe', topics: [topic] }));
        }
      }
    };
  }
}

export const realtimeSocket = new RealtimeSocket();

// useRealtimeTopic subscribes onUpdate to topic for as long as the
// component is mounted, connecting the shared socket lazily on first use.
// Pass undefined to skip subscribing (mirrors useQuery's own `enabled`
// pattern for a not-yet-known id).
export function useRealtimeTopic<T = unknown>(topic: string | undefined, onUpdate: (data: T) => void) {
  useEffect(() => {
    if (!topic) return;
    realtimeSocket.connect();
    return realtimeSocket.subscribe(topic, (data) => onUpdate(data as T));
    // onUpdate is intentionally excluded — callers pass an inline closure
    // each render; re-subscribing on every render would thrash the socket's
    // subscribe/unsubscribe messages for no benefit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [topic]);
}

// useRealtimeConnected drives each hook's fallback-poll switch (plan doc
// §4/§5): poll normally while disconnected, stop polling once the socket is
// up and pushing updates instead.
export function useRealtimeConnected(): boolean {
  const [connected, setConnected] = useState(() => realtimeSocket.isConnected());
  useEffect(() => {
    return realtimeSocket.onConnectionChange(setConnected);
  }, []);
  return connected;
}
