import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  useCallback,
  ReactNode,
} from "react";
import type { MultiNowEntry, ServerAlias } from "../types/multi-server";

type Ctx = {
  sessions: MultiNowEntry[];
  error: string | null;
  isConnected: boolean;
  server: ServerAlias;
  setServer: (s: ServerAlias) => void;
};

const MultiServerContext = createContext<Ctx | undefined>(undefined);

const STORAGE_KEY = "ea_server_filter";

function getInitialServer(): ServerAlias {
  if (typeof window === "undefined") return "all";
  const v = window.localStorage.getItem(STORAGE_KEY);
  if (v === "emby" || v === "plex" || v === "jellyfin" || v === "all") return v;
  return "all";
}

export function MultiServerProvider({ children }: { children: ReactNode }) {
  const [sessions, setSessions] = useState<MultiNowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [server, setServerState] = useState<ServerAlias>(getInitialServer());
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<NodeJS.Timeout | null>(null);

  const updateServer = useCallback((s: ServerAlias) => {
    setServerState(s);
    try { if (typeof window !== "undefined") window.localStorage.setItem(STORAGE_KEY, s); } catch {}
    // Reconnect WS
    if (wsRef.current) { wsRef.current.close(); }
  }, []);

  const orderRef = useRef<Map<string, number>>(new Map());
  const orderCounter = useRef(0);
  const orderSessions = useCallback((arr: MultiNowEntry[]): MultiNowEntry[] => {
    for (const s of arr) {
      if (!orderRef.current.has(s.session_id)) orderRef.current.set(s.session_id, orderCounter.current++);
    }
    const present = new Set(arr.map((s) => s.session_id));
    for (const k of Array.from(orderRef.current.keys())) { if (!present.has(k)) orderRef.current.delete(k); }
    return [...arr].sort((a,b) => (orderRef.current.get(a.session_id) ?? 0) - (orderRef.current.get(b.session_id) ?? 0));
  }, []);

  const loadSnapshot = useCallback(async (alias: ServerAlias) => {
    try {
      const res = await fetch(`/api/now/snapshot?server=${alias}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: MultiNowEntry[] = await res.json();
      setSessions(orderSessions(Array.isArray(data) ? data : []));
      setError(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(`Failed to load now playing: ${msg}`);
    }
  }, [orderSessions]);

  const connectWS = useCallback((alias: ServerAlias) => {
    if (typeof window === "undefined") return;
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const wsURL = `${proto}://${window.location.host}/api/now/ws?server=${alias}`;
    try {
      const ws = new WebSocket(wsURL);
      wsRef.current = ws;
      ws.onopen = () => {
        setIsConnected(true);
        if (reconnectRef.current) { clearTimeout(reconnectRef.current); reconnectRef.current = null; }
      };
      ws.onmessage = (ev) => {
        try {
          const data: MultiNowEntry[] = JSON.parse(ev.data);
          setSessions(orderSessions(Array.isArray(data) ? data : []));
        } catch {}
      };
      ws.onerror = () => {
        setIsConnected(false);
        void loadSnapshot(alias);
      };
      ws.onclose = () => {
        setIsConnected(false);
        if (!reconnectRef.current) {
          reconnectRef.current = setTimeout(() => { reconnectRef.current = null; connectWS(alias); }, 2000);
        }
      };
    } catch {
      setIsConnected(false);
      if (!reconnectRef.current) {
        reconnectRef.current = setTimeout(() => { reconnectRef.current = null; connectWS(alias); }, 2000);
      }
    }
  }, [loadSnapshot, orderSessions]);

  useEffect(() => {
    void loadSnapshot(server);
    connectWS(server);
    return () => {
      if (wsRef.current) wsRef.current.close();
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
    };
  }, [server, connectWS, loadSnapshot]);

  const value: Ctx = { sessions, error, isConnected, server, setServer: updateServer };
  return <MultiServerContext.Provider value={value}>{children}</MultiServerContext.Provider>;
}

export function useMultiServer() {
  const ctx = useContext(MultiServerContext);
  if (!ctx) throw new Error("useMultiServer must be used within MultiServerProvider");
  return ctx;
}
