import React from "react";
import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import TopUsers from "../components/TopUsers";
import TopItems from "../components/TopItems";
import QualitiesTable from "../components/QualitiesTable";
import CodecsTable from "../components/CodecsTable";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import NowPlaying from "../components/NowPlaying";
import Masonry from "../components/ui/Masonry";

import PlaybackMethodsCard from "../components/PlaybackMethodsCard";

export default function Dashboard() {
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
            {/* Playback Methods donut */}
            <PlaybackMethodsCard />

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
