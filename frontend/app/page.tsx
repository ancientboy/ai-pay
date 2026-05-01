import Link from "next/link";

export default function Home() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-950 p-6 text-slate-100">
      <section className="w-full max-w-2xl rounded-2xl border border-slate-800 bg-slate-900/80 p-8">
        <p className="text-xs uppercase tracking-wider text-slate-400">AI-native Payments</p>
        <h1 className="mt-2 text-3xl font-semibold text-blue-300">AI Pay 控制台</h1>
        <p className="mt-3 text-sm text-slate-300">
          支持订阅收款、充值、对账与运营管理。你可以先注册普通用户，也可以使用管理员账号登录。
        </p>
        <div className="mt-6 flex flex-wrap gap-3">
          <Link
            href="/login"
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
          >
            去登录
          </Link>
          <Link
            href="/register"
            className="rounded-md border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
          >
            注册普通用户
          </Link>
        </div>
      </section>
    </main>
  );
}
