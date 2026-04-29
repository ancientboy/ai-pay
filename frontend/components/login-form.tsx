"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLocale } from "@/components/locale-provider";
import { ApiClientError, toReadableError } from "@/lib/error-map";

export function LoginForm({ next = "/dashboard" }: { next?: string }) {
  const router = useRouter();
  const { t, locale } = useLocale();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("admin123");
  const [regUsername, setRegUsername] = useState("");
  const [regPassword, setRegPassword] = useState("");
  const [error, setError] = useState("");
  const [registerMessage, setRegisterMessage] = useState("");
  const [loading, setLoading] = useState(false);
  const [registering, setRegistering] = useState(false);

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
              if (payload.code) {
                setError(toReadableError(new ApiClientError(payload.code, payload.message ?? ""), locale));
              } else {
                setError(payload.message || t("login.failed"));
              }
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
            autoComplete="username"
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="mt-3 block text-sm text-slate-300">
          {t("login.password")}
          <input
            value={password}
            type="password"
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
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
        <div className="mt-6 border-t border-slate-800 pt-4">
          <p className="text-sm font-medium text-slate-200">
            {locale === "en-US" ? "Register a user account" : "注册用户账户"}
          </p>
          <div className="mt-2 space-y-2">
            <input
              value={regUsername}
              onChange={(e) => setRegUsername(e.target.value)}
              autoComplete="username"
              placeholder={locale === "en-US" ? "New username" : "新用户名"}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
            <input
              value={regPassword}
              onChange={(e) => setRegPassword(e.target.value)}
              type="password"
              autoComplete="new-password"
              placeholder={locale === "en-US" ? "New password (>=6 chars)" : "新密码（至少6位）"}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
            <button
              type="button"
              disabled={registering}
              onClick={async () => {
                setRegistering(true);
                setRegisterMessage("");
                setError("");
                try {
                  const response = await fetch("/api/auth/register", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ username: regUsername, password: regPassword }),
                  });
                  const payload = (await response.json()) as { code?: string; message?: string };
                  if (!response.ok || payload.code !== "0") {
                    if (payload.code) {
                      setError(toReadableError(new ApiClientError(payload.code, payload.message ?? ""), locale));
                    } else {
                      setError(payload.message || (locale === "en-US" ? "Register failed" : "注册失败"));
                    }
                    return;
                  }
                  setRegisterMessage(
                    locale === "en-US"
                      ? `Registered: ${regUsername}. You can log in now.`
                      : `注册成功：${regUsername}，现在可直接登录。`,
                  );
                  setUsername(regUsername);
                  setPassword(regPassword);
                } catch {
                  setError(locale === "en-US" ? "Register failed" : "注册失败");
                } finally {
                  setRegistering(false);
                }
              }}
              className="w-full rounded-md border border-slate-700 px-3 py-2 text-sm font-medium text-slate-200 hover:bg-slate-800 disabled:opacity-60"
            >
              {registering
                ? (locale === "en-US" ? "Registering..." : "注册中...")
                : (locale === "en-US" ? "Register" : "注册")}
            </button>
            {registerMessage ? <p className="text-xs text-emerald-300">{registerMessage}</p> : null}
          </div>
        </div>
      </form>
    </div>
  );
}
