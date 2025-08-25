// app/src/components/OverviewCards.tsx
import { useOverview } from "../hooks/useData";
import type { OverviewData } from "../types";
import { fmtInt } from "../lib/format";

export default function OverviewCards() {
  const { data, error, isLoading } = useOverview();

  if (error) {
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map(i => (
          <div key={i} className="bg-neutral-800 rounded-2xl p-4 shadow">
            <div className="text-sm text-gray-400">Error</div>
            <div className="text-red-400 text-sm">Failed to load</div>
          </div>
        ))}
      </div>
    );
  }

  // Default values while loading or if data is missing
  const overview = data || {
    total_users: 0,
    total_items: 0,
    total_plays: 0,
    unique_plays: 0
  };

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <Card 
        label="Users" 
        value={isLoading ? "..." : fmtInt(overview.total_users)}
      />
      <Card 
        label="Library Items" 
        value={isLoading ? "..." : fmtInt(overview.total_items)}
      />
      <Card 
        label="Total Plays" 
        value={isLoading ? "..." : fmtInt(overview.total_plays)}
      />
      <Card 
        label="Unique Plays" 
        value={isLoading ? "..." : fmtInt(overview.unique_plays)}
      />
    </div>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400">{label}</div>
      <div className="text-2xl font-bold text-white">{value}</div>
    </div>
  );
}