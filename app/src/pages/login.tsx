import React, { useEffect, useState } from "react";
import Head from "next/head";
import Link from "next/link";
import { useRouter } from "next/router";

type AuthConfig = {
  enabled: boolean;
  registration_mode: "closed" | "secret" | "open" | string;
  registration_open: boolean;
  requires_secret: boolean;
};

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [invite, setInvite] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [cfg, setCfg] = useState<AuthConfig | null>(null);

  useEffect(() => {
    // Already authenticated? bounce
    (async () => {
      try {
        const res = await fetch("/auth/me", { credentials: "include" });
        if (res.ok) {
          router.replace("/");
        }
      } catch (e) {
        setServerError(getErrorMessage(e) || null);
      }
    })();
  }, [router]);

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch("/auth/config", { credentials: "include" });
        if (res.ok) {
          const j = (await res.json()) as AuthConfig;
          setCfg(j);
        }
      } catch {}
    })();
  }, []);

  const handleLogin = async (e?: React.FormEvent) => {
    e?.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const res = await fetch("/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username, password }),
      });
      if (!res.ok) {
        const msg = await readError(res);
        setError(msg || "Invalid username or password");
        return;
      }
      const next = (router.query?.next as string) || "/";
      router.replace(next);
    } catch (err: unknown) {
      setError(getErrorMessage(err) || "Login failed");
    } finally {
      setBusy(false);
    }
  };

  const handleCreate = async () => {
    setError(null);
    setBusy(true);
    try {
      const payload: Record<string, string> = { username, password };
      if (invite.trim()) payload.secret = invite.trim();
      const res = await fetch("/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const msg = await readError(res);
        setError(msg || "Registration failed");
        return;
      }
      const next = (router.query?.next as string) || "/";
      router.replace(next);
    } catch (err: unknown) {
      setError(getErrorMessage(err) || "Failed to create account");
    } finally {
      setBusy(false);
    }
  };

  function getErrorMessage(e: unknown): string {
    if (typeof e === "string") return e;
    if (e && typeof e === "object") {
      const maybe = (e as { message?: unknown }).message;
      if (typeof maybe === "string") return maybe;
    }
    return "";
  }

  async function readError(res: Response): Promise<string> {
    try {
      const text = await res.text();
      try {
        const j = JSON.parse(text);
        if (j && typeof j.error === "string") return j.error;
        return text;
      } catch {
        return text;
      }
    } catch {
      return "";
    }
  }

  return (
    <>
      <Head>
        <title>Login - Emby Analytics</title>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white flex items-center justify-center p-4">
        <div className="w-full max-w-md bg-neutral-800 border border-neutral-700 rounded-xl p-6 shadow-lg">
          <div className="mb-6 text-center">
            <h1 className="text-2xl font-bold">Emby Analytics</h1>
            <p className="text-sm text-gray-400 mt-1">Sign in or create an account</p>
          </div>

          <form onSubmit={handleLogin} className="space-y-4">
            <div>
              <label htmlFor="username" className="block text-sm text-gray-300 mb-1">
                Username
              </label>
              <input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                placeholder="Enter username"
                autoComplete="username"
                disabled={busy}
                required
              />
            </div>
            <div>
              <label htmlFor="password" className="block text-sm text-gray-300 mb-1">
                Password
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                placeholder="Enter password"
                autoComplete="current-password"
                disabled={busy}
                required
              />
            </div>

            {cfg?.requires_secret && (
              <div>
                <label htmlFor="invite" className="block text-sm text-gray-300 mb-1">
                  Invite code
                </label>
                <input
                  id="invite"
                  type="text"
                  value={invite}
                  onChange={(e) => setInvite(e.target.value)}
                  className="w-full px-3 py-2 rounded-md bg-neutral-900 border border-neutral-700 focus:outline-none focus:ring-2 focus:ring-amber-500"
                  placeholder="Enter invite/registration code"
                  disabled={busy}
                  required
                />
                <p className="text-xs text-gray-400 mt-1">Registration requires a valid invite code.</p>
              </div>
            )}

            {error && (
              <div className="text-red-400 text-sm bg-red-900/20 border border-red-600/30 rounded p-2">
                {error}
              </div>
            )}
            {serverError && (
              <div className="text-yellow-400 text-xs bg-yellow-900/20 border border-yellow-600/30 rounded p-2">
                {serverError}
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={busy}
                className="flex-1 bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-black font-semibold px-4 py-2 rounded-md"
              >
                {busy ? "Working…" : "Login"}
              </button>
              <button
                type="button"
                onClick={handleCreate}
                disabled={busy || !cfg?.registration_open}
                className="flex-1 bg-neutral-700 hover:bg-neutral-600 disabled:opacity-50 text-white font-semibold px-4 py-2 rounded-md border border-neutral-600"
                title={cfg?.registration_open ? "Create a new local account" : "Registration is closed"}
              >
                Create Account
              </button>
            </div>

            {!cfg?.registration_open && (
              <p className="text-xs text-gray-400">
                Registration is currently closed. Ask an admin to enable invites or create your account.
              </p>
            )}
          </form>

          <div className="mt-4 text-center">
            <Link href="/" className="text-blue-300 hover:text-white underline decoration-dotted text-sm">
              Back to dashboard
            </Link>
          </div>
        </div>
      </div>
    </>
  );
}
