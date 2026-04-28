"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { useLocale } from "@/components/locale-provider";
import { SessionClaims } from "@/lib/session";

export function ConsoleShell({
  session,
  children,
}: {
  session: SessionClaims | null;
  children: React.ReactNode;
}) {
  const { t } = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const [loggingOut, setLoggingOut] = useState(false);
  const currentRole = (session?.role || "operator").toLowerCase();
  const canManageDeveloper = currentRole !== "readonly";
  const navItems = [
    { href: "/console", label: t("nav.home") },
    { href: "/dashboard", label: t("nav.dashboard") },
    { href: "/agents", label: t("nav.agents") },
    { href: "/kyc", label: t("nav.kyc") },
    { href: "/billing", label: t("nav.billing") },
    { href: "/authorize", label: t("nav.authorize") },
    { href: "/recharge", label: t("nav.recharge") },
    { href: "/transactions", label: t("nav.transactions") },
    { href: "/self-hosted", label: t("nav.selfHosted") },
    ...(currentRole === "admin" ? [{ href: "/admin-subscriptions", label: t("nav.adminSubscriptions") }] : []),
    ...(canManageDeveloper ? [{ href: "/developer", label: t("nav.developer") }] : []),
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
                className={`rounded-md px-3 py-2 text-sm transition ${
                  pathname.startsWith(item.href)
                    ? "bg-slate-800 text-white"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                }`}
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
              <div className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">
                {t("app.role")}: {currentRole}
              </div>
              <LocaleSwitcher />
              <div className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">
                {t("common.environment")}: {t("common.local")}
              </div>
              <button
                type="button"
                disabled={loggingOut}
                onClick={async () => {
                  setLoggingOut(true);
                  try {
                    await fetch("/api/auth/logout", { method: "POST" });
                  } finally {
                    router.replace("/login");
                    router.refresh();
                    setLoggingOut(false);
                  }
                }}
                className="rounded border border-slate-700 px-3 py-1 text-xs text-slate-300 hover:bg-slate-800 disabled:opacity-60"
              >
                {loggingOut ? `${t("common.loading")}` : t("common.logout")}
              </button>
            </div>
          </header>
          <main className="flex-1 p-6">{children}</main>
        </div>
      </div>
    </div>
  );
}
