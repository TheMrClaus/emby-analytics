import { useRouter } from "next/router";
import { useEffect, useState } from "react";
import Head from "next/head";
import Link from "next/link";
import Header from "../../components/Header";
import Card from "../../components/ui/Card";
import { fetchItemsByCodec, ItemsByCodecResponse, LibraryItemResponse, fetchConfig } from "../../lib/api";
import { openInEmby } from "../../lib/emby"; 
import { fmtInt } from "../../lib/format";

export default function CodecDetailPage() {
  const router = useRouter();
  const { codec, media_type } = router.query;
  const [data, setData] = useState<ItemsByCodecResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [embyExternalUrl, setEmbyExternalUrl] = useState<string>('');
  const [embyServerId, setEmbyServerId] = useState<string>('');

  useEffect(() => {
    if (!codec || typeof codec !== "string") return;

    setLoading(true);
    setError(null);

    fetchItemsByCodec(
      codec,
      page,
      50,
      typeof media_type === "string" ? media_type : undefined
    )
      .then(setData)
      .catch((err) => {
        console.error('Failed to fetch items:', err);
        setError('Failed to load items');
      })
      .finally(() => setLoading(false));
  }, [codec, media_type, page]);

  // Fetch config once on component mount to get Emby external URL
  useEffect(() => {
    fetchConfig()
      .then(config => {
        setEmbyExternalUrl(config.emby_external_url);
        setEmbyServerId(config.emby_server_id);
      })
      .catch(err => console.error('Failed to fetch config:', err));
  }, []);

  const formatResolution = (item: LibraryItemResponse) => {
    if (item.height && item.width) {
      return `${item.width}×${item.height}`;
    } else if (item.height) {
      const width = Math.round(item.height * 16 / 9);
      return `${width}×${item.height}`;
    }
    return "Unknown";
  };

  const getQualityLabel = (height?: number) => {
    if (!height) return "Unknown";
    if (height >= 2160) return "4K";
    if (height >= 1080) return "1080p";
    if (height >= 720) return "720p";
    if (height >= 480) return "SD";
    return "Unknown";
  };

  if (loading) {
    return (
      <>
        <Head>
          <title>Loading Codec Details - Emby Analytics</title>
        </Head>
        <div className="min-h-screen bg-neutral-900 text-white">
          <Header />
          <main className="p-4 md:p-6">
            <div className="text-center py-12">
              <div className="text-lg">Loading...</div>
            </div>
          </main>
        </div>
      </>
    );
  }

  if (error || !data) {
    return (
      <>
        <Head>
          <title>Error - Emby Analytics</title>
        </Head>
        <div className="min-h-screen bg-neutral-900 text-white">
          <Header />
          <main className="p-4 md:p-6">
            <div className="text-center py-12">
              <div className="text-red-400 text-lg">{error || "Failed to load data"}</div>
              <Link href="/" className="text-blue-400 hover:text-blue-300 mt-4 inline-block">
                ← Back to Dashboard
              </Link>
            </div>
          </main>
        </div>
      </>
    );
  }

  const title = media_type 
    ? `${codec} Codec - ${media_type} Items` 
    : `${codec} Codec - All Items`;

  const totalPages = Math.ceil(data.total / data.page_size);

  return (
    <>
      <Head>
        <title>{title} - Emby Analytics</title>
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
              <h1 className="text-2xl font-bold">{title}</h1>
              <p className="text-gray-400">{fmtInt(data.total)} items found</p>
            </div>
          </div>

          {/* Items Table */}
          <Card title={`Items using ${codec} codec`}>
            {data.items.length === 0 ? (
              <div className="text-center py-8 text-gray-400">
                No items found for this codec
              </div>
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
                          <td className="py-3 font-medium">{item.name}</td>
                          <td className="py-3">
                            <span className="px-2 py-1 bg-neutral-700 rounded text-xs">
                              {item.media_type}
                            </span>
                          </td>
                          <td className="py-3">{getQualityLabel(item.height)}</td>
                          <td className="py-3">{formatResolution(item)}</td>
                          <td className="py-3">
                            <code className="bg-neutral-800 px-2 py-1 rounded text-xs">
                              {item.codec}
                            </code>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Pagination */}
                {totalPages > 1 && (
                  <div className="flex items-center justify-between mt-4 pt-4 border-t border-neutral-700">
                    <div className="text-sm text-gray-400">
                      Page {page} of {totalPages} ({fmtInt(data.total)} total items)
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => setPage(p => Math.max(1, p - 1))}
                        disabled={page <= 1}
                        className="px-3 py-1 bg-neutral-700 hover:bg-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed rounded text-sm transition-colors"
                      >
                        Previous
                      </button>
                      <button
                        onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                        disabled={page >= totalPages}
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
        </main>
      </div>
    </>
  );
}