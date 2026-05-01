import Link from "next/link";

const plans = [
  {
    name: "Starter",
    price: "$49/mo",
    desc: "适合 PoC 与小规模试运行",
    bullets: ["订阅收款", "基础对账", "标准运营面板"],
  },
  {
    name: "Growth",
    price: "$199/mo",
    desc: "适合生产环境与团队协作",
    bullets: ["支付链接充值", "退款/争议回退", "异常对账与导出"],
  },
  {
    name: "Enterprise",
    price: "Custom",
    desc: "适合高合规与高交易量场景",
    bullets: ["角色权限细分", "审计与运维能力", "可扩展私有化能力"],
  },
] as const;

export default function Home() {
  return (
    <main className="min-h-screen bg-slate-950 text-slate-100">
      <section className="mx-auto max-w-6xl px-6 py-14">
        <p className="text-xs uppercase tracking-[0.2em] text-slate-400">AI-native Payments</p>
        <h1 className="mt-3 text-4xl font-semibold text-blue-300">AI Pay 平台首页</h1>
        <p className="mt-4 max-w-3xl text-sm text-slate-300">
          聚合订阅收款、充值、退款回退、对账与运营管控。管理员与普通用户使用同一平台，但按角色看到不同能力边界。
        </p>
        <div className="mt-7 flex flex-wrap gap-3">
          <Link
            href="/login"
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500"
          >
            登录控制台
          </Link>
          <Link
            href="/register"
            className="rounded-md border border-slate-700 px-4 py-2 text-sm text-slate-200 hover:bg-slate-800"
          >
            注册普通用户
          </Link>
          <Link
            href="/billing"
            className="rounded-md border border-emerald-700/60 px-4 py-2 text-sm text-emerald-200 hover:bg-emerald-950/30"
          >
            查看账单能力
          </Link>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-6 pb-16">
        <h2 className="text-xl font-semibold text-slate-100">订阅套餐</h2>
        <p className="mt-2 text-sm text-slate-400">你之前看到的套餐信息已恢复在首页。</p>
        <div className="mt-5 grid gap-4 md:grid-cols-3">
          {plans.map((plan) => (
            <article key={plan.name} className="rounded-xl border border-slate-800 bg-slate-900 p-5">
              <div className="flex items-center justify-between">
                <h3 className="text-lg font-medium text-slate-100">{plan.name}</h3>
                <span className="text-sm font-semibold text-blue-300">{plan.price}</span>
              </div>
              <p className="mt-2 text-sm text-slate-400">{plan.desc}</p>
              <ul className="mt-4 space-y-1 text-xs text-slate-300">
                {plan.bullets.map((b) => (
                  <li key={b}>• {b}</li>
                ))}
              </ul>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
