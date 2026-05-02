"use client";

import Link from "next/link";
import { useMemo, useState, useSyncExternalStore } from "react";
import { useLocale } from "@/components/locale-provider";

const STORAGE_KEY = "ai-pay.onboardingTour.v1";

function noopSubscribe() {
  return () => {};
}

export function OnboardingTour() {
  const { t } = useLocale();
  const isClient = useSyncExternalStore(noopSubscribe, () => true, () => false);
  const [sessionHidden, setSessionHidden] = useState(false);
  const [step, setStep] = useState(0);

  const storedDismissed =
    isClient &&
    (() => {
      try {
        return window.localStorage.getItem(STORAGE_KEY) === "dismissed";
      } catch {
        return false;
      }
    })();

  const visible = isClient && !storedDismissed && !sessionHidden;

  const steps = useMemo(
    () => [
      {
        title: t("onboarding.step1Title"),
        body: t("onboarding.step1Body"),
        primaryHref: "/agents",
        primaryLabel: t("onboarding.goAgents"),
      },
      {
        title: t("onboarding.step2Title"),
        body: t("onboarding.step2Body"),
        primaryHref: "/authorize",
        primaryLabel: t("onboarding.goAuthorize"),
      },
      {
        title: t("onboarding.step3Title"),
        body: t("onboarding.step3Body"),
        primaryHref: "/docs/integration",
        primaryLabel: t("onboarding.goIntegration"),
      },
      {
        title: t("onboarding.step4Title"),
        body: t("onboarding.step4Body"),
        primaryHref: "/settings",
        primaryLabel: t("onboarding.goSettings"),
      },
    ],
    [t],
  );

  const dismiss = () => {
    try {
      window.localStorage.setItem(STORAGE_KEY, "dismissed");
    } catch {
      // ignore
    }
    setSessionHidden(true);
  };

  if (!visible || steps.length === 0) {
    return null;
  }

  const current = steps[Math.min(step, steps.length - 1)]!;
  const isLast = step >= steps.length - 1;

  return (
    <div
      className="fixed inset-0 z-[60] flex items-end justify-center bg-slate-950/70 p-4 sm:items-center"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-tour-title"
    >
      <div className="w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-5 shadow-xl">
        <p className="text-xs font-medium uppercase tracking-wide text-blue-400">
          {t("onboarding.badge")}
        </p>
        <h2 id="onboarding-tour-title" className="mt-2 text-lg font-semibold text-slate-50">
          {current.title}
        </h2>
        <p className="mt-2 text-sm leading-relaxed text-slate-400">{current.body}</p>
        <div className="mt-4 flex gap-1">
          {steps.map((_, i) => (
            <span
              key={i}
              className={`h-1.5 flex-1 rounded-full ${i <= step ? "bg-blue-500" : "bg-slate-700"}`}
            />
          ))}
        </div>
        <div className="mt-5 flex flex-wrap items-center justify-between gap-2">
          <button
            type="button"
            onClick={dismiss}
            className="text-xs text-slate-500 underline-offset-2 hover:text-slate-300 hover:underline"
          >
            {t("onboarding.skipForever")}
          </button>
          <div className="flex gap-2">
            {step > 0 ? (
              <button
                type="button"
                onClick={() => setStep((s) => Math.max(0, s - 1))}
                className="rounded-md border border-slate-600 px-3 py-1.5 text-xs text-slate-200 hover:bg-slate-800"
              >
                {t("onboarding.back")}
              </button>
            ) : null}
            {!isLast ? (
              <button
                type="button"
                onClick={() => setStep((s) => s + 1)}
                className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500"
              >
                {t("onboarding.next")}
              </button>
            ) : (
              <button
                type="button"
                onClick={dismiss}
                className="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-500"
              >
                {t("onboarding.done")}
              </button>
            )}
          </div>
        </div>
        <div className="mt-4 border-t border-slate-800 pt-4">
          <Link
            href={current.primaryHref}
            onClick={dismiss}
            className="inline-flex w-full items-center justify-center rounded-md border border-blue-500/50 bg-blue-950/40 px-3 py-2 text-sm font-medium text-blue-200 hover:bg-blue-950/70"
          >
            {current.primaryLabel}
          </Link>
        </div>
      </div>
    </div>
  );
}
