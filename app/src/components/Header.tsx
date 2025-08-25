// app/src/components/Header.tsx
import { useEffect, useState } from 'react';
import { useUsage, useNowSnapshot, useRefreshStatus } from '../hooks/useData';
import { startRefresh } from '../lib/api';

type SnapshotEntry = {
  play_method?: string;
};

export default function Header() {
  const [currentTime, setCurrentTime] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  // Use SWR for data fetching
  const { data: weeklyUsage = [], error: usageError } = useUsage(7);
  const { data: nowPlaying = [], error: snapshotError } = useNowSnapshot();
  const { data: refreshStatus, error: refreshError } = useRefreshStatus(refreshing);

  // Calculate derived state from SWR data
  const weeklyHours = weeklyUsage.reduce((acc, r) => acc + (r.hours || 0), 0);
  const streamsTotal = nowPlaying.length;
  const directPlay = nowPlaying.filter((s: SnapshotEntry) => 
    s.play_method === "DirectPlay" || s.play_method === "DirectStream"
  ).length;
  const transcoding = streamsTotal - directPlay;

  // Calculate progress percentage from refreshStatus
  const progress = refreshStatus && refreshStatus.total && refreshStatus.total > 0
    ? Math.max(0, Math.min(100, (refreshStatus.imported / refreshStatus.total) * 100))
    : 0;

  // Clock update effect
  useEffect(() => {
    const updateTime = () => setCurrentTime(new Date().toLocaleTimeString());
    updateTime();
    const t = setInterval(updateTime, 1000);
    return () => clearInterval(t);
  }, []);

  // Monitor refresh status
  useEffect(() => {
    if (refreshStatus && refreshing) {
      if (refreshStatus.running === false) {
        setRefreshing(false);
        showToast('Refresh completed successfully!');
      }
      
      if (refreshStatus.error) {
        setRefreshing(false);
        showToast(`Refresh failed: ${refreshStatus.error}`);
      }
    }
  }, [refreshStatus, refreshing]);

  const showToast = (msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  };

  const handleRefresh = async () => {
    if (refreshing) return;

    try {
      setRefreshing(true);
      await startRefresh();
      showToast('Refresh started...');
    } catch (err) {
      setRefreshing(false);
      showToast('Failed to start refresh');
      console.error('Refresh error:', err);
    }
  };

  return (
    <header className="bg-neutral-900 border-b border-neutral-700 px-6 py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-8">
          <h1 className="text-2xl font-bold text-white">Emby Analytics</h1>
          
          {/* Current Time */}
          <div className="text-gray-400">
            <span className="text-sm">Current Time: </span>
            <span className="text-white font-mono">{currentTime}</span>
          </div>
        </div>

        <div className="flex items-center gap-6">
          {/* Weekly Hours */}
          <div className="text-center">
            <div className="text-sm text-gray-400">Weekly Hours</div>
            <div className="text-xl font-bold text-white">
              {usageError ? (
                <span className="text-red-400 text-sm">Error</span>
              ) : (
                weeklyHours.toFixed(1)
              )}
            </div>
          </div>

          {/* Current Streams */}
          <div className="text-center">
            <div className="text-sm text-gray-400">Streams</div>
            <div className="text-xl font-bold text-white">
              {snapshotError ? (
                <span className="text-red-400 text-sm">Error</span>
              ) : (
                <span>
                  {streamsTotal} 
                  {streamsTotal > 0 && (
                    <span className="text-sm text-gray-400 ml-1">
                      ({directPlay}D/{transcoding}T)
                    </span>
                  )}
                </span>
              )}
            </div>
          </div>

          {/* Refresh Button */}
          <div className="relative">
            <button
              onClick={handleRefresh}
              disabled={refreshing}
              className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                refreshing
                  ? 'bg-yellow-600 text-white cursor-not-allowed'
                  : 'bg-blue-600 hover:bg-blue-700 text-white'
              }`}
            >
              {refreshing ? (
                <span>
                  Refreshing... {progress.toFixed(0)}%
                  {refreshStatus && refreshStatus.total && (
                    <span className="text-xs ml-1">
                      ({refreshStatus.imported}/{refreshStatus.total})
                    </span>
                  )}
                </span>
              ) : (
                'Refresh Library'
              )}
            </button>
            
            {/* Progress Bar */}
            {refreshing && (
              <div className="absolute -bottom-1 left-0 w-full h-1 bg-gray-600 rounded-b-lg overflow-hidden">
                <div 
                  className="h-full bg-yellow-400 transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Toast Notification */}
      {toast && (
        <div className="fixed top-4 right-4 bg-green-600 text-white px-4 py-2 rounded-lg shadow-lg z-50">
          {toast}
        </div>
      )}
    </header>
  );
}