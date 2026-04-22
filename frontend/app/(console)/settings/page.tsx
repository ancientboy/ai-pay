"use client";

import { useMemo, useState } from "react";
import { useToast } from "@/components/toast-provider";
import { getSavedApiBaseURL, saveApiBaseURL } from "@/lib/console-api";

export default function SettingsPage() {
  const { showToast } = useToast();
  const envBaseURL =
    process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
  const [apiBaseURL, setApiBaseURL] = useState(() => {
    const saved = getSavedApiBaseURL();
    return saved || envBaseURL;
  });

  const usingRuntimeOverride = useMemo(
    () => apiBaseURL.trim() !== "" && apiBaseURL.trim() !== envBaseURL,
    [apiBaseURL, envBaseURL],
  );

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Settings</h2>
        <p className="mt-1 text-sm text-slate-400">
          Configure API endpoint and security preferences.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">API Endpoint</h3>
          <p className="mt-2 text-sm text-slate-400">
            NEXT_PUBLIC_API_BASE_URL
          </p>
          <div className="mt-1 space-y-2">
            <input
              value={apiBaseURL}
              onChange={(e) => setApiBaseURL(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-300"
            />
            <div className="flex gap-2">
              <button
                onClick={() => {
                  saveApiBaseURL(apiBaseURL);
                  showToast("success", "API endpoint saved");
                }}
                className="rounded-md bg-blue-600 px-3 py-1 text-xs text-white"
              >
                Save
              </button>
              <button
                onClick={() => {
                  saveApiBaseURL("");
                  setApiBaseURL(envBaseURL);
                  showToast("info", "Reverted to environment endpoint");
                }}
                className="rounded-md border border-slate-700 px-3 py-1 text-xs text-slate-200"
              >
                Use Env Default
              </button>
            </div>
            <p className="text-xs text-slate-500">
              Current mode: {usingRuntimeOverride ? "Runtime Override" : "Env Default"}
            </p>
          </div>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Security Notes</h3>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-sm text-slate-400">
            <li>Always pass Idempotency-Key for payment requests.</li>
            <li>Sign requests with timestamp within 5 minutes.</li>
            <li>Trace failures by requestId in transaction details.</li>
          </ul>
        </div>
      </div>
    </section>
  );
}
