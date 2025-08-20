// app/src/hooks/useNowStream.ts
import { useEffect, useRef, useState } from "react";
import type { NowEntry } from "../types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

export function useNowStream() {
  const [items, setItems] = useState<NowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const url = `${proto}://${window.location.host}/now/ws`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data);
        const arr = Array.isArray(data) ? data : [data];
        setItems(arr);
        setError(null);
      } catch {
        setError("Invalid data from server");
      }
    };

    ws.onerror = () => {
      setError("WebSocket error");
    };

    ws.onclose = () => {
      setError("WebSocket closed, retrying...");
      // try to reconnect after 2s
      setTimeout(() => {
        if (!wsRef.current || wsRef.current.readyState === WebSocket.CLOSED) {
          // re-init by re-running the effect
          setItems([]);
          setError("Reconnecting...");
          window.location.reload(); // simplest auto-reconnect
        }
      }, 2000);
    };

    return () => {
      ws.close();
    };
  }, []);

  return { items, error };
}
