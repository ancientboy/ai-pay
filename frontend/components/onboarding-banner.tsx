"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLocale } from "@/components/locale-provider";

type StepItem = {
  key: string;
  href: string;
  done: boolean;
};

export function OnboardingBanner() {
  const pathname = usePathname();
  const { locale } = useLocale();
  const isEN = locale === "en-US";
  const steps: StepItem[] = [
    { key: "agent", href: "/agents", done: pathname.startsWith("/agents") },
    { key: "kyc", href: "/kyc", done: pathname.startsWith("/kyc") },
    { key: "recharge", href: "/recharge", done: pathname.startsWith("/recharge") },
    { key: "pay", href: "/transactions", done: pathname.startsWith("/transactions") },
    { key: "selfhosted", href: "/self-hosted", done: pathname.startsWith("/self-hosted") },
  ];
  const doneCount = steps.filter((s) => s.done).length;

  function stepLabel(key: string) {
    if (isEN) {
      if (key === "agent") return "Register Agent";
      if (key === "kyc") return "Complete KYC";
      if (key === "recharge") return "Recharge";
      if (key === "pay") return "First Payment";
      return "Self-hosted Sign";
    }
    if (key === "agent") return "注册 Agent";
    if (key === "kyc") return "完成 KYC";
    if (key === "recharge") return "充值入金";
    if (key === "pay") return "首笔支付";
    return "自托管签名";
  }

  return (
    <div className="rounded-xl border border-blue-700/50 bg-blue-950/30 p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-blue-200">
            {isEN ? "New to AI Pay? Follow this quick path" : "新手快速上手路径"}
          </p>
          <p className="mt-1 text-xs text-blue-300/90">
            {isEN
              ? `Progress ${doneCount}/${steps.length}.`
              : `当前进度 ${doneCount}/${steps.length}。`}
          </p>
        </div>
      </div>
      <ol className="mt-3 grid gap-2 md:grid-cols-5">
        {steps.map((step, idx) => (
          <li key={step.key}>
            <Link
              href={step.href}
              className={`block rounded-md border px-2 py-2 text-xs ${
                step.done
                  ? "border-emerald-600/60 bg-emerald-950/40 text-emerald-200"
                  : "border-slate-700 bg-slate-900/70 text-slate-200 hover:border-blue-600/60"
              }`}
            >
              <p className="font-medium">
                {idx + 1}. {stepLabel(step.key)}
              </p>
              <p className="mt-1 text-[11px] opacity-80">
                {step.done ? (isEN ? "Current step" : "当前步骤") : (isEN ? "Go" : "前往")}
              </p>
            </Link>
          </li>
        ))}
      </ol>
    </div>
  );
}
