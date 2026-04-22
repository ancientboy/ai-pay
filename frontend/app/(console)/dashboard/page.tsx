import { getDashboardMetrics, getHealth, getReady } from "@/lib/api";
import { MetricsTrendChart } from "@/components/metrics-trend-chart";

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
  const [health, ready, metrics] = await Promise.all([
    getHealth().catch(() => null),
    getReady().catch(() => null),
    getDashboardMetrics(),
  ]);

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Dashboard</h2>
        <p className="mt-1 text-sm text-slate-400">
          Overview for AI payment operations.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard label="Total Balance (GUSD)" value={metrics.totalBalance} />
        <MetricCard label="Today Spend (GUSD)" value={metrics.todaySpend} />
        <MetricCard
          label="Payment Success Rate"
          value={metrics.paymentSuccessRate}
          suffix="%"
        />
        <MetricCard label="Open Alerts" value={metrics.alertCount} />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Health Check</h3>
          <p className="mt-2 text-sm text-slate-400">
            Status: {health?.data.status ?? "offline"}
          </p>
          <p className="text-xs text-slate-500">
            Request ID: {health?.data.requestId ?? "N/A"}
          </p>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Readiness</h3>
          <p className="mt-2 text-sm text-slate-400">
            Ready: {ready?.data.ready ? "true" : "false"}
          </p>
          <p className="text-xs text-slate-500">
            Timestamp: {ready?.data.time ?? "N/A"}
          </p>
        </div>
      </div>

      <MetricsTrendChart
        todaySpend={metrics.todaySpend}
        successRate={metrics.paymentSuccessRate}
      />
    </section>
  );
}
