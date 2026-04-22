"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export function LoginForm({ next = "/dashboard" }: { next?: string }) {
  const router = useRouter();
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
              setError(payload.message || "登录失败");
              return;
            }
            router.replace(next);
          } catch {
            setError("登录失败，请重试");
          } finally {
            setLoading(false);
          }
        }}
      >
        <h1 className="text-xl font-semibold text-slate-100">AI Pay 登录</h1>
        <p className="mt-1 text-sm text-slate-400">
          MVP 测试登录，可后续替换为真实认证
        </p>
        <label className="mt-4 block text-sm text-slate-300">
          Username
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="mt-3 block text-sm text-slate-300">
          Password
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
          {loading ? "登录中..." : "登录"}
        </button>
      </form>
    </div>
  );
}
