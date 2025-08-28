// app/src/components/ui/Card.tsx
import { ReactNode } from "react";

export default function Card({
  title,
  right,
  children,
  className = "",
}: {
  title: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={
        // inline-block + w-full so each card is a single masonry item
        // break-inside-avoid prevents splitting between columns
        `bg-neutral-800 rounded-2xl p-4 shadow inline-block w-full align-top
         break-inside-avoid mb-4 ${className}`
      }
    >
      <div className="flex justify-between items-center mb-2">
        <div className="text-sm text-gray-400">{title}</div>
        {right ? <div>{right}</div> : null}
      </div>
      {children}
    </div>
  );
}
