"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { listRecharges, recharge } from "@/lib/console-api";
import { ApiClientError, toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";
import { getValidationSchemas } from "@/lib/validation";

const RECHARGE_DRAFT_KEY = "ai-pay.recharge.draft.v1";

type RechargeDraft = {
  vaAccountId: string;
  vaCardNo: string;
  amount: string;
};

type RecoverableError = {
  message: string;
  code?: string;
  requestId?: string;
};

function loadRechargeDraft(): RechargeDraft {
  if (typeof window === "undefined") {
    return { vaAccountId: "", vaCardNo: "", amount: "100" };
  }
  try {
    const raw = window.localStorage.getItem(RECHARGE_DRAFT_KEY);
    if (!raw) {
      return { vaAccountId: "", vaCardNo: "", amount: "100" };
    }
    const parsed = JSON.parse(raw) as RechargeDraft;
    return {
      vaAccountId: parsed.vaAccountId ?? "",
      vaCardNo: parsed.vaCardNo ?? "",
      amount: parsed.amount ?? "100",
    };
  } catch {
    return { vaAccountId: "", vaCardNo: "", amount: "100" };
  }
}

export default function RechargePage() {
  const { t, locale } = useLocale();
  const { rechargeSchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const { showToast } = useToast();
  const [draft] = useState<RechargeDraft>(() => loadRechargeDraft());
  const [vaAccountId, setVaAccountId] = useState(draft.vaAccountId);
  const [vaCardNo, setVaCardNo] = useState(draft.vaCardNo);
  const [amount, setAmount] = useState(draft.amount);
  const [message, setMessage] = useState("");
  const [errorDetails, setErrorDetails] = useState<RecoverableError | null>(null);
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

  useEffect(() => {
    const draft: RechargeDraft = { vaAccountId, vaCardNo, amount };
    window.localStorage.setItem(RECHARGE_DRAFT_KEY, JSON.stringify(draft));
  }, [vaAccountId, vaCardNo, amount]);

  const mutation = useMutation({
    mutationFn: () =>
      recharge({
        vaAccountId: vaAccountId.trim() || undefined,
        vaCardNo: vaCardNo.trim() || undefined,
        amount,
      }),
    onSuccess: () => {
      setMessage(t("recharge.settled"));
      setErrorDetails(null);
      showToast("success", t("recharge.settled"));
      rechargesQuery.refetch();
    },
    onError: (err) => {
      const message = toReadableError(err, locale);
      setMessage(message);
      setErrorDetails(
        err instanceof ApiClientError
          ? {
              message,
              code: err.code,
              requestId: err.requestId,
            }
          : { message },
      );
      showToast("error", message);
    },
  });

  const targetId = vaCardNo.trim() || vaAccountId.trim();

  async function copyErrorInfo() {
    if (!errorDetails) {
      return;
    }
    try {
      await navigator.clipboard.writeText(
        [
          `${t("common.errorMessage")}: ${errorDetails.message}`,
          `${t("common.errorCode")}: ${errorDetails.code ?? t("common.notAvailable")}`,
          `${t("common.requestId")}: ${errorDetails.requestId ?? t("common.notAvailable")}`,
        ].join("\n"),
      );
      showToast("success", t("common.copySuccess"));
    } catch {
      showToast("error", t("common.copyFailed"));
    }
  }

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
            setErrorDetails(null);
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
          {errorDetails ? (
            <div className="mt-3 rounded-md border border-rose-700/60 bg-rose-950/30 p-3 text-xs text-rose-100">
              <p>{t("common.errorCode")}: {errorDetails.code ?? t("common.notAvailable")}</p>
              <p>{t("common.requestId")}: {errorDetails.requestId ?? t("common.notAvailable")}</p>
              <div className="mt-2 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => mutation.mutate()}
                  className="rounded border border-rose-500/60 px-2 py-1"
                >
                  {t("common.retry")}
                </button>
                <button
                  type="button"
                  onClick={copyErrorInfo}
                  className="rounded border border-rose-500/60 px-2 py-1"
                >
                  {t("common.copyError")}
                </button>
              </div>
            </div>
          ) : null}
        </form>

        <div className="space-y-4">
          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.confirmTitle")}</h3>
            <p className="mt-2 text-xs text-slate-400">{t("recharge.confirmHint")}</p>
            <div className="mt-3 space-y-1 text-xs text-slate-300">
              <p>{t("recharge.target")}: {targetId || t("common.notAvailable")}</p>
              <p>{t("recharge.amount")}: {amount || t("common.notAvailable")}</p>
            </div>
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.flowTitle")}</h3>
            <ol className="mt-2 list-decimal space-y-1 pl-5 text-xs text-slate-300">
              <li>{t("recharge.flowStep1")}</li>
              <li>{t("recharge.flowStep2")}</li>
              <li>{t("recharge.flowStep3")}</li>
            </ol>
            <p className="mt-3 text-xs text-slate-400">{t("recharge.commonFailuresTitle")}</p>
            <ul className="mt-1 list-disc space-y-1 pl-5 text-xs text-slate-300">
              <li>{t("recharge.commonFailure1")}</li>
              <li>{t("recharge.commonFailure2")}</li>
              <li>{t("recharge.commonFailure3")}</li>
            </ul>
          </div>

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
