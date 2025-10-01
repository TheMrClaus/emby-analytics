import { ReactNode, useEffect, useState } from "react";

/**
 * ClientOnly delays rendering of dynamic UI until after hydration finishes.
 * This prevents hydration mismatches when server-rendered markup differs
 * from client state (e.g. components that rely on window, timers, or live data).
 */
export default function ClientOnly({ children }: { children: ReactNode }) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) return null;
  return <>{children}</>;
}
