// app/src/components/Header.tsx
import { useState, useEffect } from 'react';

export default function Header() {
  const [currentTime, setCurrentTime] = useState('');
  
  useEffect(() => {
    const updateTime = () => {
      const now = new Date();
      setCurrentTime(now.toLocaleTimeString());
    };
    
    updateTime();
    const interval = setInterval(updateTime, 1000);
    
    return () => clearInterval(interval);
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
            <p className="text-sm text-gray-400">Reimagined — Black/Yellow premium</p>
          </div>
        </div>

        {/* Right side - Statistics */}
        <div className="text-right">
          <div className="text-xs text-gray-400 uppercase tracking-wide">THIS WEEK</div>
          <div className="text-2xl font-bold text-yellow-400">41.7h watched</div>
        </div>
      </div>

      {/* Bottom row */}
      <div className="flex items-center justify-between">
        {/* Status section */}
        <div className="flex items-center gap-8">
          <div>
            <div className="text-xs text-gray-400 uppercase tracking-wide mb-1">STATUS</div>
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 bg-green-400 rounded-full"></div>
              <span className="text-sm text-gray-300">Auto-refresh: every 2s (mock)</span>
            </div>
          </div>

          <div className="text-2xl font-bold text-white">3</div>

          <div>
            <div className="text-xs text-gray-400 uppercase tracking-wide mb-1">ACTIVE STREAMS</div>
            <div className="flex gap-4">
              <span className="bg-teal-600 text-white px-2 py-1 rounded text-sm">DirectPlay 2</span>
              <span className="bg-orange-600 text-white px-2 py-1 rounded text-sm">Transcoding 1</span>
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