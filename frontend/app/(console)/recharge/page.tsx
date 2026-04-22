"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { listRecharges, recharge } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";
import { getValidationSchemas } from "@/lib/validation";

export default function RechargePage() {
  const { t, locale } = useLocale();
  const { rechargeSchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const { showToast } = useToast();
  const [vaAccountId, setVaAccountId] = useState("");
  const [vaCardNo, setVaCardNo] = useState("");
  const [amount, setAmount] = useState("100");
  const [message, setMessage] = useState("");
  const [selected, setSelected] = useState<{
    rechargeId: string;
    vaAccountId: string;
    amount: number;
    status: string;
    createdAt: string;
  } | null>(null);

  const rechargesQuery = useQuery({
    queryKey: ["recharges", vaAccountId],
    queryFn: () => listRecharges(vaAccountId || undefined, 20),
  });

  const mutation = useMutation({
    mutationFn: () =>
      recharge({
        vaAccountId: vaAccountId.trim() || undefined,
        vaCardNo: vaCardNo.trim() || undefined,
        amount,
      }),
    onSuccess: () => {
      setMessage(t("recharge.settled"));
      showToast("success", t("recharge.settled"));
      rechargesQuery.refetch();
    },
    onError: (err) => {
      const message = toReadableError(err, locale);
      setMessage(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("recharge.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {t("recharge.subtitle")}
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <form
          className="rounded-xl border border-slate-800 bg-slate-900 p-4"
          onSubmit={(e) => {
            e.preventDefault();
            setMessage("");
            const parsed = rechargeSchema.safeParse({ vaAccountId, vaCardNo, amount });
            if (!parsed.success) {
              const msg = parsed.error.issues[0]?.message ?? `${t("common.failed")}`;
              setMessage(msg);
              showToast("error", msg);
              return;
            }
            mutation.mutate();
          }}
        >
          <h3 className="text-sm font-medium text-slate-200">{t("recharge.newRecharge")}</h3>
          <label className="mt-4 block text-sm text-slate-300">
            {t("recharge.va")}
            <input
              value={vaAccountId}
              onChange={(e) => setVaAccountId(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
          </label>
          <label className="mt-3 block text-sm text-slate-300">
            {t("recharge.vaCardNo")}
            <input
              value={vaCardNo}
              onChange={(e) => setVaCardNo(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
          </label>
          <label className="mt-3 block text-sm text-slate-300">
            {t("recharge.amount")}
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
          </label>
          <button
            disabled={mutation.isPending}
            className="mt-4 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
          >
            {t("recharge.submit")}
          </button>
          {message ? <p className="mt-3 text-sm text-slate-300">{message}</p> : null}
        </form>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("recharge.recent")}</h3>
          <ul className="mt-3 space-y-2 text-sm text-slate-300">
            {(rechargesQuery.data ?? []).map((item) => (
              <li
                key={item.rechargeId}
                className="cursor-pointer rounded px-2 py-1 hover:bg-slate-800/40"
                onClick={() => setSelected(item)}
              >
                {item.rechargeId} - {formatStatus(locale, item.status)} - {item.amount} GUSD ({item.vaAccountId})
              </li>
            ))}
            {(rechargesQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("recharge.noRecords")}</li>
            ) : null}
          </ul>
        </div>
      </div>
      <DetailModal
        open={!!selected}
        title={t("recharge.detail")}
        onClose={() => setSelected(null)}
      >
        {selected ? (
          <div className="space-y-1 font-mono text-xs">
            <p>{t("recharge.rechargeId")}: {selected.rechargeId}</p>
            <p>{t("agents.va")}: {selected.vaAccountId}</p>
            <p>{t("transactions.amount")}: {selected.amount}</p>
            <p>{t("transactions.status")}: {formatStatus(locale, selected.status)}</p>
            <p>{t("common.createdAt")}: {selected.createdAt}</p>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
