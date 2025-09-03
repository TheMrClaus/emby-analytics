import React, { useState } from 'react';
import Head from 'next/head';
import Header from '../components/Header';
import { useSettings } from '../hooks/useSettings';
import SettingsIcon from '@mui/icons-material/Settings';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import InfoIcon from '@mui/icons-material/Info';
import RefreshIcon from '@mui/icons-material/Refresh';

export default function SettingsPage() {
  const { data: settings, error, isLoading, updateSetting } = useSettings();
  const [saving, setSaving] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<{ key: string; status: 'success' | 'error' } | null>(null);

  const handleToggleChange = async (key: string, currentValue: string) => {
    const newValue = currentValue === 'true' ? 'false' : 'true';
    setSaving(key);
    setSaveStatus(null);

    try {
      await updateSetting(key, newValue);
      setSaveStatus({ key, status: 'success' });
      setTimeout(() => setSaveStatus(null), 3000);
    } catch (error) {
      console.error('Failed to update setting:', error);
      setSaveStatus({ key, status: 'error' });
      setTimeout(() => setSaveStatus(null), 5000);
    } finally {
      setSaving(null);
    }
  };

  const includeTrakt = settings?.find(s => s.key === 'include_trakt_items')?.value || 'false';

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
                <SettingsIcon className="w-6 h-6 text-gray-400" />
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
                <SettingsIcon className="w-6 h-6 text-gray-400" />
                <h1 className="text-2xl font-bold">Settings</h1>
              </div>
              <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-6">
                <div className="flex items-center gap-3 text-red-400">
                  <ErrorIcon className="w-5 h-5 text-red-400" />
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
              <span className="text-2xl">⚙️</span>
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
                      {saveStatus?.key === 'include_trakt_items' && (
                        <div className={`flex items-center gap-1 text-sm ${
                          saveStatus.status === 'success' ? 'text-green-400' : 'text-red-400'
                        }`}>
                          {saveStatus.status === 'success' ? (
                            <>
                              <CheckCircleIcon className="w-4 h-4 text-green-400" />
                              <span>Saved</span>
                            </>
                          ) : (
                            <>
                              <ErrorIcon className="w-4 h-4 text-red-400" />
                              <span>Error saving</span>
                            </>
                          )}
                        </div>
                      )}
                    </div>
                    <p className="text-gray-400 text-sm mb-3">
                      When enabled, items marked as "played" through Trakt sync will count toward your total watch time. 
                      When disabled, only items actually watched through Emby will be counted.
                    </p>
                    <div className="flex items-start gap-2 text-xs text-blue-300 bg-blue-900/20 border border-blue-500/30 rounded p-3">
                      <InfoIcon className="w-4 h-4 mt-0.5 flex-shrink-0 text-blue-400" />
                      <div>
                        <strong>How it works:</strong> Trakt-synced items have "Played=true" but "PlayCount=0" in Emby, 
                        while actually watched items have "PlayCount &gt; 0". This setting lets you choose whether 
                        to include the full runtime of Trakt-synced items in your lifetime watch statistics.
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex items-center gap-3 ml-6">
                    {saving === 'include_trakt_items' && (
                      <RefreshIcon className="w-4 h-4 text-gray-400 animate-spin" />
                    )}
                    <button
                      id="include_trakt_items"
                      onClick={() => handleToggleChange('include_trakt_items', includeTrakt)}
                      disabled={saving === 'include_trakt_items'}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 focus:ring-offset-neutral-900 ${
                        includeTrakt === 'true' 
                          ? 'bg-amber-600' 
                          : 'bg-neutral-600'
                      } ${saving === 'include_trakt_items' ? 'opacity-50 cursor-not-allowed' : ''}`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          includeTrakt === 'true' ? 'translate-x-6' : 'translate-x-1'
                        }`}
                      />
                    </button>
                  </div>
                </div>
              </div>

              <div className="mt-6 p-4 bg-neutral-700/30 rounded-lg border border-neutral-600">
                <h3 className="text-sm font-medium text-gray-300 mb-2">Note about changes</h3>
                <p className="text-sm text-gray-400">
                  Changes to this setting will take effect the next time user data is synced (every 12 hours by default, 
                  or when manually triggered via the admin panel). The new calculation will be applied to all users.
                </p>
              </div>
            </div>
          </div>
        </main>
      </div>
    </>
  );
}