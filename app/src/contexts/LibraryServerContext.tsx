import { createContext, useContext, useCallback, useEffect, useState, ReactNode } from "react";
import type { ServerAlias } from "../types/multi-server";

interface LibraryServerContextValue {
  server: ServerAlias;
  setServer: (value: ServerAlias) => void;
}

const LibraryServerContext = createContext<LibraryServerContextValue | undefined>(undefined);

const STORAGE_KEY = "ea_library_server";

function getInitialServer(): ServerAlias {
  if (typeof window === "undefined") return "all";
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === "emby" || stored === "plex" || stored === "jellyfin" || stored === "all") {
      return stored;
    }
  } catch {
    // ignore
  }
  return "all";
}

export function LibraryServerProvider({ children }: { children: ReactNode }) {
  const [server, setServerState] = useState<ServerAlias>(getInitialServer);

  useEffect(() => {
    try {
      if (typeof window !== "undefined") {
        window.localStorage.setItem(STORAGE_KEY, server);
      }
    } catch {
      // ignore storage errors
    }
  }, [server]);

  const setServer = useCallback((value: ServerAlias) => {
    setServerState(value);
  }, []);

  return (
    <LibraryServerContext.Provider value={{ server, setServer }}>
      {children}
    </LibraryServerContext.Provider>
  );
}

export function useLibraryServer() {
  const ctx = useContext(LibraryServerContext);
  if (!ctx) {
    throw new Error("useLibraryServer must be used within LibraryServerProvider");
  }
  return ctx;
}
