"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useLocale } from "@/components/locale-provider";
import { listAgents } from "@/lib/console-api";

type CheckItem = {
  key: string;
  title: string;
  desc: string;
  done: boolean;
  href?: string;
};

export default function LaunchReadinessPage() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";
  const agentsQuery = useQuery({
    queryKey: ["launch-readiness", "agents"],
    queryFn: listAgents,
  });

  const hasAgents = (agentsQuery.data ?? []).length > 0;
  const hasWalletBound = (agentsQuery.data ?? []).some((a) => (a.walletAddress ?? "").trim().length > 0);
  const checks: CheckItem[] = [
    {
      key: "agent",
      title: isEN ? "At least one Agent created" : "已创建至少一个 Agent",
      desc: isEN ? "Production traffic needs a valid payer DID + VA account." : "生产支付需要有效的付款 DID 与 VA 账户。",
      done: hasAgents,
      href: "/agents",
    },
    {
      key: "kyc",
      title: isEN ? "KYC and recharge address verified" : "KYC 与充值地址已验证",
      desc: isEN ? "Complete KYC flow and verify stablecoin deposit address availability." : "完成 KYC 后确认稳定币充值地址可用。",
      done: hasWalletBound,
      href: "/kyc",
    },
    {
      key: "authorize",
      title: isEN ? "Authorize and risk rules configured" : "授权与风控规则已配置",
      desc: isEN ? "Set single/day limits and merchant whitelist before go-live." : "上线前需配置单笔/日限额与白名单。",
      done: hasAgents,
      href: "/authorize",
    },
    {
      key: "payment",
      title: isEN ? "Small-amount payment smoke passed" : "小额支付冒烟验证通过",
      desc: isEN ? "Run at least one end-to-end payment and confirm status traceability." : "至少完成一笔端到端支付并确认状态可追踪。",
      done: hasAgents,
      href: "/transactions",
    },
    {
      key: "incident",
      title: isEN ? "Incident runbook in place" : "失败处置预案已就绪",
      desc: isEN ? "Ensure PAY-00x troubleshooting path and requestId tracing are clear." : "确保 PAY-00x 处置路径与 requestId 排障链路明确。",
      done: true,
      href: "/incident-center",
    },
  ];

  const doneCount = checks.filter((c) => c.done).length;

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{isEN ? "Launch Readiness Checklist" : "生产就绪检查清单"}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {isEN ? "Use this checklist before enabling real customer traffic." : "上线真实用户流量前，请逐项确认。"}
        </p>
      </div>

      <div className="rounded-xl border border-blue-700/50 bg-blue-950/30 p-4 text-sm">
        <p className="text-blue-200">
          {isEN ? "Readiness progress" : "就绪进度"}: {doneCount}/{checks.length}
        </p>
      </div>

      <div className="space-y-3">
        {checks.map((item) => (
          <article
            key={item.key}
            className={`rounded-xl border p-4 ${
              item.done
                ? "border-emerald-700/50 bg-emerald-950/20"
                : "border-slate-800 bg-slate-900"
            }`}
          >
            <p className="text-sm font-medium text-slate-100">
              {item.done ? "✓ " : "○ "}
              {item.title}
            </p>
            <p className="mt-1 text-xs text-slate-300">{item.desc}</p>
            {item.href ? (
              <Link
                href={item.href}
                className="mt-3 inline-block rounded border border-slate-700 px-3 py-1 text-xs text-slate-100 hover:bg-slate-800"
              >
                {isEN ? "Open" : "前往"}
              </Link>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}
