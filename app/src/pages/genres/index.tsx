import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import Head from "next/head";
import Link from "next/link";
import Header from "../../components/Header";
import Card from "../../components/ui/Card";
import {
  fetchItemsByGenre,
  ItemsByGenreResponse,
  LibraryItemResponse,
  fetchConfig,
} from "../../lib/api";
import { openInEmby } from "../../lib/emby";
import { fmtInt } from "../../lib/format";

export default function GenrePage() {
  const router = useRouter();
  const { genre, media_type } = router.query;
  const [data, setData] = useState<ItemsByGenreResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [embyExternalUrl, setEmbyExternalUrl] = useState<string>("");
  const [embyServerId, setEmbyServerId] = useState<string>("");

  useEffect(() => {
    if (!genre || typeof genre !== "string") {
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);

    fetchItemsByGenre(
      genre,
      page,
      50,
      typeof media_type === "string" ? media_type : undefined
    )
      .then(setData)
      .catch((err) => {
        console.error("Failed to fetch items:", err);
        setError("Failed to load items");
      })
      .finally(() => setLoading(false));
  }, [genre, media_type, page]);

  useEffect(() => {
    fetchConfig()
      .then((config) => {
        setEmbyExternalUrl(config.emby_external_url);
        setEmbyServerId(config.emby_server_id);
      })
      .catch((err) => console.error("Failed to fetch config:", err));
  }, []);

  const formatResolution = (item: LibraryItemResponse) => {
    if (item.height && item.width) return `${item.width}×${item.height}`;
    if (item.height) {
      const width = Math.round((item.height * 16) / 9);
      return `${width}×${item.height}`;
    }
    return "Unknown";
  };

  const getQualityLabel = (height?: number) => {
    if (!height) return "Unknown";
    if (height >= 4320) return "8K";
    if (height >= 2160) return "4K";
    if (height >= 1080) return "1080p";
    if (height >= 720) return "720p";
    if (height >= 480) return "SD";
    return "Unknown";
  };

  const pageTitle = (() => {
    if (!genre || typeof genre !== "string") return "Genre";
    if (typeof media_type === "string") return `${genre} - ${media_type} Items`;
    return `${genre} - Items`;
  })();

  return (
    <>
      <Head>
        <title>{pageTitle} - Emby Analytics</title>
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white">
        <Header />
        <main className="p-4 md:p-6 space-y-6 border-t border-neutral-800">
          {/* Header with back button */}
          <div className="flex items-center gap-4">
            <Link href="/" className="text-gray-400 hover:text-white transition-colors text-xl">
              ←
            </Link>
            <div>
              <h1 className="text-2xl font-bold">{pageTitle}</h1>
              {data && (
                <p className="text-gray-400">{fmtInt(data.total)} items found</p>
              )}
            </div>
          </div>

          {!genre || typeof genre !== "string" ? (
            <Card title="Genre">
              <div className="text-gray-400">No genre specified.</div>
            </Card>
          ) : loading ? (
            <div className="text-center py-12">
              <div className="text-lg">Loading...</div>
            </div>
          ) : error || !data ? (
            <Card title="Genre">
              <div className="text-red-400">{error || "Failed to load data"}</div>
            </Card>
          ) : (
            <Card title={`Items in ${data.genre}`}>
              {data.items.length === 0 ? (
                <div className="text-center py-8 text-gray-400">No items found for this genre</div>
              ) : (
                <>
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm text-left text-gray-300">
                      <thead className="text-gray-400 border-b border-neutral-700">
                        <tr>
                          <th className="py-3">Name</th>
                          <th className="py-3">Type</th>
                          <th className="py-3">Quality</th>
                          <th className="py-3">Resolution</th>
                          <th className="py-3">Codec</th>
                        </tr>
                      </thead>
                      <tbody>
                        {data.items.map((item) => (
                        <tr
                          key={item.id}
                          className="border-b border-neutral-800 last:border-0 hover:bg-neutral-800 cursor-pointer transition-colors"
                          onClick={() => openInEmby(item.id, embyExternalUrl, embyServerId)}
                          title="Click to open in Emby"
                        >
                          <td className="py-3 font-medium">
                            <div>{item.name}</div>
                            {Array.isArray((item as any).genres) && (item as any).genres.length > 0 && (
                              <div className="mt-1 flex flex-wrap gap-1">
                                {(item as any).genres.map((g: string) => (
                                  <a
                                    key={g}
                                    href={`/genres?genre=${encodeURIComponent(g)}`}
                                    onClick={(e) => e.stopPropagation()}
                                    className="text-xs px-2 py-0.5 rounded-full bg-neutral-700 hover:bg-neutral-600 text-gray-200 border border-neutral-600"
                                  >
                                    {g}
                                  </a>
                                ))}
                              </div>
                            )}
                          </td>
                          <td className="py-3">
                            <span className="px-2 py-1 bg-neutral-700 rounded text-xs">
                              {item.media_type}
                            </span>
                          </td>
                            <td className="py-3">{getQualityLabel(item.height)}</td>
                            <td className="py-3">{formatResolution(item)}</td>
                            <td className="py-3">
                              <code className="bg-neutral-800 px-2 py-1 rounded text-xs">{item.codec}</code>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  {/* Pagination */}
                  {Math.ceil(data.total / data.page_size) > 1 && (
                    <div className="flex items-center justify-between mt-4 pt-4 border-t border-neutral-700">
                      <div className="text-sm text-gray-400">
                        Page {page} of {Math.ceil(data.total / data.page_size)} ({fmtInt(data.total)} total items)
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => setPage((p) => Math.max(1, p - 1))}
                          disabled={page <= 1}
                          className="px-3 py-1 bg-neutral-700 hover:bg-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed rounded text-sm transition-colors"
                        >
                          Previous
                        </button>
                        <button
                          onClick={() => setPage((p) => p + 1)}
                          disabled={page >= Math.ceil(data.total / data.page_size)}
                          className="px-3 py-1 bg-neutral-700 hover:bg-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed rounded text-sm transition-colors"
                        >
                          Next
                        </button>
                      </div>
                    </div>
                  )}
                </>
              )}
            </Card>
          )}
        </main>
      </div>
    </>
  );
}
