import { ServerAlias } from "../types/multi-server";
import { useMultiServer } from "../contexts/MultiServerContext";

const options: { key: ServerAlias; label: string }[] = [
  { key: "all", label: "All" },
  { key: "emby", label: "Emby" },
  { key: "plex", label: "Plex" },
  { key: "jellyfin", label: "Jellyfin" },
];

export default function ServerSelector() {
  const { server, setServer } = useMultiServer();
  return (
    <div className="inline-flex gap-2 bg-neutral-800 border border-neutral-700 rounded-xl p-1">
      {options.map((o) => (
        <button
          key={o.key}
          onClick={() => setServer(o.key)}
          className={`px-3 py-1 rounded-lg text-sm ${
            server === o.key
              ? "bg-neutral-700 text-white border border-neutral-600"
              : "bg-transparent text-gray-300 hover:text-white"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
