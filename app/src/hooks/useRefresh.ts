// app/src/hooks/useRefresh.ts
import { useCallback, useEffect, useRef, useState } from "react";
import { fetchRefreshStatus, startRefresh } from "../lib/api";
import type { RefreshState } from "../types";

export function useRefresh(pollMs = 1000) {
  const [state, setState] = useState<RefreshState>({
    running: false,
    imported: 0,
    total: undefined,
    page: 0,
    error: null,
  });
  const timer = useRef<number | null>(null);

  const stopTimer = () => {
    if (timer.current) {
      window.clearInterval(timer.current);
      timer.current = null;
    }
  };

  const poll = useCallback(async () => {
    try {
      const s = await fetchRefreshStatus();
      setState(s);
      if (!s.running) {
        stopTimer();
      }
    } catch (e) {
      // Stop polling on errors to avoid loops
      stopTimer();
    }
  }, []);

  const begin = useCallback(async () => {
    await startRefresh();
    // kick immediate poll
    await poll();
    // then periodic
    stopTimer();
    timer.current = window.setInterval(poll, pollMs) as unknown as number;
  }, [poll, pollMs]);

  useEffect(() => () => stopTimer(), []);

  return { state, start: begin };
}
