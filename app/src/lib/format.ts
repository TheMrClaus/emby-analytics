// app/src/lib/format.ts
export const fmtInt = (n: number) => new Intl.NumberFormat().format(n);

// hours (float) -> human axis tick like "1h 30m" (compact)
export const fmtAxisTime = (hours: number) => {
  const totalSec = Math.round(hours * 3600);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  if (h > 0) return m ? `${h}h ${m}m` : `${h}h`;
  return `${m}m`;
};

// hours (float) -> tooltip time "1h 32m 12s"
export const fmtTooltipTime = (hours: number) => {
  const totalSec = Math.round(hours * 3600);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  const parts = [];
  if (h) parts.push(`${h}h`);
  if (m) parts.push(`${m}m`);
  if (s || parts.length === 0) parts.push(`${s}s`);
  return parts.join(" ");
};

export const pct = (n: number) => `${(n * 100).toFixed(1)}%`;

// hours (float) -> display time like "1h 25m" or "25m" for UI cards
export const fmtHours = (hours: number) => {
  const totalSec = Math.round(hours * 3600);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`;
  return `${m}m`;
};
