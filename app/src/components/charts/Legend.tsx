import React from 'react';

type Item = { label: string; color: string; gradientId?: string };

export default function ChartLegend({ items }: { items: Item[] }) {
  return (
    <div className="chart-legend">
      {items.map(({ label, color, gradientId }) => (
        <span className="item" key={label}>
          <span
            className="swatch"
            style={{
              background: gradientId ? `url(#${gradientId})` : color,
              backgroundColor: color,
            }}
          />
          <span>{label}</span>
        </span>
      ))}
    </div>
  );
}
