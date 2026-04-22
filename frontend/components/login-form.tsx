"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale } from "@/components/locale-provider";

export function LoginForm({ next = "/dashboard" }: { next?: string }) {
  const router = useRouter();
  const { t } = useLocale();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin123");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 p-4">
      <form
        className="w-full max-w-md rounded-xl border border-slate-800 bg-slate-900 p-6"
        onSubmit={async (e) => {
          e.preventDefault();
          setLoading(true);
          setError("");
          try {
            const response = await fetch("/api/auth/login", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ username, password }),
            });
            const payload = (await response.json()) as {
              code?: string;
              message?: string;
            };
            if (!response.ok || payload.code !== "0") {
              setError(payload.message || t("login.failed"));
              return;
            }
            router.replace(next);
          } catch {
            setError(t("login.failed"));
          } finally {
            setLoading(false);
          }
        }}
      >
        <h1 className="text-xl font-semibold text-slate-100">{t("login.title")}</h1>
        <p className="mt-1 text-sm text-slate-400">{t("login.subtitle")}</p>
        <label className="mt-4 block text-sm text-slate-300">
          {t("login.username")}
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="mt-3 block text-sm text-slate-300">
          {t("login.password")}
          <input
            value={password}
            type="password"
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        {error ? <p className="mt-3 text-sm text-rose-400">{error}</p> : null}
        <button
          disabled={loading}
          className="mt-5 w-full rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
        >
          {loading ? t("login.submitting") : t("login.submit")}
        </button>
      </form>
    </div>
  );
}
