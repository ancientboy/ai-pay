"use client";

import Link from "next/link";
import { useLocale } from "@/components/locale-provider";
import { PublicHeader } from "@/components/public-header";

const plans = [
  {
    code: "starter" as const,
    name: "Starter",
    priceKey: "landing.planStarterPrice",
    descKey: "landing.planStarterDesc",
    bulletKeys: ["landing.planStarterB1", "landing.planStarterB2", "landing.planStarterB3"] as const,
    buyLabelKey: "landing.buyStarter",
  },
  {
    code: "growth" as const,
    name: "Growth",
    priceKey: "landing.planGrowthPrice",
    descKey: "landing.planGrowthDesc",
    bulletKeys: ["landing.planGrowthB1", "landing.planGrowthB2", "landing.planGrowthB3"] as const,
    buyLabelKey: "landing.buyGrowth",
  },
  {
    code: "enterprise" as const,
    name: "Enterprise",
    priceKey: "landing.planEnterprisePrice",
    descKey: "landing.planEnterpriseDesc",
    bulletKeys: ["landing.planEnterpriseB1", "landing.planEnterpriseB2", "landing.planEnterpriseB3"] as const,
    buyLabelKey: "landing.buyEnterprise",
  },
] as const;

export function LandingHome() {
  const { t } = useLocale();

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <PublicHeader />

      <section className="relative overflow-hidden border-b border-slate-800/60">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,rgba(59,130,246,0.22),transparent)]" />
        <div className="relative mx-auto max-w-6xl px-6 pb-20 pt-14 md:pb-24 md:pt-20">
          <p className="text-xs font-medium uppercase tracking-[0.22em] text-blue-400/90">
            {t("landing.heroEyebrow")}
          </p>
          <h1 className="mt-4 max-w-3xl text-4xl font-semibold leading-tight tracking-tight text-white md:text-5xl">
            {t("landing.heroTitle")}
          </h1>
          <p className="mt-6 max-w-2xl text-base leading-relaxed text-slate-400 md:text-lg">
            {t("landing.heroSubtitle")}
          </p>
          <div className="mt-10 flex flex-wrap gap-3">
            <Link
              href="/login"
              className="rounded-lg bg-blue-600 px-5 py-2.5 text-sm font-medium text-white shadow-lg shadow-blue-900/30 hover:bg-blue-500"
            >
              {t("landing.ctaLogin")}
            </Link>
            <Link
              href="/register"
              className="rounded-lg border border-slate-600 bg-slate-900/50 px-5 py-2.5 text-sm font-medium text-slate-100 hover:bg-slate-800"
            >
              {t("landing.ctaRegister")}
            </Link>
            <Link
              href="/docs/integration"
              className="rounded-lg border border-emerald-700/50 bg-emerald-950/30 px-5 py-2.5 text-sm font-medium text-emerald-200 hover:bg-emerald-950/50"
            >
              {t("landing.ctaIntegration")}
            </Link>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-6 py-16 md:py-20">
        <div className="max-w-2xl">
          <h2 className="text-2xl font-semibold text-white">{t("landing.sectionProduct")}</h2>
          <p className="mt-3 text-sm leading-relaxed text-slate-400">{t("landing.sectionProductLead")}</p>
        </div>
        <div className="mt-10 grid gap-6 md:grid-cols-3">
          <article className="rounded-2xl border border-slate-800 bg-gradient-to-b from-slate-900 to-slate-950 p-6 shadow-xl shadow-black/20">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-600/20 text-lg">
              🤖
            </div>
            <h3 className="mt-4 text-lg font-medium text-white">{t("landing.featureAgentsTitle")}</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">{t("landing.featureAgentsDesc")}</p>
          </article>
          <article className="rounded-2xl border border-slate-800 bg-gradient-to-b from-slate-900 to-slate-950 p-6 shadow-xl shadow-black/20">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-violet-600/20 text-lg">
              💳
            </div>
            <h3 className="mt-4 text-lg font-medium text-white">{t("landing.featureBillingTitle")}</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">{t("landing.featureBillingDesc")}</p>
          </article>
          <article className="rounded-2xl border border-slate-800 bg-gradient-to-b from-slate-900 to-slate-950 p-6 shadow-xl shadow-black/20">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-600/20 text-lg">
              🛡️
            </div>
            <h3 className="mt-4 text-lg font-medium text-white">{t("landing.featureRiskTitle")}</h3>
            <p className="mt-2 text-sm leading-relaxed text-slate-400">{t("landing.featureRiskDesc")}</p>
          </article>
        </div>
      </section>

      <section className="border-y border-slate-800/80 bg-slate-900/40 py-16 md:py-20">
        <div className="mx-auto max-w-6xl px-6">
          <h2 className="text-xl font-semibold text-white">{t("landing.sectionPersonas")}</h2>
          <div className="mt-8 grid gap-6 md:grid-cols-2">
            <div className="rounded-xl border border-slate-800 bg-slate-950/80 p-6">
              <p className="text-xs font-medium uppercase tracking-wide text-blue-400">{t("landing.personaAdminBadge")}</p>
              <h3 className="mt-2 text-lg font-medium text-slate-100">{t("landing.personaAdminTitle")}</h3>
              <p className="mt-3 text-sm leading-relaxed text-slate-400">{t("landing.personaAdminDesc")}</p>
            </div>
            <div className="rounded-xl border border-slate-800 bg-slate-950/80 p-6">
              <p className="text-xs font-medium uppercase tracking-wide text-emerald-400">{t("landing.personaOperatorBadge")}</p>
              <h3 className="mt-2 text-lg font-medium text-slate-100">{t("landing.personaOperatorTitle")}</h3>
              <p className="mt-3 text-sm leading-relaxed text-slate-400">{t("landing.personaOperatorDesc")}</p>
            </div>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-6 py-16 md:py-20">
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="text-2xl font-semibold text-white">{t("landing.sectionPricing")}</h2>
            <p className="mt-2 max-w-xl text-sm text-slate-400">{t("landing.sectionPricingLead")}</p>
          </div>
          <Link
            href="/docs/integration"
            className="text-sm font-medium text-blue-400 hover:text-blue-300"
          >
            {t("landing.linkIntegrationDoc")} →
          </Link>
        </div>
        <div className="mt-10 grid gap-6 md:grid-cols-3">
          {plans.map((plan) => (
            <article
              key={plan.code}
              className={`flex flex-col rounded-2xl border p-6 ${
                plan.code === "growth"
                  ? "border-blue-500/40 bg-slate-900/90 shadow-lg shadow-blue-900/10 ring-1 ring-blue-500/20"
                  : "border-slate-800 bg-slate-900/60"
              }`}
            >
              <div className="flex items-baseline justify-between gap-2">
                <h3 className="text-lg font-semibold text-white">{plan.name}</h3>
                <span className="text-sm font-semibold text-blue-300">{t(plan.priceKey)}</span>
              </div>
              <p className="mt-3 text-sm text-slate-400">{t(plan.descKey)}</p>
              <ul className="mt-5 flex-1 space-y-2 text-sm text-slate-300">
                {plan.bulletKeys.map((k) => (
                  <li key={k} className="flex gap-2">
                    <span className="text-blue-500">✓</span>
                    <span>{t(k)}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-8">
                {plan.code === "enterprise" ? (
                  <Link
                    href="#enterprise-contact"
                    className="inline-flex w-full items-center justify-center rounded-lg border border-slate-600 bg-slate-800/50 px-4 py-2.5 text-sm font-medium text-slate-100 hover:bg-slate-800"
                  >
                    {t(plan.buyLabelKey)}
                  </Link>
                ) : (
                  <Link
                    href={`/billing?plan=${plan.code}&checkoutType=subscription`}
                    className="inline-flex w-full items-center justify-center rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-medium text-white hover:bg-blue-500"
                  >
                    {t(plan.buyLabelKey)}
                  </Link>
                )}
                <p className="mt-3 text-center text-[11px] leading-relaxed text-slate-500">
                  {plan.code === "enterprise" ? t("landing.planEnterpriseFootnote") : t("landing.planCheckoutHint")}
                </p>
              </div>
            </article>
          ))}
        </div>
      </section>

      <section id="enterprise-contact" className="mx-auto max-w-6xl px-6 pb-24">
        <div className="rounded-2xl border border-slate-800 bg-gradient-to-br from-slate-900 to-slate-950 p-8 md:p-10">
          <h2 className="text-xl font-semibold text-white">{t("landing.enterpriseTitle")}</h2>
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-slate-400">{t("landing.enterpriseLead")}</p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              href="/login"
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
            >
              {t("landing.enterpriseCtaLogin")}
            </Link>
            <Link
              href="/docs/integration"
              className="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
            >
              {t("landing.enterpriseCtaDoc")}
            </Link>
          </div>
        </div>
      </section>

      <footer className="border-t border-slate-800 py-8 text-center text-xs text-slate-600">
        {t("landing.footer")}
      </footer>
    </div>
  );
}
