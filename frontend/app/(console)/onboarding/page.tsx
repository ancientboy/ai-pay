"use client";

import Link from "next/link";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useLocale } from "@/components/locale-provider";
import { getOnboardingProgress, setOnboardingStepDone } from "@/lib/console-api";

export default function OnboardingPage() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";
  const progressQuery = useQuery({
    queryKey: ["onboarding", "progress"],
    queryFn: getOnboardingProgress,
  });
  const doneMutation = useMutation({
    mutationFn: (step: "agent" | "kyc" | "recharge" | "authorize" | "pay" | "selfhosted") =>
      setOnboardingStepDone(step),
    onSuccess: () => progressQuery.refetch(),
  });

  const steps: Array<{
    id: "agent" | "kyc" | "recharge" | "authorize" | "pay" | "selfhosted";
    no: string;
    title: string;
    desc: string;
    href: string;
    cta: string;
    done: boolean;
  }> = [
    {
      id: "agent",
      no: "1",
      title: isEN ? "Create your first Agent" : "创建第一个 Agent",
      desc: isEN
        ? "Register DID and generate VA account for payment identity."
        : "完成 DID 注册并生成用于支付的 VA 账户。",
      href: "/agents",
      cta: isEN ? "Go to Agents" : "前往 Agent 管理",
      done: !!progressQuery.data?.agent,
    },
    {
      id: "kyc",
      no: "2",
      title: isEN ? "Complete KYC (recommended first)" : "完成 KYC（建议优先）",
      desc: isEN
        ? "Sync Bridge customer and finish hosted KYC to unlock stablecoin address allocation."
        : "同步 Bridge 客户并完成 Hosted KYC，解锁稳定币充值地址分配。",
      href: "/kyc",
      cta: isEN ? "Go to KYC" : "前往 KYC",
      done: !!progressQuery.data?.kyc,
    },
    {
      id: "recharge",
      no: "3",
      title: isEN ? "Recharge your account" : "账户充值",
      desc: isEN
        ? "Choose currency and recharge mode, then verify on-chain confirmation if needed."
        : "选择币种与充值模式，必要时校验链上确认状态。",
      href: "/recharge",
      cta: isEN ? "Go to Recharge" : "前往充值",
      done: !!progressQuery.data?.recharge,
    },
    {
      id: "authorize",
      no: "4",
      title: isEN ? "Set authorization rules" : "设置授权规则",
      desc: isEN
        ? "Configure single/day limits and merchant whitelist before production traffic."
        : "上线前请配置单笔/日限额与商户白名单。",
      href: "/authorize",
      cta: isEN ? "Go to Authorize" : "前往授权规则",
      done: !!progressQuery.data?.authorize,
    },
    {
      id: "pay",
      no: "5",
      title: isEN ? "Run first payment" : "发起首笔支付",
      desc: isEN
        ? "Execute payment via x402 or self-hosted signing flow and monitor transaction status."
        : "通过 x402 或自托管签名发起支付，并跟踪交易状态。",
      href: "/transactions",
      cta: isEN ? "Go to Transactions" : "前往交易",
      done: !!progressQuery.data?.pay,
    },
  ];

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">
          {isEN ? "Quick Start Onboarding" : "新手快速引导"}
        </h2>
        <p className="mt-1 text-sm text-slate-400">
          {isEN
            ? "Follow these steps to move from registration to your first production-grade payment."
            : "按下面步骤，从注册走到首笔可生产化支付。"}
        </p>
      </div>
      <div className="space-y-3">
        {steps.map((step) => (
          <article
            key={step.id}
            className="rounded-xl border border-slate-800 bg-slate-900 p-4"
          >
            <p className="text-xs text-blue-300">
              {isEN ? "Step" : "步骤"} {step.no}
            </p>
            <h3 className="mt-1 text-sm font-medium text-slate-100">{step.title}</h3>
            <p className="mt-2 text-xs text-slate-300">{step.desc}</p>
            <div className="mt-3 flex items-center gap-2">
              <Link
                href={step.href}
                className="inline-block rounded border border-slate-700 px-3 py-1 text-xs text-slate-100 hover:bg-slate-800"
              >
                {step.cta}
              </Link>
              <button
                type="button"
                onClick={() => doneMutation.mutate(step.id)}
                disabled={doneMutation.isPending}
                className="rounded border border-emerald-700/60 px-3 py-1 text-xs text-emerald-200 disabled:opacity-60"
              >
                {isEN ? "Mark done" : "标记完成"}
              </button>
              {step.done ? (
                <span className="text-xs text-emerald-300">
                  {isEN ? "Done" : "已完成"}
                </span>
              ) : null}
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}
