// app/src/components/OverviewCards.tsx
import { useEffect, useState } from "react";
import { fetchOverview } from "../lib/api";
import type { OverviewData } from "../types";
import { fmtInt } from "../lib/format";

export default function OverviewCards() {
  const [data, setData] = useState<OverviewData | null>(null);

  useEffect(() => {
    fetchOverview().then(setData).catch(() => {});
  }, []);

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
      <Card label="Users" value={data ? fmtInt(data.total_users) : "—"} />
      <Card label="Library Items" value={data ? fmtInt(data.total_items) : "—"} />
      <Card label="Total Plays" value={data ? fmtInt(data.total_plays) : "—"} />
      <Card label="Unique Plays" value={data ? fmtInt(data.unique_plays) : "—"} />
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

