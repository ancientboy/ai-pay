import { getDashboardMetrics, getHealth, getReady } from "@/lib/api";
import { MetricsTrendChart } from "@/components/metrics-trend-chart";
import { cookies, headers } from "next/headers";
import { localeFromAcceptLanguage, normalizeLocale, t } from "@/lib/i18n";

function MetricCard({
  label,
  value,
  suffix = "",
}: {
  label: string;
  value: number;
  suffix?: string;
}) {
  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-50">
        {value}
        {suffix}
      </p>
    </div>
  );
}

export default async function DashboardPage() {
  const cookieStore = await cookies();
  const headerStore = await headers();
  const localeFromCookie = cookieStore.get("ai_pay_locale")?.value;
  const locale = localeFromCookie
    ? normalizeLocale(localeFromCookie)
    : localeFromAcceptLanguage(headerStore.get("accept-language"));
  const [health, ready, metrics] = await Promise.all([
    getHealth().catch(() => null),
    getReady().catch(() => null),
    getDashboardMetrics(),
  ]);

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t(locale, "dashboard.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t(locale, "dashboard.subtitle")}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard label={t(locale, "dashboard.totalBalance")} value={metrics.totalBalance} />
        <MetricCard label={t(locale, "dashboard.todaySpend")} value={metrics.todaySpend} />
        <MetricCard
          label={t(locale, "dashboard.successRate")}
          value={metrics.paymentSuccessRate}
          suffix="%"
        />
        <MetricCard label={t(locale, "dashboard.openAlerts")} value={metrics.alertCount} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t(locale, "dashboard.health")}</h3>
          <p className="mt-2 text-sm text-slate-400">
            {t(locale, "dashboard.status")}:{" "}
            {health?.data.status === "ok"
              ? t(locale, "common.ok")
              : (health?.data.status ?? t(locale, "common.offline"))}
          </p>
          <p className="text-xs text-slate-500">
            {t(locale, "dashboard.requestId")}: {health?.data.requestId ?? t(locale, "common.notAvailable")}
          </p>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t(locale, "dashboard.readiness")}</h3>
          <p className="mt-2 text-sm text-slate-400">
            {t(locale, "dashboard.ready")}: {ready?.data.ready ? t(locale, "common.yes") : t(locale, "common.no")}
          </p>
          <p className="text-xs text-slate-500">
            {t(locale, "dashboard.timestamp")}: {ready?.data.time ?? t(locale, "common.notAvailable")}
          </p>
        </div>
      </div>

      <MetricsTrendChart
        todaySpend={metrics.todaySpend}
        successRate={metrics.paymentSuccessRate}
        title={t(locale, "dashboard.trend")}
        spendLabel={t(locale, "dashboard.todaySpend")}
        successLabel={t(locale, "dashboard.successRate")}
      />
    </section>
  );
}
