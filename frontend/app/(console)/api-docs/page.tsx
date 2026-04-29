"use client";

import Link from "next/link";
import { useLocale } from "@/components/locale-provider";

export default function ApiDocsPage() {
  const { t } = useLocale();
  const samples = [
    {
      title: t("apiDocs.sampleCurlAgent"),
      command:
        "curl -X POST \"$BASE/api/v1/agents\" -H \"Content-Type: application/json\" -H \"X-User-Id: demo-user\" -d '{\"agentDid\":\"did:example:agent-001\"}'",
    },
    {
      title: t("apiDocs.sampleCurlRecharge"),
      command:
        "curl -X POST \"$BASE/api/v1/recharge\" -H \"Content-Type: application/json\" -H \"X-User-Id: demo-user\" -d '{\"vaAccount\":\"va_demo_001\",\"amount\":\"10.5\",\"currency\":\"USDC\"}'",
    },
    {
      title: t("apiDocs.sampleCurlPay"),
      command:
        "curl -X POST \"$BASE/api/v1/pay\" -H \"Content-Type: application/json\" -H \"Idempotency-Key: pay-demo-001\" -H \"X-User-Id: demo-user\" -d '{\"payerDid\":\"did:example:agent-001\",\"merchantId\":\"merchant_demo\",\"amount\":\"1.2\",\"currency\":\"USDC\"}'",
    },
  ];

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("apiDocs.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("apiDocs.subtitle")}</p>
      </div>

      <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("apiDocs.backendApi")}</h3>
        <p className="mt-1 text-xs text-slate-400">{t("apiDocs.backendDesc")}</p>
        <a
          href="/swagger/index.html"
          target="_blank"
          rel="noreferrer"
          className="mt-3 inline-block rounded border border-blue-600 px-3 py-1 text-xs text-blue-200 hover:bg-blue-950/60"
        >
          {t("apiDocs.swaggerLink")}
        </a>
      </article>

      <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("apiDocs.sampleCurlTitle")}</h3>
        <div className="mt-3 space-y-3">
          {samples.map((sample) => (
            <div key={sample.title} className="rounded border border-slate-800 bg-slate-950 p-3">
              <p className="text-xs text-slate-300">{sample.title}</p>
              <p className="mt-2 overflow-x-auto font-mono text-[11px] text-slate-400">{sample.command}</p>
            </div>
          ))}
        </div>
      </article>

      <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("apiDocs.externalDocs")}</h3>
        <ul className="mt-3 space-y-2 text-sm text-slate-300">
          <li>
            <Link
              href="https://apidocs.bridge.xyz"
              target="_blank"
              rel="noreferrer"
              className="text-blue-300 hover:text-blue-200"
            >
              {t("apiDocs.bridgeApi")}
            </Link>
          </li>
          <li>
            <Link
              href="https://docs.stripe.com/api"
              target="_blank"
              rel="noreferrer"
              className="text-blue-300 hover:text-blue-200"
            >
              {t("apiDocs.stripeApi")}
            </Link>
          </li>
        </ul>
      </article>
    </section>
  );
}
