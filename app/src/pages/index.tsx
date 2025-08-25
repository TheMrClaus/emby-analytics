// app/src/pages/index.tsx
import Head from "next/head";
import Header from "../components/Header";
import OverviewCards from "../components/OverviewCards";
import UsageChart from "../components/UsageChart";
import TopUsers from "../components/TopUsers";
import TopItems from "../components/TopItems";
import QualitiesChart from "../components/QualitiesChart";
import CodecsChart from "../components/CodecsChart";
import ActiveUsersLifetime from "../components/ActiveUsersLifetime";
import RefreshControls from "../components/RefreshControls";
import NowPlaying from "../components/NowPlaying";

export default function Dashboard() {
  return (
    <div className="bg-neutral-900 text-white p-4 border-t border-neutral-800">
      <div className="grid grid-cols-4 gap-4">
        <div className="bg-neutral-800 rounded-2xl p-4 shadow">
          <div className="text-sm text-gray-400">Users</div>
          <div className="text-2xl font-bold">1</div>
        </div>

        <div className="bg-neutral-800 rounded-2xl p-4 shadow">
          <div className="text-sm text-gray-400">Library Items</div>
          <div className="text-2xl font-bold">348,019</div>
        </div>

        <div className="bg-neutral-800 rounded-2xl p-4 shadow">
          <div className="text-sm text-gray-400">Total Plays</div>
          <div className="text-2xl font-bold">44,936</div>
        </div>

        <div className="bg-neutral-800 rounded-2xl p-4 shadow">
          <div className="text-sm text-gray-400">Unique Plays</div>
          <div className="text-2xl font-bold">15</div>
        </div>
      </div>
    </div>
  );
}
