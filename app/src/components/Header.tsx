// app/src/components/Header.tsx
import { useEffect, useMemo, useState } from 'react';
import { fetchUsage, fetchNowSnapshot } from '../lib/api';

type SnapshotEntry = {
  play_method?: string;
};

export default function Header() {
  const [currentTime, setCurrentTime] = useState('');
  const [weeklyHours, setWeeklyHours] = useState<number | null>(null);
  const [streamsTotal, setStreamsTotal] = useState<number>(0);
  const [directPlay, setDirectPlay] = useState<number>(0);
  const [transcoding, setTranscoding] = useState<number>(0);

  // ----- clock -----
  useEffect(() => {
    const updateTime = () => {
      const now = new Date();
      setCurrentTime(now.toLocaleTimeString());
    };
    updateTime();
    const t = setInterval(updateTime, 1000);
    return () => clearInterval(t);
  }, []);

  // ----- weekly usage (sum last 7 days from /stats/usage) -----
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

  // ----- live "now playing" snapshot (poll every 2s) -----
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
        // if it fails, just keep previous values
      }
    };

    // initial + interval
    load();
    const id = setInterval(load, 2000);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, []);

  return (
    <header className="bg-black text-white p-4">
      {/* Top row */}
      <div className="flex items-center justify-between mb-4">
        {/* Left side - Logo and title */}
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-yellow-500 rounded flex items-center justify-center">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" className="text-black">
              <path d="M8 5v14l11-7z" fill="currentColor"/>
            </svg>
          </div>
          <div>
            <h1 className="text-xl font-semibold text-white">Emby Analytics</h1>
          </div>
        </div>

        {/* Right side - Statistics */}
        <div className="text-right">
          <div className="text-xs text-gray-400 uppercase tracking-wide">THIS WEEK</div>
          <div className="text-2xl font-bold text-yellow-400">
            {weeklyHours == null ? '—' : `${weeklyHours.toFixed(1)}h`} watched
          </div>
        </div>
      </div>

      {/* Bottom row */}
      <div className="flex items-center justify-between">
        {/* Status section */}
        <div className="flex items-center gap-8">

          <div className="text-2xl font-bold text-white tabular-nums">{streamsTotal}</div>

          <div>
            <div className="text-xs text-gray-400 uppercase tracking-wide mb-1">ACTIVE STREAMS</div>
            <div className="flex gap-4">
              <span className="bg-teal-600 text-white px-2 py-1 rounded text-sm">
                DirectPlay {directPlay}
              </span>
              <span className="bg-orange-600 text-white px-2 py-1 rounded text-sm">
                Transcoding {transcoding}
              </span>
            </div>
          </div>
        </div>

        {/* Refresh button */}
        <button
          className="bg-yellow-600 hover:bg-yellow-700 text-black px-4 py-2 rounded font-medium text-sm transition-colors"
          onClick={() => window.location.reload()}
        >
          Refresh now
        </button>
      </div>
    </header>
  );
}
