// app/src/components/ui/Masonry.tsx
import { ReactNode } from "react";

/**
 * Masonry layout using CSS multi-columns.
 * Children must avoid breaking across columns (handled in Card).
 */
export default function Masonry({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={`columns-1 lg:columns-2 [column-gap:1rem] ${className}`}>{children}</div>;
}
