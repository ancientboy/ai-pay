"use client";

import Link from "next/link";
import { useState } from "react";
import { useLocale } from "@/components/locale-provider";

const FIRST_RUN_KEY = "ai-pay.onboarding.dismissed.v1";

export function FirstRunGuideModal() {
  const { locale } = useLocale();
  const isEN = locale === "en-US";
  const [open, setOpen] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }
    const dismissed = window.localStorage.getItem(FIRST_RUN_KEY) === "1";
    return !dismissed;
  });

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4">
      <div className="w-full max-w-xl rounded-xl border border-slate-700 bg-slate-900 p-5 shadow-2xl">
        <h3 className="text-lg font-semibold text-slate-100">
          {isEN ? "Welcome to AI Pay" : "欢迎使用 AI Pay"}
        </h3>
        <p className="mt-2 text-sm text-slate-300">
          {isEN
            ? "Start from the onboarding page to complete your first end-to-end payment."
            : "建议先进入新手引导页，按步骤完成首笔支付闭环。"}
        </p>
        <ol className="mt-4 list-decimal space-y-1 pl-5 text-xs text-slate-300">
          <li>{isEN ? "Create Agent and VA account" : "创建 Agent 与 VA 账户"}</li>
          <li>{isEN ? "Complete KYC and recharge" : "完成 KYC 与充值"}</li>
          <li>{isEN ? "Set authorize rules and pay" : "配置授权规则并发起支付"}</li>
        </ol>
        <div className="mt-5 flex items-center justify-end gap-2">
          <button
            type="button"
            onClick={() => {
              window.localStorage.setItem(FIRST_RUN_KEY, "1");
              setOpen(false);
            }}
            className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-300"
          >
            {isEN ? "Later" : "稍后"}
          </button>
          <Link
            href="/onboarding"
            onClick={() => {
              window.localStorage.setItem(FIRST_RUN_KEY, "1");
              setOpen(false);
            }}
            className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white"
          >
            {isEN ? "Open onboarding" : "打开新手引导"}
          </Link>
        </div>
      </div>
    </div>
  );
}
