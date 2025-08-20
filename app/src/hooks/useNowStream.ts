// app/src/hooks/useNowStream.ts
import { useEffect, useRef, useState } from "react";
import type { NowEntry } from "../types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

/**
 * Uses SSE stream (/now/stream) if available. Falls back to polling snapshot.
 */
export function useNowStream(pollFallbackMs = 5000) {
  const [items, setItems] = useState<NowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    let closed = false;

    const startPolling = () => {
      // Avoid duplicate timers
      if (pollRef.current) return;
      const pollOnce = async () => {
        try {
          const res = await fetch(`${API_BASE}/now/snapshot`);
          if (!res.ok) throw new Error(res.statusText);
          const data = (await res.json()) as NowEntry[];
          if (!closed) setItems(data ?? []);
        } catch (e: any) {
          if (!closed) setError(e?.message ?? "Polling error");
        }
      };
      pollOnce();
      pollRef.current = window.setInterval(pollOnce, pollFallbackMs) as unknown as number;
    };

    try {
      const es = new EventSource(`${API_BASE}/now/stream`);
      esRef.current = es;

      es.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data);
          // stream may send an array or single object; normalize
       	  const arr = Array.isArray(data) ? data : [data];
          setItems(arr);
          setError(null);
        } catch (e) {
          // malformed event; ignore
        }
      };
      es.onerror = () => {
        setError("SSE disconnected; using polling");
        es.close();
        startPolling();
      };
    } catch {
      setError("SSE not supported; using polling");
      startPolling();
    }

    return () => {
      closed = true;
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      if (pollRef.current) {
        window.clearInterval(pollRef.current);
        pollRef.current = null;
      }
    };
  }, [pollFallbackMs]);

  return { items, error };
}

