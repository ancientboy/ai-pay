"use client";

import Link from "next/link";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { useLocale } from "@/components/locale-provider";

export function LandingPage() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <header className="mx-auto flex max-w-7xl items-center justify-between px-6 py-5">
        <div>
          <p className="text-sm font-semibold text-blue-300">
            {isEN ? "AI Native Payments" : "AI 原生支付"}
          </p>
          <p className="text-xs text-slate-400">
            {isEN ? "Unified payment infra for AI agents" : "面向 AI Agent 的统一支付基础设施"}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <LocaleSwitcher />
          <Link
            href="/login"
            className="rounded border border-slate-700 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800"
          >
            {isEN ? "Sign in" : "登录控制台"}
          </Link>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-6 pb-16">
        <section className="rounded-2xl border border-slate-800 bg-slate-900/60 p-8">
          <h1 className="text-3xl font-semibold leading-tight text-white">
            {isEN ? "Payment infrastructure for AI agents" : "面向 AI Agent 的支付基础设施"}
          </h1>
          <p className="mt-3 max-w-3xl text-sm text-slate-300">
            {isEN
              ? "Unified onboarding, signed payments, risk controls, and fund operations."
              : "统一接入、签名支付、风控与资金管理，开箱即用。"}
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <Link
              href="/login?next=/workspace"
              className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
            >
              {isEN ? "Get Started" : "立即开始"}
            </Link>
            <Link
              href="/pricing"
              className="rounded border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
            >
              {isEN ? "View Plans" : "查看套餐"}
            </Link>
          </div>
        </section>

        <section className="mt-10 grid gap-4 md:grid-cols-3">
          <FeatureCard
            title={isEN ? "Unified payment orchestration" : "统一支付编排"}
            desc={
              isEN
                ? "x402, card payment, fund transfer, and reconciliation workflows."
                : "支持 x402、卡支付、资金划拨与对账流程。"
            }
          />
          <FeatureCard
            title={isEN ? "Risk and compliance" : "风控与合规"}
            desc={
              isEN
                ? "Policy engine, KYC, and full audit trail."
                : "规则引擎、KYC 与全链路审计追踪。"
            }
          />
          <FeatureCard
            title={isEN ? "Self-hosted signing" : "自托管签名"}
            desc={
              isEN
                ? "Wallet binding, auth sessions, and two-step signing payment."
                : "钱包绑定、会话授权与两步签名支付。"
            }
          />
        </section>

        <section className="mt-12">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xl font-semibold text-white">{isEN ? "Pricing Plans" : "订阅套餐"}</h2>
            <Link href="/pricing" className="text-sm text-blue-300 hover:text-blue-200">
              {isEN ? "More details" : "查看详情"}
            </Link>
          </div>
          <div className="grid gap-4 md:grid-cols-3">
            <article className="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
              <h3 className="text-lg font-semibold text-white">Starter</h3>
              <p className="mt-1 text-sm text-slate-300">$0 / month</p>
              <ul className="mt-4 space-y-2 text-sm text-slate-300">
                <li>• {isEN ? "Agent onboarding + basic payment" : "Agent 注册与基础支付"}</li>
                <li>• {isEN ? "Recharge and transaction query" : "充值与交易查询"}</li>
                <li>• {isEN ? "Basic risk policy" : "基础风控规则"}</li>
              </ul>
            </article>
            <article className="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
              <h3 className="text-lg font-semibold text-white">Growth</h3>
              <p className="mt-1 text-sm text-slate-300">$299 / month</p>
              <ul className="mt-4 space-y-2 text-sm text-slate-300">
                <li>• {isEN ? "All Starter features" : "含 Starter 全部能力"}</li>
                <li>• {isEN ? "Advanced risk + card payment" : "高级风控与卡支付"}</li>
                <li>• {isEN ? "Self-hosted signing" : "自托管签名与会话授权"}</li>
              </ul>
            </article>
            <article className="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
              <h3 className="text-lg font-semibold text-white">Enterprise</h3>
              <p className="mt-1 text-sm text-slate-300">{isEN ? "Custom pricing" : "定制报价"}</p>
              <ul className="mt-4 space-y-2 text-sm text-slate-300">
                <li>• {isEN ? "All Growth features" : "含 Growth 全部能力"}</li>
                <li>• {isEN ? "Dedicated channel and audit policy" : "专属通道与审计策略"}</li>
                <li>• {isEN ? "Enterprise SLA support" : "企业级 SLA 与技术支持"}</li>
              </ul>
            </article>
          </div>
        </section>
      </main>
    </div>
  );
}

function FeatureCard({ title, desc }: { title: string; desc: string }) {
  return (
    <article className="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
      <h3 className="text-base font-semibold text-white">{title}</h3>
      <p className="mt-2 text-sm text-slate-300">{desc}</p>
    </article>
  );
}
