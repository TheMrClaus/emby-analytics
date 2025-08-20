// app/src/hooks/useNowStream.ts
import { useEffect, useRef, useState } from "react";
import type { NowEntry } from "../types";

export function useNowStream() {
  const [items, setItems] = useState<NowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const connect = () => {
      const proto = window.location.protocol === "https:" ? "wss" : "ws";
      const url = `${proto}://${window.location.host}/now/ws`;
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        setError(null);
      };

      ws.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data);
          setItems(Array.isArray(data) ? data : [data]);
        } catch {
          setError("Invalid WS payload");
        }
      };

      ws.onerror = () => {
        setError("WebSocket error");
      };

      ws.onclose = () => {
        setError("WebSocket closed, retrying...");
        if (!reconnectTimer.current) {
          reconnectTimer.current = setTimeout(() => {
            reconnectTimer.current = null;
            connect();
          }, 2000);
        }
      };
    };

    connect();

    return () => {
      if (wsRef.current) wsRef.current.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    };
  }, []);

  return { items, error };
}
