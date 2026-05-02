"use client";

import Link from "next/link";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { useLocale } from "@/components/locale-provider";

export function PublicHeader() {
  const { t } = useLocale();
  return (
    <header className="sticky top-0 z-50 border-b border-slate-800/80 bg-slate-950/90 backdrop-blur-md">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-3">
        <Link href="/" className="flex flex-col leading-tight">
          <span className="text-sm font-semibold tracking-tight text-blue-300">{t("landing.brand")}</span>
          <span className="text-[11px] font-medium text-slate-500">{t("landing.brandZh")}</span>
        </Link>
        <nav className="flex flex-wrap items-center justify-end gap-2 sm:gap-4">
          <Link
            href="/docs/integration"
            className="text-xs text-slate-400 transition hover:text-slate-100 sm:text-sm"
          >
            {t("landing.navIntegration")}
          </Link>
          <Link href="/login" className="text-xs text-slate-400 transition hover:text-slate-100 sm:text-sm">
            {t("landing.navLogin")}
          </Link>
          <Link
            href="/register"
            className="rounded-md border border-slate-600 px-2 py-1 text-xs text-slate-200 hover:bg-slate-800 sm:px-3 sm:text-sm"
          >
            {t("landing.navRegister")}
          </Link>
          <LocaleSwitcher />
        </nav>
      </div>
    </header>
  );
}
