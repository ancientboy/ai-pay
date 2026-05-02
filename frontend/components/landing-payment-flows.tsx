"use client";

import { useLocale } from "@/components/locale-provider";

function ArrowRight({ className }: { className?: string }) {
  return (
    <div
      className={`flex shrink-0 items-center justify-center text-slate-600 ${className ?? ""}`}
      aria-hidden
    >
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" className="hidden md:block">
        <path
          d="M5 12h12m0 0l-4-4m4 4l-4 4"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
      <span className="md:hidden text-lg leading-none text-slate-600">↓</span>
    </div>
  );
}

function FlowCard({
  accent,
  title,
  subtitle,
  pill,
}: {
  accent: "blue" | "violet" | "emerald" | "amber";
  title: string;
  subtitle: string;
  pill: string;
}) {
  const ring =
    accent === "blue"
      ? "ring-blue-500/30 border-blue-500/25"
      : accent === "violet"
        ? "ring-violet-500/30 border-violet-500/25"
        : accent === "emerald"
          ? "ring-emerald-500/30 border-emerald-500/25"
          : "ring-amber-500/30 border-amber-500/25";
  const dot =
    accent === "blue"
      ? "bg-blue-500"
      : accent === "violet"
        ? "bg-violet-500"
        : accent === "emerald"
          ? "bg-emerald-500"
          : "bg-amber-500";
  return (
    <div
      className={`relative flex min-h-[120px] flex-1 flex-col rounded-xl border bg-slate-900/80 p-4 ring-1 ${ring}`}
    >
      <span
        className={`mb-2 inline-flex w-fit items-center rounded-full px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-slate-200 ${accent === "blue" ? "bg-blue-950/80" : accent === "violet" ? "bg-violet-950/80" : accent === "emerald" ? "bg-emerald-950/80" : "bg-amber-950/80"}`}
      >
        <span className={`mr-1.5 h-1.5 w-1.5 rounded-full ${dot}`} />
        {pill}
      </span>
      <p className="text-sm font-medium text-white">{title}</p>
      <p className="mt-2 text-xs leading-relaxed text-slate-400">{subtitle}</p>
    </div>
  );
}

export function LandingMoneyLines() {
  const { t } = useLocale();
  return (
    <div className="mt-10 space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-stretch md:justify-between">
        <FlowCard
          accent="blue"
          pill={t("landing.moneyLinePillPlatform")}
          title={t("landing.moneyLineTitlePlatform")}
          subtitle={t("landing.moneyLineDescPlatform")}
        />
        <ArrowRight className="md:pt-10" />
        <FlowCard
          accent="emerald"
          pill={t("landing.moneyLinePillVa")}
          title={t("landing.moneyLineTitleVa")}
          subtitle={t("landing.moneyLineDescVa")}
        />
        <ArrowRight className="md:pt-10" />
        <FlowCard
          accent="violet"
          pill={t("landing.moneyLinePillAgent")}
          title={t("landing.moneyLineTitleAgent")}
          subtitle={t("landing.moneyLineDescAgent")}
        />
      </div>
      <p className="rounded-lg border border-slate-800 bg-slate-950/50 px-4 py-3 text-xs leading-relaxed text-slate-500">
        {t("landing.moneyLineFootnote")}
      </p>
    </div>
  );
}

export function LandingAgentPaySwimlane() {
  const { t } = useLocale();
  const steps = [
    { key: "landing.flowStepRegister", accent: "slate" as const },
    { key: "landing.flowStepTopup", accent: "emerald" as const },
    { key: "landing.flowStepAuth", accent: "amber" as const },
    { key: "landing.flowStepPay", accent: "blue" as const },
    { key: "landing.flowStepTx", accent: "violet" as const },
  ];
  const bar =
    "h-1 w-full rounded-full bg-gradient-to-r from-slate-600 via-blue-600 to-violet-600 opacity-80";
  return (
    <div className="mt-10">
      <div className={bar} />
      <div className="mt-6 grid gap-4 sm:grid-cols-5">
        {steps.map((s, i) => (
          <div key={s.key} className="relative text-center">
            <div
              className={`mx-auto flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold text-white ${
                s.accent === "slate"
                  ? "bg-slate-600"
                  : s.accent === "emerald"
                    ? "bg-emerald-600"
                    : s.accent === "amber"
                      ? "bg-amber-600"
                      : s.accent === "blue"
                        ? "bg-blue-600"
                        : "bg-violet-600"
              }`}
            >
              {i + 1}
            </div>
            <p className="mt-3 text-xs font-medium leading-snug text-slate-200">{t(s.key)}</p>
            {i < steps.length - 1 ? (
              <div className="absolute right-0 top-5 hidden w-1/2 translate-x-1/2 border-t border-dashed border-slate-700 sm:block" />
            ) : null}
          </div>
        ))}
      </div>
      <div className="mt-8 rounded-xl border border-slate-800 bg-gradient-to-br from-slate-900/90 to-slate-950 p-6 md:p-8">
        <div className="flex flex-col items-center gap-2 md:flex-row md:justify-center md:gap-3">
          <span className="rounded-lg border border-slate-700 bg-slate-950 px-4 py-2 font-mono text-xs text-blue-300">
            {t("landing.flowLaneAgentKeys")}
          </span>
          <span className="text-slate-600">→</span>
          <span className="rounded-lg border border-emerald-800/50 bg-emerald-950/40 px-4 py-2 font-mono text-xs text-emerald-200">
            {t("landing.flowLaneVaBalance")}
          </span>
          <span className="text-slate-600">→</span>
          <span className="rounded-lg border border-amber-800/50 bg-amber-950/40 px-4 py-2 font-mono text-xs text-amber-100">
            {t("landing.flowLaneAuth")}
          </span>
          <span className="text-slate-600">→</span>
          <span className="rounded-lg border border-blue-800/50 bg-blue-950/40 px-4 py-2 font-mono text-xs text-blue-100">
            {t("landing.flowLanePay")}
          </span>
        </div>
        <p className="mt-6 text-center text-[11px] text-slate-500">{t("landing.flowDiagramCaption")}</p>
      </div>
    </div>
  );
}

export function LandingPlatformCheckoutNote() {
  const { t } = useLocale();
  return (
    <div className="rounded-xl border border-blue-900/40 bg-blue-950/20 p-5">
      <div className="flex flex-wrap items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-600/30 text-lg">
          👤
        </div>
        <div>
          <p className="text-sm font-medium text-blue-100">{t("landing.platformCheckoutTitle")}</p>
          <p className="mt-2 text-xs leading-relaxed text-slate-400">{t("landing.platformCheckoutBody")}</p>
        </div>
      </div>
    </div>
  );
}
