"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  adminAdjustSubscription,
  adminListSubscriptions,
  SubscriptionSummary,
  SubscriptionPlanID,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

export default function AdminSubscriptionsPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [targetUserID, setTargetUserID] = useState("");
  const [targetPlan, setTargetPlan] = useState<SubscriptionPlanID>("starter");

  const subscriptionsQuery = useQuery({
    queryKey: ["admin-subscriptions"],
    queryFn: () => adminListSubscriptions(),
  });

  const adjustMutation = useMutation({
    mutationFn: (input: { userId: string; targetPlan: SubscriptionPlanID }) =>
      adminAdjustSubscription({
        userId: input.userId,
        planCode: input.targetPlan,
        autoRenew: true,
      }),
    onSuccess: () => {
      showToast("success", t("billing.adminAdjustSuccess"));
      setTargetUserID("");
      queryClient.invalidateQueries({ queryKey: ["admin-subscriptions"] });
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("billing.adminTitle")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("billing.adminSubtitle")}</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("billing.adjustTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <input
            value={targetUserID}
            onChange={(e) => setTargetUserID(e.target.value)}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("billing.targetUserId")}
          />
          <select
            value={targetPlan}
            onChange={(e) => setTargetPlan(e.target.value as SubscriptionPlanID)}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            <option value="starter">Starter</option>
            <option value="growth">Growth</option>
            <option value="enterprise">Enterprise</option>
          </select>
          <button
            onClick={() => {
              if (!targetUserID.trim()) {
                showToast("error", t("billing.targetUserIdRequired"));
                return;
              }
              adjustMutation.mutate({ userId: targetUserID.trim(), targetPlan });
            }}
            disabled={adjustMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white disabled:opacity-60"
          >
            {t("billing.applyAdjust")}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("billing.adminListTitle")}</h3>
        {subscriptionsQuery.isLoading ? (
          <p className="mt-3 text-sm text-slate-400">{t("common.loading")}</p>
        ) : null}
        <table className="mt-3 w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="pb-2">{t("billing.userId")}</th>
              <th className="pb-2">{t("billing.plan")}</th>
              <th className="pb-2">{t("billing.status")}</th>
              <th className="pb-2">{t("billing.nextRenewalAt")}</th>
            </tr>
          </thead>
          <tbody>
            {(subscriptionsQuery.data ?? []).map((item: SubscriptionSummary) => (
              <tr key={`${item.userId}-${item.plan}`} className="border-b border-slate-800/60">
                <td className="py-2">{item.userId}</td>
                <td className="py-2">{item.plan}</td>
                <td className="py-2">{item.status}</td>
                <td className="py-2">{new Date(item.nextRenewalAt).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
