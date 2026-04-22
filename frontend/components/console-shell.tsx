"use client";

import Link from "next/link";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { useLocale } from "@/components/locale-provider";

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  const { t } = useLocale();
  const navItems = [
    { href: "/dashboard", label: t("nav.dashboard") },
    { href: "/agents", label: t("nav.agents") },
    { href: "/authorize", label: t("nav.authorize") },
    { href: "/recharge", label: t("nav.recharge") },
    { href: "/transactions", label: t("nav.transactions") },
    { href: "/developer", label: t("nav.developer") },
    { href: "/settings", label: t("nav.settings") },
  ];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex min-h-screen max-w-7xl">
        <aside className="w-64 border-r border-slate-800 bg-slate-900/70 p-6">
          <h1 className="text-lg font-semibold tracking-wide text-blue-300">
            {t("app.title")}
          </h1>
          <p className="mt-1 text-xs text-slate-400">{t("app.subtitle")}</p>
          <nav className="mt-8 flex flex-col gap-2">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="rounded-md px-3 py-2 text-sm text-slate-300 transition hover:bg-slate-800 hover:text-white"
              >
                {item.label}
              </Link>
            ))}
          </nav>
        </aside>
        <div className="flex flex-1 flex-col">
          <header className="flex h-16 items-center justify-between border-b border-slate-800 px-6">
            <div>
              <p className="text-xs uppercase tracking-wider text-slate-500">
                {t("app.aiNative")}
              </p>
              <p className="text-sm text-slate-300">{t("app.controlCenter")}</p>
            </div>
            <div className="flex items-center gap-3">
              <LocaleSwitcher />
              <div className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">
                {t("common.environment")}: {t("common.local")}
              </div>
            </div>
          </header>
          <main className="flex-1 p-6">{children}</main>
        </div>
      </div>
    </div>
  );
}
