import React from "react";
import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import QualitiesTable from "../components/QualitiesTable";
import CodecsTable from "../components/CodecsTable";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import NowPlaying from "../components/NowPlaying";
import { ErrorBoundary } from "../components/ErrorBoundary";
import DragDropDashboard from "../components/DragDropDashboard";
import LibraryServerSelector from "../components/LibraryServerSelector";
import ClientOnly from "../components/ClientOnly";

export default function Dashboard() {
  return (
    <>
      <Head>
        <title>Emby Analytics</title>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white overflow-x-hidden">
        <Header />
        <main className="p-4 md:p-6 space-y-6 border-t border-neutral-800">
          <ErrorBoundary>
            <ClientOnly>
              <OverviewCards />
            </ClientOnly>
          </ErrorBoundary>

          {/* Live sessions */}
          <ErrorBoundary>
            <ClientOnly>
              <NowPlaying />
            </ClientOnly>
          </ErrorBoundary>

          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <h2 className="text-lg font-semibold text-gray-200">Library Server Filter</h2>
            <LibraryServerSelector />
          </div>

          {/* Dashboard cards with drag and drop functionality */}
          <ClientOnly>
            <DragDropDashboard />
          </ClientOnly>

          {/* Pin Media Qualities & Media Codecs just above Most Active */}
          <div className="space-y-4">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <ErrorBoundary>
                <ClientOnly>
                  <QualitiesTable />
                </ClientOnly>
              </ErrorBoundary>
              <ErrorBoundary>
                <ClientOnly>
                  <CodecsTable />
                </ClientOnly>
              </ErrorBoundary>
            </div>

            <ErrorBoundary>
              <ClientOnly>
                <ActiveUsersLifetime limit={10} />
              </ClientOnly>
            </ErrorBoundary>
          </div>
        </main>
      </div>
    </>
  );
}
