import React from "react";
import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import UsageChart from "../components/UsageChart";
import TopUsers from "../components/TopUsers";
import TopItems from "../components/TopItems";
import QualitiesTable from "../components/QualitiesTable";
import CodecsTable from "../components/CodecsTable";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import NowPlaying from "../components/NowPlaying";
import Masonry from "../components/ui/Masonry";

import PlaybackMethodsCard from "../components/PlaybackMethodsCard";
import type { PlayMethodCounts } from "../types";

export default function Dashboard() {
  const [playMethods, setPlayMethods] = React.useState<PlayMethodCounts | null>(null);
  const [pmError, setPmError] = React.useState<string | null>(null);

  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/stats/play-methods?days=30");
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const json = (await res.json()) as PlayMethodCounts;
        if (!cancelled) setPlayMethods(json);
      } catch (e: any) {
        if (!cancelled) setPmError(e?.message || "Failed to load playback methods");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <>
      <Head>
        <title>Emby Analytics</title>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white">
        <Header />
        <main className="p-4 md:p-6 space-y-6 border-t border-neutral-800">
          <OverviewCards />

          {/* Live sessions */}
          <NowPlaying />

          {/* Dashboard cards in masonry (Tetris) layout */}
          <Masonry className="mt-4">
            {/* New: Playback Methods donut */}
            {pmError ? (
              <div className="bg-neutral-800 rounded-2xl p-4 shadow inline-block w-full align-top break-inside-avoid mb-4">
                <div className="text-sm text-gray-400 mb-2">Playback Methods</div>
                <div className="text-red-400">{pmError}</div>
              </div>
            ) : (
              <PlaybackMethodsCard data={playMethods} />
            )}

            <TopUsers />
            <TopItems />
            <QualitiesTable />
            <CodecsTable />
          </Masonry>

          <ActiveUsersLifetime limit={10} />
        </main>
      </div>
    </>
  );
}
