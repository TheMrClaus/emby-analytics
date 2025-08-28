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

          {/* ADD THIS COMPONENT BACK */}
          <NowPlaying />

          {/* Dashboard cards in masonry (Tetris) layout */}
          <Masonry className="mt-4">
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