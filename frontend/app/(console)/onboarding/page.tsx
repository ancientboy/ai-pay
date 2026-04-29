"use client";

import Link from "next/link";
import { useLocale } from "@/components/locale-provider";

export default function OnboardingPage() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";

  const steps = [
    {
      id: "1",
      title: isEN ? "Create your first Agent" : "创建第一个 Agent",
      desc: isEN
        ? "Register DID and generate VA account for payment identity."
        : "完成 DID 注册并生成用于支付的 VA 账户。",
      href: "/agents",
      cta: isEN ? "Go to Agents" : "前往 Agent 管理",
    },
    {
      id: "2",
      title: isEN ? "Complete KYC (recommended first)" : "完成 KYC（建议优先）",
      desc: isEN
        ? "Sync Bridge customer and finish hosted KYC to unlock stablecoin address allocation."
        : "同步 Bridge 客户并完成 Hosted KYC，解锁稳定币充值地址分配。",
      href: "/kyc",
      cta: isEN ? "Go to KYC" : "前往 KYC",
    },
    {
      id: "3",
      title: isEN ? "Recharge your account" : "账户充值",
      desc: isEN
        ? "Choose currency and recharge mode, then verify on-chain confirmation if needed."
        : "选择币种与充值模式，必要时校验链上确认状态。",
      href: "/recharge",
      cta: isEN ? "Go to Recharge" : "前往充值",
    },
    {
      id: "4",
      title: isEN ? "Set authorization rules" : "设置授权规则",
      desc: isEN
        ? "Configure single/day limits and merchant whitelist before production traffic."
        : "上线前请配置单笔/日限额与商户白名单。",
      href: "/authorize",
      cta: isEN ? "Go to Authorize" : "前往授权规则",
    },
    {
      id: "5",
      title: isEN ? "Run first payment" : "发起首笔支付",
      desc: isEN
        ? "Execute payment via x402 or self-hosted signing flow and monitor transaction status."
        : "通过 x402 或自托管签名发起支付，并跟踪交易状态。",
      href: "/transactions",
      cta: isEN ? "Go to Transactions" : "前往交易",
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
              {isEN ? "Step" : "步骤"} {step.id}
            </p>
            <h3 className="mt-1 text-sm font-medium text-slate-100">{step.title}</h3>
            <p className="mt-2 text-xs text-slate-300">{step.desc}</p>
            <Link
              href={step.href}
              className="mt-3 inline-block rounded border border-slate-700 px-3 py-1 text-xs text-slate-100 hover:bg-slate-800"
            >
              {step.cta}
            </Link>
          </article>
        ))}
      </div>
    </section>
  );
}
