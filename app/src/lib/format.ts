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

// minutes (integer) -> human long span choosing largest sensible unit
// Examples:
//  - 90m -> "1h 30m"
//  - 72h -> "3.0 days"
//  - very long -> "9.8 years", "2.3 centuries", "1.1 millennia"
export const fmtLongSpanFromMinutes = (minutes: number) => {
  if (!isFinite(minutes) || minutes <= 0) return "0m";
  const hours = minutes / 60;
  // For short spans, keep detailed h/m format
  if (hours < 48) {
    const h = Math.floor(hours);
    const m = Math.round((hours - h) * 60);
    if (h > 0) return m > 0 ? `${h}h ${m}m` : `${h}h`;
    return `${m}m`;
  }

  // Use progressively larger units for very long spans
  const H_PER_DAY = 24;
  const H_PER_WEEK = H_PER_DAY * 7;
  const H_PER_MONTH = H_PER_DAY * 30.4375; // average month
  const H_PER_YEAR = H_PER_DAY * 365.2425; // tropical year average
  const H_PER_CENTURY = H_PER_YEAR * 100;
  const H_PER_MILLENNIUM = H_PER_YEAR * 1000;

  const units: { label: string; hours: number }[] = [
    { label: "millennium", hours: H_PER_MILLENNIUM },
    { label: "century", hours: H_PER_CENTURY },
    { label: "year", hours: H_PER_YEAR },
    { label: "month", hours: H_PER_MONTH },
    { label: "week", hours: H_PER_WEEK },
    { label: "day", hours: H_PER_DAY },
  ];

  for (const u of units) {
    const v = hours / u.hours;
    if (v >= 1) {
      const n = v >= 10 ? Math.round(v) : Math.round(v * 10) / 10; // keep one decimal for <10
      const label = n === 1 ? u.label : u.label === "millennium" ? "millennia" : `${u.label}s`;
      return `${n} ${label}`;
    }
  }

  // Fallback (should not reach): days with one decimal
  const days = hours / H_PER_DAY;
  return `${Math.round(days * 10) / 10} days`;
};

// hours (float) -> human long span via minutes
export const fmtLongSpanFromHours = (hours: number) => {
  if (!isFinite(hours) || hours <= 0) return "0m";
  return fmtLongSpanFromMinutes(Math.round(hours * 60));
};

// hours (float) -> compact hierarchical span using weeks/days/hours/minutes
// Examples:
//  - 2.0 days -> "2d"
//  - 2.5 days -> "2d 12h"
//  - 9.2 days -> "1w 2d 5h"
//  - 3.2 hours -> "3h 12m"
//  - 45 minutes -> "45m"
export const fmtSpanDHMW = (hours: number) => {
  if (!isFinite(hours) || hours <= 0) return "0m";
  let totalMin = Math.round(hours * 60);
  const MIN_PER_HOUR = 60;
  const MIN_PER_DAY = 24 * MIN_PER_HOUR;
  const MIN_PER_WEEK = 7 * MIN_PER_DAY;

  const weeks = Math.floor(totalMin / MIN_PER_WEEK);
  totalMin -= weeks * MIN_PER_WEEK;
  const days = Math.floor(totalMin / MIN_PER_DAY);
  totalMin -= days * MIN_PER_DAY;
  const hrs = Math.floor(totalMin / MIN_PER_HOUR);
  const mins = totalMin % MIN_PER_HOUR;

  const parts: string[] = [];
  if (weeks > 0) {
    parts.push(`${weeks}w`);
    if (days > 0) parts.push(`${days}d`);
    if (hrs > 0) parts.push(`${hrs}h`);
    return parts.join(" ");
  }
  if (days > 0) {
    parts.push(`${days}d`);
    if (hrs > 0) parts.push(`${hrs}h`);
    if (mins > 0) parts.push(`${mins}m`);
    return parts.join(" ");
  }
  if (hrs > 0) {
    parts.push(`${hrs}h`);
    if (mins > 0) parts.push(`${mins}m`);
    return parts.join(" ");
  }
  return `${mins}m`;
};
