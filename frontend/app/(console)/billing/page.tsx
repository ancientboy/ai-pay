"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  getBillingPlans,
  getCurrentSubscription,
  listMyInvoices,
  renewSubscription,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

export default function BillingPage() {
  const { locale } = useLocale();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [planId, setPlanId] = useState("growth");

  const plansQuery = useQuery({
    queryKey: ["billing", "plans"],
    queryFn: getBillingPlans,
  });
  const subQuery = useQuery({
    queryKey: ["billing", "current"],
    queryFn: getCurrentSubscription,
  });
  const invoicesQuery = useQuery({
    queryKey: ["billing", "invoices"],
    queryFn: () => listMyInvoices(20),
  });

  const renewMutation = useMutation({
    mutationFn: () => renewSubscription(planId, 1),
    onSuccess: () => {
      showToast("success", locale === "en-US" ? "Subscription updated" : "订阅已更新");
      queryClient.invalidateQueries({ queryKey: ["billing"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{locale === "en-US" ? "My Subscription" : "我的订阅"}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {locale === "en-US" ? "Manage plan and renewals" : "管理套餐与续费"}
        </p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4 text-sm">
        <p>
          {locale === "en-US" ? "Current Plan" : "当前套餐"}:{" "}
          <span className="font-semibold text-slate-100">{subQuery.data?.planId ?? "-"}</span>
        </p>
        <p className="mt-1 text-slate-400">
          {locale === "en-US" ? "Status" : "状态"}: {subQuery.data?.status ?? "-"}
        </p>
        <p className="mt-1 text-slate-400">
          {locale === "en-US" ? "Next Billing Date" : "下次扣费日"}:{" "}
          {subQuery.data?.nextBillingAt ? new Date(subQuery.data.nextBillingAt).toLocaleString() : "-"}
        </p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{locale === "en-US" ? "Renew / Change Plan" : "续费 / 变更套餐"}</h3>
        <div className="mt-3 flex gap-2">
          <select
            value={planId}
            onChange={(e) => setPlanId(e.target.value)}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            {(plansQuery.data ?? []).map((plan) => (
              <option key={plan.planId} value={plan.planId}>
                {plan.name} (${plan.priceMonthly}/mo)
              </option>
            ))}
          </select>
          <button
            onClick={() => renewMutation.mutate()}
            disabled={renewMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
          >
            {locale === "en-US" ? "Confirm" : "确认"}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{locale === "en-US" ? "Invoices" : "账单记录"}</h3>
        <ul className="mt-3 space-y-2 text-xs text-slate-300">
          {(invoicesQuery.data ?? []).map((inv) => (
            <li key={inv.invoiceId} className="rounded border border-slate-800 p-2">
              <p>
                #{inv.invoiceId} · ${inv.amount}
              </p>
              <p className="text-slate-500">{inv.status}</p>
            </li>
          ))}
          {(invoicesQuery.data ?? []).length === 0 ? (
            <li className="text-slate-500">{locale === "en-US" ? "No invoices yet" : "暂无账单"}</li>
          ) : null}
        </ul>
      </div>
    </section>
  );
}
