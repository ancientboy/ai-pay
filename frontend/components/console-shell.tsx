import Link from "next/link";

const navItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/agents", label: "Agents" },
  { href: "/authorize", label: "Authorize Rules" },
  { href: "/recharge", label: "Recharge" },
  { href: "/transactions", label: "Transactions" },
  { href: "/developer", label: "Developer Center" },
  { href: "/settings", label: "Settings" },
];

export function ConsoleShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex min-h-screen max-w-7xl">
        <aside className="w-64 border-r border-slate-800 bg-slate-900/70 p-6">
          <h1 className="text-lg font-semibold tracking-wide text-blue-300">
            AI Pay Console
          </h1>
          <p className="mt-1 text-xs text-slate-400">MVP Operations Panel</p>
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
                AI-native Payments
              </p>
              <p className="text-sm text-slate-300">Control Center</p>
            </div>
            <div className="rounded-full border border-slate-700 px-3 py-1 text-xs text-slate-300">
              Environment: Local
            </div>
          </header>
          <main className="flex-1 p-6">{children}</main>
        </div>
      </div>
    </div>
  );
}
