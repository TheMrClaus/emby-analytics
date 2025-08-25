import { useEffect, useState } from "react";
import { fetchOverview } from "../lib/api";
import type { OverviewData } from "../types";
import { fmtInt } from "../lib/format";

export default function OverviewCards() {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400">Users</div>
        <div className="text-2xl font-bold text-white">1</div>
      </div>

      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400">Library Items</div>
        <div className="text-2xl font-bold text-white">348,019</div>
      </div>

      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400">Total Plays</div>
        <div className="text-2xl font-bold text-white">44,936</div>
      </div>

      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400">Unique Plays</div>
        <div className="text-2xl font-bold text-white">15</div>
      </div>
    </div>
  );
}


function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-4">
      <div className="ty-caption">{label}</div>
      <div className="ty-title mt-1">{value}</div>
    </div>
  );
}

