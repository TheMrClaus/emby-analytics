import React from "react";
import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import QualitiesTable from "../components/QualitiesTable";
import CodecsTable from "../components/CodecsTable";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import NowPlaying from "../components/NowPlaying";
import MultiServerNowPlaying from "../components/MultiServerNowPlaying";
import { ErrorBoundary } from "../components/ErrorBoundary";
import DragDropDashboard from "../components/DragDropDashboard";

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
          <ErrorBoundary>
            <OverviewCards />
          </ErrorBoundary>

          {/* Live sessions (multi-server) */}
          <ErrorBoundary>
            <MultiServerNowPlaying />
          </ErrorBoundary>

          {/* Dashboard cards with drag and drop functionality */}
          <DragDropDashboard />

          {/* Pin Media Qualities & Media Codecs just above Most Active */}
          <div className="space-y-4">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <ErrorBoundary>
                <QualitiesTable />
              </ErrorBoundary>
              <ErrorBoundary>
                <CodecsTable />
              </ErrorBoundary>
            </div>

            <ErrorBoundary>
              <ActiveUsersLifetime limit={10} />
            </ErrorBoundary>
          </div>
        </main>
      </div>
    </>
  );
}
