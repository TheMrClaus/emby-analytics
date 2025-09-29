import React, { useEffect, useState, useCallback, useMemo } from "react";
import Link from "next/link";
import Head from "next/head";
import Header from "../components/Header";
import { useSettings } from "../hooks/useSettings";
import {
  Settings,
  RotateCcw,
  RefreshCcw,
  Check,
  AlertCircle,
  Info,
  ArrowLeft,
  UserPlus,
  Shield,
  User,
  Trash2,
  Pencil,
  Save,
  X,
} from "lucide-react";
import {
  fetchAppUsers,
  createAppUser,
  updateAppUser,
  deleteAppUser,
  AppUser,
  fetchServers,
  MediaServerInfo,
  syncServer,
  deleteServerMedia,
  fetchRuntimeOutliers,
  fetchItemsByIds,
} from "../lib/api";
import type { ItemRow, RuntimeOutlier, RuntimeOutlierResponse, ServerSyncProgress } from "../types";
import useSWR from "swr";
import { useRefreshStatus } from "../hooks/useData";

export default function SettingsPage() {
  const { data: settings, error, isLoading, updateSetting } = useSettings();
  const { data: servers } = useSWR<MediaServerInfo[]>("/api/servers", fetchServers);
  const [saving, setSaving] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<{ key: string; status: "success" | "error" } | null>(
    null
  );
  const [syncingServer, setSyncingServer] = useState<string | null>(null);
  const [serverSyncStatus, setServerSyncStatus] = useState<Record<string, "success" | "error">>({});
  const [deletingServer, setDeletingServer] = useState<string | null>(null);
  const [serverDeleteStatus, setServerDeleteStatus] = useState<Record<string, "success" | "error">>(
    {}
  );
  const { data: refreshStatus } = useRefreshStatus(true);
  const serverProgressMap = useMemo(() => {
    const map: Record<string, ServerSyncProgress> = {};
    refreshStatus?.servers?.forEach((entry) => {
      map[entry.server_id] = entry;
    });
    return map;
  }, [refreshStatus?.servers]);
  const orderedServers = useMemo(() => {
    if (!servers) return undefined;
    return [...servers].sort((a, b) =>
      a.name.localeCompare(b.name, undefined, { sensitivity: "base" })
    );
  }, [servers]);

  const handleToggleChange = async (key: string, currentValue: string) => {
    const newValue = currentValue === "true" ? "false" : "true";
    setSaving(key);
    setSaveStatus(null);

    try {
      await updateSetting(key, newValue);
      setSaveStatus({ key, status: "success" });
      setTimeout(() => setSaveStatus(null), 3000);
    } catch (error) {
      console.error("Failed to update setting:", error);
      setSaveStatus({ key, status: "error" });
      setTimeout(() => setSaveStatus(null), 5000);
    } finally {
      setSaving(null);
    }
  };

  const includeTrakt = settings?.find((s) => s.key === "include_trakt_items")?.value || "false";
  const prevent4kTranscoding =
    settings?.find((s) => s.key === "prevent_4k_video_transcoding")?.value || "false";
  const [meRole, setMeRole] = useState<string | null>(null);
  const [users, setUsers] = useState<AppUser[] | null>(null);
  const [usersError, setUsersError] = useState<string | null>(null);
  const [busyUsers, setBusyUsers] = useState(false);

  const [newUser, setNewUser] = useState<{
    username: string;
    password: string;
    role: "admin" | "user";
  }>({ username: "", password: "", role: "user" });
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editDraft, setEditDraft] = useState<
    Partial<{ username: string; password: string; role: "admin" | "user" }>
  >({});
  const [selectedOutlier, setSelectedOutlier] = useState<RuntimeOutlier | null>(null);

  const runtimeOutlierLimit = 100;
  const shouldLoadRuntimeOutliers = meRole?.toLowerCase() === "admin";
  const {
    data: runtimeOutliers,
    error: runtimeOutliersError,
    isLoading: runtimeOutliersLoading,
    mutate: reloadRuntimeOutliers,
  } = useSWR<RuntimeOutlierResponse>(
    shouldLoadRuntimeOutliers ? ["runtimeOutliers", runtimeOutlierLimit] : null,
    () => fetchRuntimeOutliers(runtimeOutlierLimit),
    { revalidateOnFocus: false }
  );
  const runtimeOutlierThresholdMinutes = runtimeOutliers?.threshold_minutes ?? 24 * 60;
  const runtimeOutlierThresholdHours = Math.round((runtimeOutlierThresholdMinutes / 60) * 10) / 10;
  const runtimeOutlierThresholdDays =
    Math.round((runtimeOutlierThresholdMinutes / (60 * 24)) * 10) / 10;
  const runtimeOutlierItems = runtimeOutliers?.items ?? [];
  const {
    data: selectedOutlierDetails,
    isLoading: selectedOutlierDetailsLoading,
    error: selectedOutlierDetailsError,
  } = useSWR<ItemRow[] | null>(
    selectedOutlier?.item_id ? ["outlier-detail", selectedOutlier.item_id] : null,
    async () => {
      if (!selectedOutlier?.item_id) return null;
      const detail = await fetchItemsByIds([selectedOutlier.item_id]);
      return detail;
    },
    { revalidateOnFocus: false }
  );

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch("/auth/me", { credentials: "include" });
        if (res.ok) {
          const j = await res.json();
          setMeRole(String(j?.role || ""));
        } else {
          setMeRole(null);
        }
      } catch {
        setMeRole(null);
      }
    })();
  }, []);

  const getErrMessage = (e: unknown): string => {
    if (typeof e === "string") return e;
    if (
      e &&
      typeof e === "object" &&
      "message" in e &&
      typeof (e as { message: unknown }).message === "string"
    ) {
      return (e as { message: string }).message;
    }
    return "";
  };

  const loadUsers = useCallback(async () => {
    setBusyUsers(true);
    setUsersError(null);
    try {
      const list = await fetchAppUsers();
      setUsers(list);
    } catch (e: unknown) {
      setUsersError(getErrMessage(e) || "Failed to load users");
    } finally {
      setBusyUsers(false);
    }
  }, []);

  const clearServerSyncStatus = useCallback((serverId: string) => {
    setServerSyncStatus((prev) => {
      const next = { ...prev };
      delete next[serverId];
      return next;
    });
  }, []);

  const clearServerDeleteStatus = useCallback((serverId: string) => {
    setServerDeleteStatus((prev) => {
      const next = { ...prev };
      delete next[serverId];
      return next;
    });
  }, []);

  const handleServerSync = useCallback(
    async (serverId: string) => {
      setSyncingServer(serverId);
      clearServerSyncStatus(serverId);
      try {
        await syncServer(serverId);
        setServerSyncStatus((prev) => ({ ...prev, [serverId]: "success" }));
        setTimeout(() => clearServerSyncStatus(serverId), 4000);
      } catch (error) {
        console.error("Failed to start server sync:", error);
        setServerSyncStatus((prev) => ({ ...prev, [serverId]: "error" }));
        setTimeout(() => clearServerSyncStatus(serverId), 6000);
        setSyncingServer(null);
      }
    },
    [clearServerSyncStatus]
  );

  const handleServerDelete = useCallback(
    async (serverId: string) => {
      if (typeof window !== "undefined") {
        const confirmed = window.confirm(
          "Delete all local media metadata for this server? Watch history will be preserved."
        );
        if (!confirmed) {
          return;
        }
      }

      setDeletingServer(serverId);
      clearServerDeleteStatus(serverId);
      try {
        await deleteServerMedia(serverId);
        setServerDeleteStatus((prev) => ({ ...prev, [serverId]: "success" }));
        setTimeout(() => clearServerDeleteStatus(serverId), 4000);
      } catch (error) {
        console.error("Failed to delete server media:", error);
        setServerDeleteStatus((prev) => ({ ...prev, [serverId]: "error" }));
        setTimeout(() => clearServerDeleteStatus(serverId), 6000);
      } finally {
        setDeletingServer(null);
      }
    },
    [clearServerDeleteStatus]
  );

  useEffect(() => {
    if (meRole && meRole.toLowerCase() === "admin") {
      loadUsers();
    }
  }, [meRole, loadUsers]);

  useEffect(() => {
    if (!syncingServer) {
      return;
    }
    const entry = serverProgressMap[syncingServer];
    if (entry && !entry.running) {
      setSyncingServer(null);
      return;
    }
    if (!entry && typeof window !== "undefined") {
      const timer = window.setTimeout(() => setSyncingServer(null), 8000);
      return () => window.clearTimeout(timer);
    }
  }, [syncingServer, serverProgressMap]);

  if (isLoading) {
    return (
      <>
        <Head>
          <title>Settings - Emby Analytics</title>
          <meta name="viewport" content="initial-scale=1, width=device-width" />
        </Head>
        <div className="min-h-screen bg-neutral-900 text-white">
          <Header />
          <main className="p-4 md:p-6 border-t border-neutral-800">
            <div className="max-w-4xl mx-auto">
              <div className="flex items-center gap-3 mb-6">
                <Link
                  href="/"
                  className="flex items-center justify-center w-10 h-10 rounded-lg bg-neutral-700 hover:bg-neutral-600 transition-colors text-gray-300 hover:text-white"
                  title="Back to Dashboard"
                >
                  <ArrowLeft className="w-5 h-5" />
                </Link>
                <Settings className="w-6 h-6 text-gray-400" />
                <h1 className="text-2xl font-bold">Settings</h1>
              </div>
              <div className="bg-neutral-800 rounded-lg p-6">
                <div className="animate-pulse">
                  <div className="h-4 bg-neutral-700 rounded w-1/4 mb-4"></div>
                  <div className="h-10 bg-neutral-700 rounded w-full mb-4"></div>
                  <div className="h-3 bg-neutral-700 rounded w-3/4"></div>
                </div>
              </div>
            </div>
          </main>
        </div>
      </>
    );
  }

  if (error) {
    return (
      <>
        <Head>
          <title>Settings - Emby Analytics</title>
          <meta name="viewport" content="initial-scale=1, width=device-width" />
        </Head>
        <div className="min-h-screen bg-neutral-900 text-white">
          <Header />
          <main className="p-4 md:p-6 border-t border-neutral-800">
            <div className="max-w-4xl mx-auto">
              <div className="flex items-center gap-3 mb-6">
                <Link
                  href="/"
                  className="flex items-center justify-center w-10 h-10 rounded-lg bg-neutral-700 hover:bg-neutral-600 transition-colors text-gray-300 hover:text-white"
                  title="Back to Dashboard"
                >
                  <ArrowLeft className="w-5 h-5" />
                </Link>
                <Settings className="w-6 h-6 text-gray-400" />
                <h1 className="text-2xl font-bold">Settings</h1>
              </div>
              <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-6">
                <div className="flex items-center gap-3 text-red-400">
                  <AlertCircle className="w-5 h-5 text-red-400" />
                  <span>Failed to load settings: {error.message}</span>
                </div>
              </div>
            </div>
          </main>
        </div>
      </>
    );
  }

  return (
    <>
      <Head>
        <title>Settings - Emby Analytics</title>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white">
        <Header />
        <main className="p-4 md:p-6 border-t border-neutral-800">
          <div className="max-w-4xl mx-auto">
            <div className="flex items-center gap-3 mb-6">
              <Link
                href="/"
                className="flex items-center justify-center w-10 h-10 rounded-lg bg-neutral-700 hover:bg-neutral-600 transition-colors text-gray-300 hover:text-white"
                title="Back to Dashboard"
              >
                <ArrowLeft className="w-5 h-5" />
              </Link>
              <Settings className="w-6 h-6 text-gray-400" />
              <h1 className="text-2xl font-bold">Settings</h1>
            </div>

            <div className="bg-neutral-800 rounded-lg p-6">
              <h2 className="text-lg font-semibold mb-4">Watch Time Calculation</h2>

              <div className="space-y-4">
                <div className="flex items-center justify-between p-4 bg-neutral-700/50 rounded-lg">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <label htmlFor="include_trakt_items" className="text-white font-medium">
                        Include Trakt-synced items in watch time
                      </label>
                      {saveStatus?.key === "include_trakt_items" && (
                        <div
                          className={`flex items-center gap-1 text-sm ${
                            saveStatus.status === "success" ? "text-green-400" : "text-red-400"
                          }`}
                        >
                          {saveStatus.status === "success" ? (
                            <>
                              <Check className="w-4 h-4 text-green-400" />
                              <span>Saved</span>
                            </>
                          ) : (
                            <>
                              <AlertCircle className="w-4 h-4 text-red-400" />
                              <span>Error saving</span>
                            </>
                          )}
                        </div>
                      )}
                    </div>
                    <p className="text-gray-400 text-sm mb-3">
                      When enabled, items marked as &quot;played&quot; through Trakt sync will count
                      toward your total watch time. When disabled, only items actually watched
                      through Emby will be counted.
                    </p>
                    <div className="flex items-start gap-2 text-xs text-blue-300 bg-blue-900/20 border border-blue-500/30 rounded p-3">
                      <Info className="w-4 h-4 mt-0.5 flex-shrink-0 text-blue-400" />
                      <div>
                        <strong>How it works:</strong> Trakt-synced items have
                        &quot;Played=true&quot; but &quot;PlayCount=0&quot; in Emby, while actually
                        watched items have &quot;PlayCount &gt; 0&quot;. This setting lets you
                        choose whether to include the full runtime of Trakt-synced items in your
                        lifetime watch statistics.
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 ml-6">
                    {saving === "include_trakt_items" && (
                      <RotateCcw className="w-4 h-4 text-gray-400 animate-spin" />
                    )}
                    <button
                      id="include_trakt_items"
                      onClick={() => handleToggleChange("include_trakt_items", includeTrakt)}
                      disabled={saving === "include_trakt_items"}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 focus:ring-offset-neutral-900 ${
                        includeTrakt === "true" ? "bg-amber-600" : "bg-neutral-600"
                      } ${saving === "include_trakt_items" ? "opacity-50 cursor-not-allowed" : ""}`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          includeTrakt === "true" ? "translate-x-6" : "translate-x-1"
                        }`}
                      />
                    </button>
                  </div>
                </div>
              </div>

              <div className="mt-6 p-4 bg-neutral-700/30 rounded-lg border border-neutral-600">
                <h3 className="text-sm font-medium text-gray-300 mb-2">Note about changes</h3>
                <p className="text-sm text-gray-400">
                  Changes to this setting will take effect the next time user data is synced (every
                  12 hours by default, or when manually triggered via the admin panel). The new
                  calculation will be applied to all users.
                </p>
              </div>
            </div>

            <div className="bg-neutral-800 rounded-lg p-6 mt-6">
              <h2 className="text-lg font-semibold mb-4">Media Server Sync</h2>
              <p className="text-sm text-gray-400 mb-4">
                Enable or disable background sync for each configured media server. Disabling sync
                stops library and watch history imports for that server while keeping existing data.
                Use the <span className="text-white">Full Sync</span> action to run a complete
                library and playback rescan (not incremental) for the selected server.
              </p>
              <div className="space-y-3">
                {(orderedServers ?? servers)?.map((server) => {
                  const key = `sync_enabled_${server.id}`;
                  const currentSetting = settings?.find((s) => s.key === key)?.value;
                  const isEnabled =
                    (currentSetting ?? (server.enabled ? "true" : "false")) === "true";
                  const busy = saving === key;
                  const serverProgress = serverProgressMap[server.id];
                  const isServerRunning = Boolean(serverProgress?.running && !serverProgress.done);
                  const progressTotal = serverProgress?.total ?? 0;
                  const progressProcessed = serverProgress?.processed ?? 0;
                  const percent =
                    progressTotal > 0
                      ? Math.max(
                          0,
                          Math.min(100, Math.round((progressProcessed / progressTotal) * 100))
                        )
                      : 0;
                  const stageText =
                    serverProgress?.stage ||
                    (isServerRunning
                      ? "Full sync running..."
                      : serverProgress?.done
                        ? "Full sync complete"
                        : undefined);
                  const showProgressBar = Boolean(
                    serverProgress && (progressTotal > 0 || isServerRunning || serverProgress.done)
                  );
                  const reachable = server.health?.is_reachable;
                  const statusColor =
                    reachable === false
                      ? "bg-red-500"
                      : reachable === true
                        ? "bg-green-400"
                        : "bg-neutral-500";
                  const statusTitle =
                    reachable === false
                      ? server.health?.error
                        ? `Unreachable: ${server.health.error}`
                        : "Server unreachable"
                      : reachable === true
                        ? "Server reachable with current credentials"
                        : "Reachability unknown";
                  const disableSync = syncingServer === server.id || isServerRunning;

                  return (
                    <div
                      key={server.id}
                      className="p-4 bg-neutral-700/50 rounded-lg border border-neutral-700/70"
                    >
                      <div className="flex flex-col gap-3">
                        <div className="flex items-center justify-between gap-4">
                          <div>
                            <div className="flex items-center gap-2">
                              <span
                                className={`inline-flex h-2.5 w-2.5 rounded-full ${statusColor}`}
                                title={statusTitle}
                              />
                              <span className="text-white font-medium">{server.name}</span>
                              <span className="text-xs px-2 py-0.5 rounded-full border border-neutral-600 text-neutral-300 uppercase">
                                {server.type}
                              </span>
                            </div>
                            <div className="text-sm text-gray-400 mt-1">
                              Background sync {isEnabled ? "enabled" : "disabled"}
                              {server.health?.is_reachable === false && (
                                <span className="ml-2 text-red-400">
                                  ({server.health.error || "unreachable"})
                                </span>
                              )}
                              {serverSyncStatus[server.id] === "success" && (
                                <span className="ml-2 text-green-400">Full sync started</span>
                              )}
                              {serverSyncStatus[server.id] === "error" && (
                                <span className="ml-2 text-red-400">Failed to start sync</span>
                              )}
                              {serverDeleteStatus[server.id] === "success" && (
                                <span className="ml-2 text-amber-300">
                                  Media cleared. Run a full sync to repopulate.
                                </span>
                              )}
                              {serverDeleteStatus[server.id] === "error" && (
                                <span className="ml-2 text-red-400">Failed to clear media</span>
                              )}
                            </div>
                          </div>
                          <div className="flex items-center gap-3">
                            <button
                              onClick={() => handleServerSync(server.id)}
                              disabled={disableSync}
                              className={`inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors border border-neutral-600 ${
                                disableSync
                                  ? "bg-neutral-700 text-neutral-300 cursor-wait"
                                  : "bg-neutral-700 hover:bg-neutral-600 text-neutral-200"
                              }`}
                              title="Run a full sync for this server (library + playback history)"
                            >
                              {disableSync ? (
                                <RotateCcw className="w-4 h-4 animate-spin" />
                              ) : (
                                <RefreshCcw className="w-4 h-4" />
                              )}
                              <span>{disableSync ? "Syncing" : "Full Sync"}</span>
                            </button>
                            <button
                              onClick={() => handleServerDelete(server.id)}
                              disabled={deletingServer === server.id}
                              className={`inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors border ${
                                deletingServer === server.id
                                  ? "border-red-800 bg-red-900/60 text-red-200 cursor-wait"
                                  : "border-red-800 text-red-300 hover:bg-red-900/30"
                              }`}
                              title="Delete local media metadata for this server"
                            >
                              <Trash2 className="w-4 h-4" />
                              <span>
                                {deletingServer === server.id ? "Deleting..." : "Delete Media"}
                              </span>
                            </button>
                            {busy && <RotateCcw className="w-4 h-4 text-gray-400 animate-spin" />}
                            <button
                              onClick={() =>
                                handleToggleChange(
                                  key,
                                  currentSetting ?? (server.enabled ? "true" : "false")
                                )
                              }
                              disabled={busy}
                              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 focus:ring-offset-neutral-900 ${
                                isEnabled ? "bg-amber-600" : "bg-neutral-600"
                              } ${busy ? "opacity-50 cursor-not-allowed" : ""}`}
                            >
                              <span
                                className={`inline-block h-5 w-5 transform rounded-full bg-white transition-transform ${isEnabled ? "translate-x-5" : "translate-x-1"}`}
                              />
                            </button>
                          </div>
                        </div>
                        {showProgressBar && (
                          <div>
                            {stageText && (
                              <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
                                <span>{stageText}</span>
                                {progressTotal > 0 && (
                                  <span>
                                    {progressProcessed}/{progressTotal}
                                  </span>
                                )}
                              </div>
                            )}
                            {progressTotal > 0 ? (
                              <div className="h-1.5 rounded bg-neutral-700">
                                <div
                                  className="h-full rounded bg-amber-500 transition-all duration-300"
                                  style={{ width: `${Math.max(4, percent)}%` }}
                                />
                              </div>
                            ) : (
                              isServerRunning && (
                                <div className="h-1.5 rounded bg-neutral-700 overflow-hidden">
                                  <div className="h-full w-1/3 bg-amber-500 animate-pulse" />
                                </div>
                              )
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
                {!servers && <div className="text-sm text-gray-500">Loading servers…</div>}
                {servers && servers.length === 0 && (
                  <div className="text-sm text-gray-500">No media servers configured.</div>
                )}
              </div>
            </div>

            <div className="bg-neutral-800 rounded-lg p-6 mt-6">
              <h2 className="text-lg font-semibold mb-4">Performance Settings</h2>

              <div className="space-y-4">
                <div className="flex items-center justify-between p-4 bg-neutral-700/50 rounded-lg">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <label
                        htmlFor="prevent_4k_video_transcoding"
                        className="text-white font-medium"
                      >
                        Prevent 4K Video Transcoding
                      </label>
                      {saveStatus?.key === "prevent_4k_video_transcoding" && (
                        <div
                          className={`flex items-center gap-1 text-sm ${
                            saveStatus.status === "success" ? "text-green-400" : "text-red-400"
                          }`}
                        >
                          {saveStatus.status === "success" ? (
                            <>
                              <Check className="w-4 h-4 text-green-400" />
                              <span>Saved</span>
                            </>
                          ) : (
                            <>
                              <AlertCircle className="w-4 h-4 text-red-400" />
                              <span>Error saving</span>
                            </>
                          )}
                        </div>
                      )}
                    </div>
                    <p className="text-gray-400 text-sm mb-3">
                      Automatically stops sessions when 4K video transcoding is detected to prevent
                      server overload. Audio and subtitle transcoding on 4K content will continue
                      normally as they have minimal performance impact.
                    </p>
                    <div className="flex items-start gap-2 text-xs text-blue-300 bg-blue-900/20 border border-blue-500/30 rounded p-3">
                      <Info className="w-4 h-4 mt-0.5 flex-shrink-0 text-blue-400" />
                      <div>
                        <strong>How it works:</strong> This setting monitors active sessions and
                        automatically stops those transcoding 4K video content. Users will receive a
                        standard Emby &quot;session stopped&quot; notification. Only video
                        transcoding is blocked - audio conversion and subtitle burn-in continue to
                        work normally for better user experience.
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 ml-6">
                    {saving === "prevent_4k_video_transcoding" && (
                      <RotateCcw className="w-4 h-4 text-gray-400 animate-spin" />
                    )}
                    <button
                      id="prevent_4k_video_transcoding"
                      onClick={() =>
                        handleToggleChange("prevent_4k_video_transcoding", prevent4kTranscoding)
                      }
                      disabled={saving === "prevent_4k_video_transcoding"}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 focus:ring-offset-neutral-900 ${
                        prevent4kTranscoding === "true" ? "bg-amber-600" : "bg-neutral-600"
                      } ${saving === "prevent_4k_video_transcoding" ? "opacity-50 cursor-not-allowed" : ""}`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          prevent4kTranscoding === "true" ? "translate-x-6" : "translate-x-1"
                        }`}
                      />
                    </button>
                  </div>
                </div>
              </div>

              <div className="mt-6 p-4 bg-neutral-700/30 rounded-lg border border-neutral-600">
                <h3 className="text-sm font-medium text-gray-300 mb-2">Performance Impact</h3>
                <p className="text-sm text-gray-400">
                  4K video transcoding can consume significant CPU/GPU resources and impact server
                  performance for all users. This setting helps maintain server stability by
                  preventing the most resource-intensive transcoding operations while preserving
                  user experience for audio and subtitle features.
                </p>
            </div>
          </div>

          {shouldLoadRuntimeOutliers && (
            <div className="bg-neutral-800 rounded-lg p-6 mt-6">
              <div className="flex items-center justify-between gap-4 mb-4">
                <div>
                  <h2 className="text-lg font-semibold">Runtime Outliers</h2>
                  <p className="text-sm text-gray-400">
                    Items reporting runtimes longer than {runtimeOutlierThresholdMinutes} minutes
                    ({runtimeOutlierThresholdHours}h, {runtimeOutlierThresholdDays}d). These entries
                    usually mean Emby cached a bad runtime before the source file was fixed.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => void reloadRuntimeOutliers()}
                  disabled={runtimeOutliersLoading}
                  className={`inline-flex items-center gap-2 rounded-md border border-neutral-600 px-3 py-2 text-sm transition-colors ${
                    runtimeOutliersLoading
                      ? "bg-neutral-700/60 text-gray-400 cursor-wait"
                      : "bg-neutral-800 hover:bg-neutral-700 text-gray-200"
                  }`}
                  title="Refresh runtime outliers"
                >
                  <RefreshCcw
                    className={`w-4 h-4 ${runtimeOutliersLoading ? "animate-spin" : ""}`}
                  />
                  Refresh
                </button>
              </div>

              {runtimeOutliersLoading && (
                <div className="animate-pulse space-y-3">
                  <div className="h-4 bg-neutral-700 rounded w-1/3" />
                  <div className="h-4 bg-neutral-700 rounded w-1/2" />
                  <div className="h-32 bg-neutral-700/60 rounded" />
                </div>
              )}

              {!runtimeOutliersLoading && runtimeOutliersError && (
                <div className="flex items-center gap-2 text-red-400">
                  <AlertCircle className="w-4 h-4" />
                  <span>
                    {runtimeOutliersError.message || "Failed to load runtime outliers"}
                  </span>
                </div>
              )}

              {!runtimeOutliersLoading && !runtimeOutliersError && runtimeOutlierItems.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="text-left text-gray-300">
                        <th className="py-2 pr-4">Title</th>
                        <th className="py-2 pr-4">Runtime</th>
                        <th className="py-2 pr-4">Updated</th>
                        <th className="py-2 pr-4">Created</th>
                      </tr>
                    </thead>
                    <tbody>
                      {runtimeOutlierItems.map((item) => {
                        const isSelected = selectedOutlier?.library_id === item.library_id;
                        return (
                          <tr
                            key={item.library_id}
                            className={`border-t border-neutral-700/60 ${
                              isSelected ? "bg-neutral-700/30" : ""
                            }`}
                          >
                            <td className="py-2 pr-4 align-top">
                              <button
                                type="button"
                                onClick={() => setSelectedOutlier(item)}
                                className="text-left w-full group"
                              >
                                <div className="flex items-center gap-2">
                                  <span className="text-white font-medium group-hover:text-amber-300">
                                    {item.name || "Unknown"}
                                  </span>
                                  <span className="text-xs text-amber-400 hidden group-hover:inline">
                                    View details
                                  </span>
                                </div>
                                <div className="mt-1 space-y-1 text-xs text-gray-400">
                                  {(item.server_type || item.server_id) && (
                                    <div>
                                      {(item.server_type && item.server_type.length > 0 && (
                                        <span className="uppercase tracking-wide text-gray-300">
                                          {item.server_type}
                                        </span>
                                      )) || null}
                                      {item.server_id && (
                                        <span className="ml-2 text-gray-500">{item.server_id}</span>
                                      )}
                                    </div>
                                  )}
                                  <div className="text-gray-500">
                                    Library ID: <code className="text-gray-300">{item.library_id}</code>
                                  </div>
                                  {item.item_id && (
                                    <div className="text-gray-500">
                                      Server Item: <code className="text-gray-300">{item.item_id}</code>
                                    </div>
                                  )}
                                  <div className="text-gray-500">
                                    Stored ticks: <code className="text-gray-300">{item.runtime_ticks}</code>
                                  </div>
                                </div>
                              </button>
                            </td>
                            <td className="py-2 pr-4 align-top text-gray-200">
                              <div className="font-semibold">{item.runtime_hours}</div>
                              <div className="text-xs text-gray-500">{item.runtime_minutes} minutes</div>
                            </td>
                            <td className="py-2 pr-4 align-top text-gray-400">
                              {item.updated_at || "—"}
                            </td>
                            <td className="py-2 pr-4 align-top text-gray-400">
                              {item.created_at || "—"}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}

              {!runtimeOutliersLoading && !runtimeOutliersError && runtimeOutlierItems.length === 0 && (
                <div className="text-sm text-gray-400">
                  All stored runtimes are below the current threshold. Newly corrected titles will
                  drop off this list after the next library sync.
                </div>
              )}

              {runtimeOutliers?.has_more && !runtimeOutliersLoading && !runtimeOutliersError && (
                <p className="text-xs text-amber-300 mt-3">
                  Showing the first {runtimeOutlierItems.length} entries. Use the admin API to
                  inspect the full list if needed.
                </p>
              )}

              {selectedOutlier && (
                <div className="mt-4 bg-neutral-900/40 border border-neutral-700 rounded-md p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h3 className="text-sm font-semibold text-white">
                        {selectedOutlier.name || "Unknown title"}
                      </h3>
                      <p className="text-xs text-gray-400 mt-1">
                        Library ID: <code className="text-gray-300">{selectedOutlier.library_id}</code>
                        {selectedOutlier.item_id && (
                          <>
                            <span className="mx-2">•</span>Server Item ID:{" "}
                            <code className="text-gray-300">{selectedOutlier.item_id}</code>
                          </>
                        )}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => setSelectedOutlier(null)}
                      className="text-xs text-gray-400 hover:text-white"
                    >
                      Close
                    </button>
                  </div>

                  <div className="mt-3 text-sm text-gray-300">
                    <div>
                      Reported runtime: <strong>{selectedOutlier.runtime_hours}</strong> (
                      {selectedOutlier.runtime_minutes} minutes)
                    </div>
                    <div className="text-xs text-gray-500 mt-1">
                      Updated at {selectedOutlier.updated_at || "—"} • Created at {" "}
                      {selectedOutlier.created_at || "—"}
                    </div>
                  </div>

                  {selectedOutlierDetailsLoading && (
                    <div className="mt-3 text-xs text-gray-400">Loading source metadata…</div>
                  )}
                  {selectedOutlierDetailsError && (
                    <div className="mt-3 text-xs text-red-400">
                      Failed to load item metadata. It may have been removed from the source
                      server.
                    </div>
                  )}
                  {!selectedOutlierDetailsLoading && !selectedOutlierDetailsError && (
                    <div className="mt-3 text-xs text-gray-300 space-y-1">
                      {selectedOutlierDetails && selectedOutlierDetails.length > 0 ? (
                        <>
                          <div>
                            <span className="text-gray-400">Analytics record:</span>{" "}
                            {selectedOutlierDetails[0].name || "(no name cached)"}
                          </div>
                          {selectedOutlierDetails[0].type && (
                            <div className="text-gray-400">
                              Media type: {selectedOutlierDetails[0].type}
                            </div>
                          )}
                        </>
                      ) : (
                        <div>
                          No analytics metadata is currently cached for this server item. This
                          usually means the title was removed from Emby without running the missing
                          item cleanup job yet.
                        </div>
                      )}
                    </div>
                  )}

                  <div className="mt-4 flex flex-wrap gap-2 text-xs">
                    {selectedOutlier.item_id && (
                      <a
                        href={`/items/by-ids?ids=${encodeURIComponent(selectedOutlier.item_id)}`}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 px-3 py-1 rounded-md border border-neutral-700 text-gray-200 hover:border-amber-500 hover:text-amber-200"
                      >
                        View analytics JSON
                      </a>
                    )}
                    <button
                      type="button"
                      onClick={() => {
                        void navigator.clipboard?.writeText(selectedOutlier.library_id);
                      }}
                      className="inline-flex items-center gap-1 px-3 py-1 rounded-md border border-neutral-700 text-gray-200 hover:border-amber-500 hover:text-amber-200"
                    >
                      Copy library ID
                    </button>
                    {selectedOutlier.item_id && (
                      <button
                        type="button"
                        onClick={() => void navigator.clipboard?.writeText(selectedOutlier.item_id)}
                        className="inline-flex items-center gap-1 px-3 py-1 rounded-md border border-neutral-700 text-gray-200 hover:border-amber-500 hover:text-amber-200"
                      >
                        Copy server item ID
                      </button>
                    )}
                  </div>
                </div>
              )}

              <div className="mt-4 text-xs text-gray-500 bg-neutral-900/40 border border-neutral-700 rounded-md p-3">
                Entries with unrealistic runtimes are automatically reset the next time they are
                encountered. If you already refreshed a title in Emby, run a library sync to
                repopulate its accurate runtime or use the missing-items cleanup job to remove stale
                library rows.
              </div>
            </div>
          )}

          {meRole && meRole.toLowerCase() === "admin" && (
            <div className="bg-neutral-800 rounded-lg p-6 mt-6">
              <div className="flex items-center gap-2 mb-4">
                  <User className="w-5 h-5 text-gray-400" />
                  <h2 className="text-lg font-semibold">Users</h2>
                </div>

                {/* Create new user */}
                <div className="p-4 bg-neutral-700/40 rounded-lg border border-neutral-600 mb-4">
                  <div className="flex flex-col md:flex-row gap-3 items-stretch md:items-end">
                    <div className="flex-1">
                      <label className="block text-sm text-gray-300 mb-1">Username</label>
                      <input
                        className="w-full px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                        value={newUser.username}
                        onChange={(e) => setNewUser((s) => ({ ...s, username: e.target.value }))}
                        placeholder="new username"
                      />
                    </div>
                    <div className="flex-1">
                      <label className="block text-sm text-gray-300 mb-1">Password</label>
                      <input
                        type="password"
                        className="w-full px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                        value={newUser.password}
                        onChange={(e) => setNewUser((s) => ({ ...s, password: e.target.value }))}
                        placeholder="set password"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-300 mb-1">Role</label>
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          className={`px-3 py-2 rounded-md border ${newUser.role === "user" ? "bg-neutral-700 border-neutral-600" : "bg-neutral-800 border-neutral-700"}`}
                          onClick={() => setNewUser((s) => ({ ...s, role: "user" }))}
                          title="Standard user"
                        >
                          User
                        </button>
                        <button
                          type="button"
                          className={`px-3 py-2 rounded-md border flex items-center gap-1 ${newUser.role === "admin" ? "bg-amber-700/50 border-amber-600 text-amber-200" : "bg-neutral-800 border-neutral-700"}`}
                          onClick={() => setNewUser((s) => ({ ...s, role: "admin" }))}
                          title="Administrator"
                        >
                          <Shield className="w-4 h-4" /> Admin
                        </button>
                      </div>
                    </div>
                    <div>
                      <button
                        disabled={busyUsers || !newUser.username || !newUser.password}
                        onClick={async () => {
                          try {
                            setBusyUsers(true);
                            await createAppUser(
                              newUser.username.trim(),
                              newUser.password,
                              newUser.role
                            );
                            setNewUser({ username: "", password: "", role: "user" });
                            await loadUsers();
                          } catch (e: unknown) {
                            alert(getErrMessage(e) || "Failed to create user");
                          } finally {
                            setBusyUsers(false);
                          }
                        }}
                        className="bg-amber-600 hover:bg-amber-500 text-black font-semibold px-4 py-2 rounded-md flex items-center gap-2 disabled:opacity-50"
                      >
                        <UserPlus className="w-4 h-4" /> Create
                      </button>
                    </div>
                  </div>
                </div>

                {/* Users table */}
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className="text-left text-gray-300">
                        <th className="py-2 pr-4">Username</th>
                        <th className="py-2 pr-4">Role</th>
                        <th className="py-2 pr-4">Created</th>
                        <th className="py-2 pr-2 text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {usersError && (
                        <tr>
                          <td colSpan={4} className="text-red-400 py-2">
                            {usersError}
                          </td>
                        </tr>
                      )}
                      {!usersError && (!users || users.length === 0) && (
                        <tr>
                          <td colSpan={4} className="text-gray-400 py-2">
                            No users
                          </td>
                        </tr>
                      )}
                      {users?.map((u) => (
                        <tr key={u.id} className="border-t border-neutral-700/60">
                          <td className="py-2 pr-4 align-middle">
                            {editingId === u.id ? (
                              <input
                                className="w-full px-2 py-1 rounded bg-neutral-900 border border-neutral-700"
                                defaultValue={u.username}
                                onChange={(e) =>
                                  setEditDraft((s) => ({ ...s, username: e.target.value }))
                                }
                              />
                            ) : (
                              <span className="text-white">{u.username}</span>
                            )}
                          </td>
                          <td className="py-2 pr-4 align-middle">
                            {editingId === u.id ? (
                              <select
                                defaultValue={u.role}
                                onChange={(e) =>
                                  setEditDraft((s) => ({
                                    ...s,
                                    role: e.target.value as "admin" | "user",
                                  }))
                                }
                                className="px-2 py-1 rounded bg-neutral-900 border border-neutral-700"
                              >
                                <option value="user">user</option>
                                <option value="admin">admin</option>
                              </select>
                            ) : (
                              <span
                                className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs ${u.role === "admin" ? "bg-amber-700/40 text-amber-200" : "bg-neutral-700/60 text-gray-200"}`}
                              >
                                {u.role === "admin" && <Shield className="w-3 h-3" />} {u.role}
                              </span>
                            )}
                          </td>
                          <td className="py-2 pr-4 align-middle text-gray-400">
                            {u.created_at?.replace(".000Z", "Z") || ""}
                          </td>
                          <td className="py-2 pr-2 align-middle">
                            <div className="flex items-center justify-end gap-2">
                              {editingId === u.id ? (
                                <>
                                  <button
                                    className="px-2 py-1 rounded bg-green-700/70 hover:bg-green-700 text-white flex items-center gap-1"
                                    onClick={async () => {
                                      try {
                                        setBusyUsers(true);
                                        await updateAppUser(u.id, editDraft);
                                        setEditingId(null);
                                        setEditDraft({});
                                        await loadUsers();
                                      } catch (e: unknown) {
                                        alert(getErrMessage(e) || "Save failed");
                                      } finally {
                                        setBusyUsers(false);
                                      }
                                    }}
                                  >
                                    <Save className="w-4 h-4" /> Save
                                  </button>
                                  <button
                                    className="px-2 py-1 rounded bg-neutral-700 hover:bg-neutral-600 text-white flex items-center gap-1"
                                    onClick={() => {
                                      setEditingId(null);
                                      setEditDraft({});
                                    }}
                                  >
                                    <X className="w-4 h-4" /> Cancel
                                  </button>
                                </>
                              ) : (
                                <>
                                  <button
                                    className="px-2 py-1 rounded bg-neutral-700 hover:bg-neutral-600 text-white flex items-center gap-1"
                                    onClick={() => {
                                      setEditingId(u.id);
                                      setEditDraft({});
                                    }}
                                  >
                                    <Pencil className="w-4 h-4" /> Edit
                                  </button>
                                  <button
                                    className="px-2 py-1 rounded bg-red-700/70 hover:bg-red-700 text-white flex items-center gap-1"
                                    onClick={async () => {
                                      if (!confirm(`Delete user ${u.username}?`)) return;
                                      try {
                                        setBusyUsers(true);
                                        await deleteAppUser(u.id);
                                        await loadUsers();
                                      } catch (e: unknown) {
                                        alert(getErrMessage(e) || "Delete failed");
                                      } finally {
                                        setBusyUsers(false);
                                      }
                                    }}
                                  >
                                    <Trash2 className="w-4 h-4" /> Delete
                                  </button>
                                </>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Inline password reset when editing */}
                {editingId && (
                  <div className="mt-4 p-3 bg-neutral-700/30 border border-neutral-600 rounded">
                    <label className="block text-sm text-gray-300 mb-1">Set new password</label>
                    <div className="flex gap-2">
                      <input
                        type="password"
                        className="flex-1 px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                        placeholder="leave blank to keep current password"
                        onChange={(e) => setEditDraft((s) => ({ ...s, password: e.target.value }))}
                      />
                    </div>
                  </div>
                )}

                {busyUsers && (
                  <div className="mt-3 text-sm text-gray-400 flex items-center gap-2">
                    <RotateCcw className="w-4 h-4 animate-spin" /> Working…
                  </div>
                )}
              </div>
            )}
          </div>
        </main>
      </div>
    </>
  );
}
