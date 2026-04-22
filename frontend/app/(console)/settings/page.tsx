"use client";

import { useMemo, useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { getSavedApiBaseURL, saveApiBaseURL } from "@/lib/console-api";

export default function SettingsPage() {
  const { t } = useLocale();
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
        <h2 className="text-xl font-semibold">{t("settings.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("settings.subtitle")}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("settings.apiEndpoint")}</h3>
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
                  showToast("success", t("settings.saved"));
                }}
                className="rounded-md bg-blue-600 px-3 py-1 text-xs text-white"
              >
                {t("settings.save")}
              </button>
              <button
                onClick={() => {
                  saveApiBaseURL("");
                  setApiBaseURL(envBaseURL);
                  showToast("info", t("settings.reverted"));
                }}
                className="rounded-md border border-slate-700 px-3 py-1 text-xs text-slate-200"
              >
                {t("settings.useEnv")}
              </button>
            </div>
            <p className="text-xs text-slate-500">
              {t("settings.currentMode")}: {usingRuntimeOverride ? t("settings.runtimeOverride") : t("settings.envDefault")}
            </p>
          </div>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("settings.securityNotes")}</h3>
          <ul className="mt-2 list-disc space-y-1 pl-4 text-sm text-slate-400">
            <li>{t("settings.notes.idempotency")}</li>
            <li>{t("settings.notes.timestamp")}</li>
            <li>{t("settings.notes.requestId")}</li>
          </ul>
        </div>
      </div>
    </section>
  );
}
