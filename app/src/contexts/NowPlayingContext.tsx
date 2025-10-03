// app/src/contexts/NowPlayingContext.tsx
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  ReactNode,
  useCallback,
} from "react";

export type NowEntry = {
  timestamp: number;
  title: string;
  user: string;
  app: string;
  device: string;
  play_method: string;
  video: string;
  audio: string;
  subs: string;
  bitrate: number;
  progress_pct: number;
  position_sec?: number;
  duration_sec?: number;
  is_paused?: boolean;
  poster: string;
  session_id: string;
  item_id: string;
  item_type?: string;
  container?: string;
  width?: number;
  height?: number;
  dolby_vision?: boolean;
  hdr10?: boolean;
  audio_lang?: string;
  audio_ch?: number;
  sub_lang?: string;
  sub_codec?: string;
  trans_video_from?: string;
  trans_video_to?: string;
  trans_audio_from?: string;
  trans_audio_to?: string;
  video_method?: string;
  audio_method?: string;
  stream_path?: string;
  stream_detail?: string;
  trans_reason?: string;
  trans_pct?: number;
  trans_audio_bitrate?: number;
  trans_video_bitrate?: number;
  // Multi-server extras
  server_id?: string;
  server_type?: string;
  series_id?: string;
};

interface NowPlayingContextType {
  sessions: NowEntry[];
  error: string | null;
  isConnected: boolean;
}

const NowPlayingContext = createContext<NowPlayingContextType | undefined>(undefined);

const apiBase =
  (typeof window !== "undefined" &&
    (window as unknown as { NEXT_PUBLIC_API_BASE?: string }).NEXT_PUBLIC_API_BASE) ||
  process.env.NEXT_PUBLIC_API_BASE ||
  "";

interface NowPlayingProviderProps {
  children: ReactNode;
}

export function NowPlayingProvider({ children }: NowPlayingProviderProps) {
  const [sessions, setSessions] = useState<NowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const sessionsLenRef = useRef(0);
  const snapshotRetryRef = useRef<NodeJS.Timeout | null>(null);
  const loadSnapshotRef = useRef<(() => Promise<void>) | null>(null);
  // Stable ordering: remember first-seen order for each session id
  const firstSeenRef = useRef<Map<string, number>>(new Map());
  const orderCounterRef = useRef(0);

  // Keep a ref of the latest sessions length for use in stable callbacks
  useEffect(() => {
    sessionsLenRef.current = sessions.length;
  }, [sessions.length]);

  // Reusable helper to maintain stable first-seen ordering of sessions
  const orderSessions = useCallback((arr: NowEntry[]): NowEntry[] => {
    // assign first-seen order to new sessions
    for (const s of arr) {
      if (!firstSeenRef.current.has(s.session_id)) {
        firstSeenRef.current.set(s.session_id, orderCounterRef.current++);
      }
    }
    // remove entries no longer present
    const present = new Set(arr.map((s) => s.session_id));
    for (const key of Array.from(firstSeenRef.current.keys())) {
      if (!present.has(key)) firstSeenRef.current.delete(key);
    }
    // return a new array sorted by first-seen
    return [...arr].sort(
      (a, b) =>
        (firstSeenRef.current.get(a.session_id) ?? 0) -
        (firstSeenRef.current.get(b.session_id) ?? 0)
    );
  }, []);

  const loadSnapshot = useCallback(async () => {
    try {
      const res = await fetch(`${apiBase}/api/now/snapshot?server=all`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: NowEntry[] = await res.json();
      const arr = Array.isArray(data) ? data : [];
      setSessions(orderSessions(arr));
      setError(null);
      if (snapshotRetryRef.current) {
        clearTimeout(snapshotRetryRef.current);
        snapshotRetryRef.current = null;
      }
    } catch (e: unknown) {
      const msg = (e as Error)?.message || String(e);
      setError(`Failed to load now playing: ${msg}`);
      if (!snapshotRetryRef.current) {
        snapshotRetryRef.current = setTimeout(() => {
          snapshotRetryRef.current = null;
          void loadSnapshotRef.current?.();
        }, 3000);
      }
    }
  }, [orderSessions]);

  useEffect(() => {
    loadSnapshotRef.current = loadSnapshot;
  }, [loadSnapshot]);

  const connectWS = useCallback(() => {
    if (typeof window === "undefined") return;

    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const wsURL = `${proto}://${window.location.host}/api/now/ws?server=all`;

    try {
      const ws = new WebSocket(wsURL);
      wsRef.current = ws;

      ws.onopen = () => {
        setIsConnected(true);
        setError(null);
        // Clear any existing reconnect timeout
        if (reconnectTimeoutRef.current) {
          clearTimeout(reconnectTimeoutRef.current);
          reconnectTimeoutRef.current = null;
        }
        if (snapshotRetryRef.current) {
          clearTimeout(snapshotRetryRef.current);
          snapshotRetryRef.current = null;
        }
      };

      ws.onmessage = (ev) => {
        try {
          const data: NowEntry[] = JSON.parse(ev.data);
          const arr = Array.isArray(data) ? data : [];
          setSessions(orderSessions(arr));
        } catch {
          /* ignore parse errors */
        }
      };

      ws.onerror = () => {
        setIsConnected(false);
        // Load snapshot as fallback if we have no sessions
        if (sessionsLenRef.current === 0) {
          void loadSnapshot();
        }
      };

      ws.onclose = () => {
        setIsConnected(false);
        // Reconnect after 2 seconds
        if (!reconnectTimeoutRef.current) {
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectTimeoutRef.current = null;
            connectWS();
          }, 2000);
        }
      };
    } catch {
      setIsConnected(false);
      // Reconnect after 2 seconds on error
      if (!reconnectTimeoutRef.current) {
        reconnectTimeoutRef.current = setTimeout(() => {
          reconnectTimeoutRef.current = null;
          connectWS();
        }, 2000);
      }
    }
  }, [loadSnapshot, orderSessions]);

  useEffect(() => {
    // Load initial snapshot
    loadSnapshot();
    // Connect WebSocket
    connectWS();

    return () => {
      // Cleanup on unmount
      if (wsRef.current) {
        wsRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (snapshotRetryRef.current) {
        clearTimeout(snapshotRetryRef.current);
        snapshotRetryRef.current = null;
      }
    };
  }, [connectWS, loadSnapshot]);

  const value: NowPlayingContextType = {
    sessions,
    error,
    isConnected,
  };

  return <NowPlayingContext.Provider value={value}>{children}</NowPlayingContext.Provider>;
}

export function useNowPlaying(): NowPlayingContextType {
  const context = useContext(NowPlayingContext);
  if (context === undefined) {
    throw new Error("useNowPlaying must be used within a NowPlayingProvider");
  }
  return context;
}
