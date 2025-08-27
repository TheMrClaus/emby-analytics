import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import UsageChart from "../components/UsageChart";
import TopUsers from "../components/TopUsers";
import TopItems from "../components/TopItems";
import QualitiesChart from "../components/QualitiesChart";
import CodecsChart from "../components/CodecsChart";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import NowPlaying from "../components/NowPlaying";

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

          <div className="grid lg:grid-cols-2 gap-4">
            <TopUsers limit={10} />
            <TopItems limit={10} />
          </div>

          <div className="grid lg:grid-cols-2 gap-4">
            <QualitiesChart />
            <CodecsChart />
          </div>

          <ActiveUsersLifetime limit={10} />
        </main>
      </div>
    </>
  );
}