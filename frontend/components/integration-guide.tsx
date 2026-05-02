"use client";

import Link from "next/link";
import { useLocale } from "@/components/locale-provider";

type Variant = "console" | "public";

function loginNext(path: string) {
  return `/login?next=${encodeURIComponent(path)}`;
}

export function IntegrationGuide({ variant }: { variant: Variant }) {
  const { t } = useLocale();

  const stepCards = [
    {
      title: t("integration.step1Title"),
      body: t("integration.step1Body"),
    },
    {
      title: t("integration.step2Title"),
      body: t("integration.step2Body"),
    },
    {
      title: t("integration.step3Title"),
      body: t("integration.step3Body"),
    },
    {
      title: t("integration.step4Title"),
      body: t("integration.step4Body"),
    },
    {
      title: t("integration.step5Title"),
      body: t("integration.step5Body"),
    },
  ];

  const consoleLinks = (
    <>
      <Link
        href="/agents"
        className="rounded-md bg-blue-600 px-3 py-2 text-xs font-medium text-white hover:bg-blue-500"
      >
        {t("integration.linkAgents")}
      </Link>
      <Link
        href="/recharge"
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.linkRecharge")}
      </Link>
      <Link
        href="/authorize"
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.linkAuthorize")}
      </Link>
      <Link
        href="/transactions"
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.linkTransactions")}
      </Link>
      <Link
        href="/developer"
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.linkDeveloper")}
      </Link>
      <Link
        href="/settings"
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.linkSettings")}
      </Link>
    </>
  );

  const publicLinks = (
    <>
      <Link
        href={loginNext("/agents")}
        className="rounded-md bg-blue-600 px-3 py-2 text-xs font-medium text-white hover:bg-blue-500"
      >
        {t("integration.ctaLoginAgents")}
      </Link>
      <Link
        href={loginNext("/recharge")}
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.ctaLoginRecharge")}
      </Link>
      <Link
        href={loginNext("/authorize")}
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.ctaLoginAuthorize")}
      </Link>
      <Link
        href={loginNext("/transactions")}
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.ctaLoginTransactions")}
      </Link>
      <Link
        href={loginNext("/developer")}
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.ctaLoginDeveloper")}
      </Link>
      <Link
        href={loginNext("/settings")}
        className="rounded-md border border-slate-600 px-3 py-2 text-xs text-slate-200 hover:bg-slate-800"
      >
        {t("integration.ctaLoginSettings")}
      </Link>
    </>
  );

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-xl font-semibold text-slate-100">{t("integration.title")}</h2>
        <p className="mt-2 text-sm leading-relaxed text-slate-400">{t("integration.subtitle")}</p>
        {variant === "public" ? (
          <p className="mt-3 text-xs text-slate-500">{t("integration.publicShareHint")}</p>
        ) : null}
      </div>

      <div className="rounded-xl border border-blue-900/50 bg-blue-950/20 p-5">
        <h3 className="text-sm font-medium text-blue-200">{t("integration.flowTitle")}</h3>
        <p className="mt-3 text-sm leading-relaxed text-slate-300">{t("integration.flowIntro")}</p>
      </div>

      <div className="space-y-4">
        {stepCards.map((card, i) => (
          <div key={i} className="rounded-xl border border-slate-800 bg-slate-900 p-5">
            <div className="flex gap-3">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-800 text-sm font-semibold text-blue-300">
                {i + 1}
              </span>
              <div>
                <h3 className="text-sm font-medium text-slate-100">{card.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-400">{card.body}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="rounded-xl border border-amber-900/40 bg-amber-950/20 p-5">
        <h3 className="text-sm font-medium text-amber-100">{t("integration.signatureHintTitle")}</h3>
        <p className="mt-2 font-mono text-xs leading-relaxed text-amber-200/90">
          {t("integration.signatureHintBody")}
        </p>
      </div>

      <div className="rounded-xl border border-slate-700 bg-slate-900/60 p-5">
        <h3 className="text-sm font-medium text-slate-200">{t("integration.merchantIdTitle")}</h3>
        <p className="mt-3 whitespace-pre-line text-sm leading-relaxed text-slate-400">
          {t("integration.merchantIdBody")}
        </p>
      </div>

      <div>
        <p className="mb-3 text-xs font-medium text-slate-500">
          {variant === "public" ? t("integration.publicShortcutsTitle") : t("integration.consoleShortcutsTitle")}
        </p>
        <div className="flex flex-wrap gap-2">{variant === "public" ? publicLinks : consoleLinks}</div>
      </div>

      <div className="border-t border-slate-800 pt-8">
        <p className="text-xs font-medium uppercase tracking-wide text-slate-500">
          {t("integration.techSectionLabel")}
        </p>
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
    </div>
  );
}
