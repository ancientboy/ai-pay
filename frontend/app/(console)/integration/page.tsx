"use client";

import Link from "next/link";
import { useLocale } from "@/components/locale-provider";

export default function IntegrationPage() {
  const { t } = useLocale();

  return (
    <section className="space-y-8">
      <div>
        <h2 className="text-xl font-semibold">{t("integration.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("integration.subtitle")}</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-5">
        <h3 className="text-sm font-medium text-slate-200">{t("integration.contractTitle")}</h3>
        <p className="mt-2 text-sm text-slate-400">{t("integration.contractDesc")}</p>
        <p className="mt-3 font-mono text-xs text-slate-500">openapi.yaml</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-5">
        <h3 className="text-sm font-medium text-slate-200">{t("integration.docTitle")}</h3>
        <p className="mt-2 text-sm text-slate-400">{t("integration.docDesc")}</p>
        <p className="mt-3 font-mono text-xs text-slate-500">docs/API_INTEGRATION.md</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-5">
        <h3 className="text-sm font-medium text-slate-200">{t("integration.exampleTitle")}</h3>
        <p className="mt-2 text-sm text-slate-400">{t("integration.exampleDesc")}</p>
        <pre className="mt-4 overflow-x-auto rounded-lg border border-slate-800 bg-slate-950 p-4 font-mono text-xs text-slate-300">
          {t("integration.exampleCommands")}
        </pre>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-5">
        <h3 className="text-sm font-medium text-slate-200">{t("integration.consoleProxyTitle")}</h3>
        <p className="mt-2 text-sm text-slate-400">{t("integration.consoleProxyDesc")}</p>
        <p className="mt-3 font-mono text-xs text-slate-500">/api/backend/*</p>
      </div>

      <div className="flex flex-wrap gap-3">
        <Link
          href="/agents"
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
        >
          {t("integration.linkAgents")}
        </Link>
        <Link
          href="/developer"
          className="rounded-md border border-slate-600 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
        >
          {t("integration.linkDeveloper")}
        </Link>
        <Link
          href="/settings"
          className="rounded-md border border-slate-600 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
        >
          {t("integration.linkSettings")}
        </Link>
      </div>
    </section>
  );
}
