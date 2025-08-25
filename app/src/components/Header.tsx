// app/src/components/Header.tsx
import { useEffect, useState } from 'react';
import {
  fetchUsage,
  fetchNowSnapshot,
  startRefresh,
  fetchRefreshStatus,
} from '../lib/api';

type SnapshotEntry = {
  play_method?: string;
};

export default function Header() {
  const [currentTime, setCurrentTime] = useState('');
  const [weeklyHours, setWeeklyHours] = useState<number | null>(null);
  const [streamsTotal, setStreamsTotal] = useState<number>(0);
  const [directPlay, setDirectPlay] = useState<number>(0);
  const [transcoding, setTranscoding] = useState<number>(0);

  const [refreshing, setRefreshing] = useState(false);
  const [progress, setProgress] = useState(0);
  const [toast, setToast] = useState<string | null>(null);

  // ----- clock -----
  useEffect(() => {
    const updateTime = () => setCurrentTime(new Date().toLocaleTimeString());
    updateTime();
    const t = setInterval(updateTime, 1000);
    return () => clearInterval(t);
  }, []);

  // ----- weekly usage -----
  useEffect(() => {
    (async () => {
      try {
        const rows = await fetchUsage(7);
        const total = rows.reduce((acc, r) => acc + (r.hours || 0), 0);
        setWeeklyHours(total);
      } catch {
        setWeeklyHours(null);
      }
    })();
  }, []);

  // ----- live "now playing" -----
  useEffect(() => {
    let stop = false;

    const load = async () => {
      try {
        const sessions: SnapshotEntry[] = await fetchNowSnapshot();
        if (stop) return;

        const total = sessions.length;
        const d = sessions.filter(
          s => (s.play_method ?? '').toLowerCase().startsWith('direct')
        ).length;
        const t = sessions.filter(
          s => (s.play_method ?? '').toLowerCase().startsWith('trans')
        ).length;

        setStreamsTotal(total);
        setDirectPlay(d);
        setTranscoding(t);
      } catch {
        /* ignore */
      }
    };

    load();
    const id = setInterval(load, 2000);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, []);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  };

  // refresh logic
  const handleRefresh = async () => {
    if (refreshing) return;

    try {
      setRefreshing(true);
      setProgress(0);

      await startRefresh();

      const poll = setInterval(async () => {
        try {
          const s = await fetchRefreshStatus();
          const pct = Math.max(0, Math.min(100, Number((s as any).progress ?? 0)));
          setProgress(pct);

          if ((s as any).status === 'done' || pct >= 100) {
            clearInterval(poll);
            setRefreshing(false);
            setProgress(100);
            showToast('Refresh complete ✅');
            fetchUsage(7)
              .then(rows => setWeeklyHours(rows.reduce((acc, r) => acc + (r.hours || 0), 0)))
              .catch(() => {});
          }
        } catch {
          clearInterval(poll);
          setRefreshing(false);
          setProgress(0);
          showToast('Refresh failed ❌');
        }
      }, 1000);
    } catch {
      setRefreshing(false);
      setProgress(0);
      showToast('Could not start refresh ❌');
    }
  };

  return (
    <header className="bg-black text-white px-6 py-3">
      {/* Top row */}
      <div className="flex items-center justify-between">
        {/* Left side: title + clock */}
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-yellow-500 rounded flex items-center justify-center">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" className="text-black">
              <path d="M8 5v14l11-7z" fill="currentColor"/>
            </svg>
          </div>
          <div>
            <h1 className="text-xl font-semibold text-white">Emby Analytics</h1>
            <p className="text-sm text-gray-400">
              <span className="tabular-nums">{currentTime}</span>
            </p>
          </div>
        </div>

        {/* Right side: THIS WEEK */}
        <div className="text-right">
          <div className="text-xs text-gray-400 uppercase tracking-wide">THIS WEEK</div>
          <div className="text-2xl font-bold text-yellow-400">
            {weeklyHours == null ? '—' : `${weeklyHours.toFixed(1)}h`} watched
          </div>
        </div>
      </div>

      {/* Bottom row */}
      <div className="flex items-center justify-between mt-3">
        {/* Left: Active Streams */}
        <div>
          <div className="text-xs text-gray-400 uppercase tracking-wide mb-1">
            ACTIVE STREAMS:{' '}
            <span className="text-2xl font-bold text-white tabular-nums">{streamsTotal}</span>
          </div>
          <div className="flex gap-4 mt-1">
            <span className="bg-teal-600 text-white px-2 py-1 rounded text-sm">
              DirectPlay {directPlay}
            </span>
            <span className="bg-orange-600 text-white px-2 py-1 rounded text-sm">
              Transcoding {transcoding}
            </span>
          </div>
        </div>

        {/* Right: Refresh button */}
        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className={`relative overflow-hidden min-w-[140px] rounded font-medium text-sm px-4 py-2 transition-colors
            ${refreshing ? 'bg-yellow-700 text-black' : 'bg-yellow-600 hover:bg-yellow-700 text-black'}`}
          aria-busy={refreshing}
          aria-label="Refresh library"
        >
          <span className="relative z-10">{refreshing ? 'Refreshing…' : 'Refresh'}</span>
          <span
            className="absolute bottom-0 left-0 h-1 bg-yellow-300 transition-[width] duration-300"
            style={{ width: `${progress}%` }}
            aria-hidden
          />
        </button>
      </div>

      {toast && (
        <div className="fixed top-4 right-4 bg-neutral-800 text-white border border-neutral-700 rounded-lg shadow px-4 py-2 text-sm">
          {toast}
        </div>
      )}
    </header>
  );
}
