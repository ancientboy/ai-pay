"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState } from "react";
import { LocaleSwitcher } from "@/components/locale-switcher";
import { useLocale } from "@/components/locale-provider";
import { type SessionClaims } from "@/lib/session";

export function ConsoleShell({
  children,
  session,
}: {
  children: React.ReactNode;
  session: SessionClaims;
}) {
  const { t } = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const [loggingOut, setLoggingOut] = useState(false);
  const role = session.role ?? "operator";
  const navItems = [
    { href: "/", label: t("nav.home"), roles: ["admin", "operator", "readonly"] },
    { href: "/dashboard", label: t("nav.dashboard"), roles: ["admin", "operator", "readonly"] },
    { href: "/billing", label: t("nav.billing"), roles: ["admin", "operator", "readonly"] },
    { href: "/agents", label: t("nav.agents"), roles: ["admin", "operator"] },
    { href: "/authorize", label: t("nav.authorize"), roles: ["admin", "operator"] },
    { href: "/recharge", label: t("nav.recharge"), roles: ["admin", "operator"] },
    { href: "/transactions", label: t("nav.transactions"), roles: ["admin", "operator", "readonly"] },
    { href: "/developer", label: t("nav.developer"), roles: ["admin"] },
    { href: "/settings", label: t("nav.settings"), roles: ["admin", "operator", "readonly"] },
  ].filter((item) => item.roles.includes(role));

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex min-h-screen max-w-7xl">
        <aside className="w-64 border-r border-slate-800 bg-slate-900/70 p-6">
          <h1 className="text-lg font-semibold tracking-wide text-blue-300">
            {t("app.title")}
          </h1>
          <p className="mt-1 text-xs text-slate-400">{t("app.subtitle")}</p>
          <nav className="mt-8 flex flex-col gap-2">
            {navItems.map((item) => {
              const active =
                item.href === "/"
                  ? pathname === "/" || pathname === ""
                  : pathname.startsWith(item.href);
              return (
              <Link
                key={item.href}
                href={item.href}
                className={`rounded-md px-3 py-2 text-sm transition ${
                  active
                    ? "bg-slate-800 text-white"
                    : "text-slate-300 hover:bg-slate-800 hover:text-white"
                }`}
              >
                {item.label}
              </Link>
            );
            })}
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
              <Link
                href="/"
                className="rounded border border-slate-600 px-3 py-1 text-xs text-slate-300 hover:bg-slate-800"
              >
                {t("nav.home")}
              </Link>
              <div className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">
                {t("app.role")}: {role}
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
