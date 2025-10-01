import { ServerAlias } from "../types/multi-server";
import { useLibraryServer } from "../contexts/LibraryServerContext";

const options: { key: ServerAlias; label: string }[] = [
  { key: "all", label: "All Servers" },
  { key: "emby", label: "Emby" },
  { key: "jellyfin", label: "Jellyfin" },
  { key: "plex", label: "Plex" },
];

export default function LibraryServerSelector() {
  const { server, setServer } = useLibraryServer();

  return (
    <div className="inline-flex items-center gap-2 bg-neutral-800 border border-neutral-700 rounded-xl p-1">
      {options.map((opt) => (
        <button
          key={opt.key}
          onClick={() => setServer(opt.key)}
          className={`px-3 py-1 rounded-lg text-xs md:text-sm transition-colors ${
            server === opt.key
              ? "bg-neutral-700 text-white border border-neutral-600"
              : "bg-transparent text-gray-300 hover:text-white"
          }`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
