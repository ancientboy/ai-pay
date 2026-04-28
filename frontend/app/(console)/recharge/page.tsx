"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  getVATopupConfig,
  listAgents,
  listRecharges,
  listVATransfers,
  queryInterest,
  queryRechargeAddress,
  queryRechargeConfirm,
  recharge,
  setVATopupConfig,
  transferVA,
} from "@/lib/console-api";
import { ApiClientError, toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";
import { getValidationSchemas } from "@/lib/validation";

const RECHARGE_DRAFT_KEY = "ai-pay.recharge.draft.v1";

type CurrencyCode = "GUSD" | "USDC" | "USDT";

type RechargeDraft = {
  vaAccountId: string;
  vaCardNo: string;
  amount: string;
  currency: CurrencyCode;
  rechargeMode: "platform" | "self_hosted";
};

type RecoverableError = {
  message: string;
  code?: string;
  requestId?: string;
};

function loadRechargeDraft(): RechargeDraft {
  if (typeof window === "undefined") {
    return { vaAccountId: "", vaCardNo: "", amount: "100", currency: "GUSD", rechargeMode: "platform" };
  }
  try {
    const raw = window.localStorage.getItem(RECHARGE_DRAFT_KEY);
    if (!raw) {
      return { vaAccountId: "", vaCardNo: "", amount: "100", currency: "GUSD", rechargeMode: "platform" };
    }
    const parsed = JSON.parse(raw) as RechargeDraft;
    return {
      vaAccountId: parsed.vaAccountId ?? "",
      vaCardNo: parsed.vaCardNo ?? "",
      amount: parsed.amount ?? "100",
      currency: (parsed.currency as CurrencyCode) ?? "GUSD",
      rechargeMode: parsed.rechargeMode ?? "platform",
    };
  } catch {
    return { vaAccountId: "", vaCardNo: "", amount: "100", currency: "GUSD", rechargeMode: "platform" };
  }
}

function toISOTime(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return undefined;
  }
  return parsed.toISOString();
}

function csvEscape(value: string | number): string {
  const raw = String(value ?? "");
  return `"${raw.replace(/"/g, "\"\"")}"`;
}

export default function RechargePage() {
  const { t, locale } = useLocale();
  const { rechargeSchema, topupConfigSchema, vaTransferSchema } = useMemo(
    () => getValidationSchemas(locale),
    [locale],
  );
  const { showToast } = useToast();
  const [draft] = useState<RechargeDraft>(() => loadRechargeDraft());
  const [vaAccountId, setVaAccountId] = useState(draft.vaAccountId);
  const [vaCardNo, setVaCardNo] = useState(draft.vaCardNo);
  const [amount, setAmount] = useState(draft.amount);
  const [currency, setCurrency] = useState<CurrencyCode>(draft.currency);
  const [rechargeMode, setRechargeMode] = useState<"platform" | "self_hosted">(draft.rechargeMode);
  const [message, setMessage] = useState("");
  const [errorDetails, setErrorDetails] = useState<RecoverableError | null>(null);
  const [interestAccountId, setInterestAccountId] = useState("");
  const [topupAccountId, setTopupAccountId] = useState("");
  const [autoTopupEnabled, setAutoTopupEnabled] = useState(false);
  const [thresholdAmount, setThresholdAmount] = useState("0");
  const [targetAmount, setTargetAmount] = useState("100");
  const [fromAccountId, setFromAccountId] = useState("");
  const [toAccountId, setToAccountId] = useState("");
  const [transferAmount, setTransferAmount] = useState("1");
  const [timelineAccountId, setTimelineAccountId] = useState("");
  const [transferStatusFilter, setTransferStatusFilter] = useState("");
  const [transferStartAt, setTransferStartAt] = useState("");
  const [transferEndAt, setTransferEndAt] = useState("");
  const [transferPage, setTransferPage] = useState(1);
  const transferPageSize = 10;
  const [interestResult, setInterestResult] = useState<{
    annualRate: number;
    accruedInterest: number;
    asOf: string;
    accountId: string;
  } | null>(null);
  const [topupResult, setTopupResult] = useState<{
    accountId: string;
    autoTopupEnabled: boolean;
    thresholdAmount: number;
    targetAmount: number;
    updatedAt: string;
  } | null>(null);
  const [transferResult, setTransferResult] = useState<{
    fromAccountId: string;
    toAccountId: string;
    amount: string;
    at: string;
  } | null>(null);
  const [rechargeAddressResult, setRechargeAddressResult] = useState<{
    mode: string;
    agentDid: string;
    currency: string;
    chainId: string;
    address: string;
    isSelfHosted: boolean;
  } | null>(null);
  const [confirmRechargeId, setConfirmRechargeId] = useState("");
  const [confirmResult, setConfirmResult] = useState<{
    rechargeId: string;
    currency: string;
    requiredConfirmations: number;
    currentConfirmations: number;
    confirmed: boolean;
    status: string;
    updatedAt: string;
  } | null>(null);
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
  const timelineRechargesQuery = useQuery({
    queryKey: ["timeline-recharges", timelineAccountId],
    queryFn: () => listRecharges(timelineAccountId || undefined, 50),
  });
  const agentsQuery = useQuery({
    queryKey: ["agents-for-recharge"],
    queryFn: () => listAgents(),
  });
  const transferOffset = (transferPage - 1) * transferPageSize;
  const transferHistoryQuery = useQuery({
    queryKey: ["va-transfers", timelineAccountId, transferStatusFilter, transferStartAt, transferEndAt, transferOffset],
    queryFn: () =>
      listVATransfers({
        accountId: timelineAccountId || undefined,
        status: transferStatusFilter || undefined,
        startTime: toISOTime(transferStartAt),
        endTime: toISOTime(transferEndAt),
        limit: transferPageSize,
        offset: transferOffset,
      }),
  });

  useEffect(() => {
    const draft: RechargeDraft = { vaAccountId, vaCardNo, amount, currency, rechargeMode };
    window.localStorage.setItem(RECHARGE_DRAFT_KEY, JSON.stringify(draft));
  }, [vaAccountId, vaCardNo, amount, currency, rechargeMode]);

  const mutation = useMutation({
    mutationFn: () =>
      recharge({
        vaAccountId: vaAccountId.trim() || undefined,
        vaCardNo: vaCardNo.trim() || undefined,
        currency,
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

  const rechargeAddressMutation = useMutation({
    mutationFn: () => queryRechargeAddress({ agentDid: (agentsQuery.data ?? [])[0]?.agentDid ?? "", currency, mode: rechargeMode }),
    onSuccess: (data) => {
      setRechargeAddressResult(data);
      setMessage(`Recharge address ready: ${data.address}`);
      setErrorDetails(null);
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const rechargeConfirmMutation = useMutation({
    mutationFn: () => queryRechargeConfirm(confirmRechargeId),
    onSuccess: (data) => {
      setConfirmResult(data);
      setMessage(`Confirmations: ${data.currentConfirmations}/${data.requiredConfirmations}`);
      setErrorDetails(null);
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const interestMutation = useMutation({
    mutationFn: () => queryInterest(interestAccountId),
    onSuccess: (data) => {
      setInterestResult(data);
      setMessage(
        `${t("recharge.accruedInterest")}: ${data.accruedInterest.toFixed(6)} {currency} · ${t("recharge.annualRate")}: ${(
          data.annualRate * 100
        ).toFixed(2)}%`,
      );
      setErrorDetails(null);
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const topupConfigGetMutation = useMutation({
    mutationFn: () => getVATopupConfig(topupAccountId),
    onSuccess: (data) => {
      setAutoTopupEnabled(data.autoTopupEnabled);
      setThresholdAmount(String(data.thresholdAmount));
      setTargetAmount(String(data.targetAmount));
      setTopupResult(data);
      setMessage(`${t("recharge.topupConfigTitle")} · ${t("common.success")}`);
      setErrorDetails(null);
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const topupConfigSetMutation = useMutation({
    mutationFn: () =>
      setVATopupConfig({
        accountId: topupAccountId,
        autoTopupEnabled,
        thresholdAmount,
        targetAmount,
      }),
    onSuccess: () => {
      setMessage(t("recharge.topupConfigSaved"));
      setErrorDetails(null);
      showToast("success", t("recharge.topupConfigSaved"));
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const transferMutation = useMutation({
    mutationFn: () =>
      transferVA({
        fromAccountId,
        toAccountId,
        amount: transferAmount,
      }),
    onSuccess: () => {
      setTransferResult({
        fromAccountId,
        toAccountId,
        amount: transferAmount,
        at: new Date().toISOString(),
      });
      setMessage(t("recharge.transferSuccess"));
      setErrorDetails(null);
      showToast("success", t("recharge.transferSuccess"));
      rechargesQuery.refetch();
      transferHistoryQuery.refetch();
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setMessage(msg);
      setErrorDetails(err instanceof ApiClientError ? { message: msg, code: err.code, requestId: err.requestId } : { message: msg });
      showToast("error", msg);
    },
  });

  const targetId = vaCardNo.trim() || vaAccountId.trim();
  const timelineRows = useMemo(() => {
    const startAt = toISOTime(transferStartAt);
    const endAt = toISOTime(transferEndAt);
    const rechargeRows = (timelineRechargesQuery.data ?? []).map((item) => ({
      id: item.rechargeId,
      type: "RECHARGE" as const,
      accountId: item.vaAccountId,
      amount: item.amount,
      status: item.status,
      createdAt: item.createdAt,
      subtitle: item.vaAccountId,
    }));
    const transferRows = (transferHistoryQuery.data ?? []).map((item) => {
      const isOut = timelineAccountId && item.fromAccountId === timelineAccountId;
      return {
        id: item.transferId,
        type: isOut ? ("TRANSFER_OUT" as const) : ("TRANSFER_IN" as const),
        accountId: isOut ? item.fromAccountId : item.toAccountId,
        amount: item.amount,
        status: item.status,
        createdAt: item.createdAt,
        subtitle: `${item.fromAccountId} -> ${item.toAccountId}`,
      };
    });
    return [...rechargeRows, ...transferRows]
      .filter((item) => {
        if (transferStatusFilter && item.status.toUpperCase() !== transferStatusFilter) {
          return false;
        }
        const created = new Date(item.createdAt).getTime();
        if (startAt && created < new Date(startAt).getTime()) {
          return false;
        }
        if (endAt && created > new Date(endAt).getTime()) {
          return false;
        }
        return true;
      })
      .sort((a, b) => {
        return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
      });
  }, [timelineRechargesQuery.data, timelineAccountId, transferEndAt, transferHistoryQuery.data, transferStartAt, transferStatusFilter]);

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

  function clearTransferFilters() {
    setTransferStatusFilter("");
    setTransferStartAt("");
    setTransferEndAt("");
    setTransferPage(1);
    timelineRechargesQuery.refetch();
    transferHistoryQuery.refetch();
  }

  async function exportTransferCsv() {
    const rows = transferHistoryQuery.data ?? [];
    if (rows.length === 0) {
      showToast("error", t("recharge.noTimeline"));
      return;
    }
    const header = ["transferId", "fromAccountId", "toAccountId", "amount", "status", "idempotencyKey", "createdAt"];
    const csv = [
      header.join(","),
      ...rows.map((item) =>
        [
          item.transferId,
          item.fromAccountId,
          item.toAccountId,
          item.amount,
          item.status,
          item.idempotencyKey,
          item.createdAt,
        ]
          .map(csvEscape)
          .join(","),
      ),
    ].join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `va-transfer-${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
    showToast("success", t("common.success"));
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
              list="recharge-va-options"
              value={vaAccountId}
              onChange={(e) => setVaAccountId(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
            <datalist id="recharge-va-options">
              {(agentsQuery.data ?? []).map((agent) => (
                <option key={agent.vaAccountId} value={agent.vaAccountId}>
                  {agent.agentDid}
                </option>
              ))}
            </datalist>
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
            Currency
            <select
              value={currency}
              onChange={(e) => setCurrency(e.target.value as CurrencyCode)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            >
              <option value="GUSD">GUSD</option>
              <option value="USDC">USDC</option>
              <option value="USDT">USDT</option>
            </select>
          </label>
          <label className="mt-3 block text-sm text-slate-300">
            Recharge Mode
            <select
              value={rechargeMode}
              onChange={(e) => setRechargeMode(e.target.value as "platform" | "self_hosted")}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            >
              <option value="platform">platform</option>
              <option value="self_hosted">self_hosted</option>
            </select>
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
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.interestTitle")}</h3>
            <div className="mt-2 flex gap-2">
              <input
                list="interest-va-options"
                value={interestAccountId}
                onChange={(e) => setInterestAccountId(e.target.value)}
                placeholder={t("recharge.interestAccountId")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <datalist id="interest-va-options">
                {(agentsQuery.data ?? []).map((agent) => (
                  <option key={`i-${agent.vaAccountId}`} value={agent.vaAccountId}>
                    {agent.agentDid}
                  </option>
                ))}
              </datalist>
              <button
                type="button"
                onClick={() => {
                  if (!interestAccountId.trim()) {
                    showToast("error", t("validation.vaRequired"));
                    return;
                  }
                  interestMutation.mutate();
                }}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                {t("recharge.interestQuery")}
              </button>
            </div>
            {interestResult ? (
              <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
                <p>{t("recharge.interestAccountId")}: {interestResult.accountId}</p>
                <p>{t("recharge.annualRate")}: {(interestResult.annualRate * 100).toFixed(2)}%</p>
                <p>{t("recharge.accruedInterest")}: {interestResult.accruedInterest.toFixed(6)} {currency}</p>
                <p>{t("recharge.asOf")}: {interestResult.asOf}</p>
              </div>
            ) : null}
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.topupConfigTitle")}</h3>
            <div className="mt-2 space-y-2">
              <input
                list="topup-va-options"
                value={topupAccountId}
                onChange={(e) => setTopupAccountId(e.target.value)}
                placeholder={t("recharge.interestAccountId")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <datalist id="topup-va-options">
                {(agentsQuery.data ?? []).map((agent) => (
                  <option key={`t-${agent.vaAccountId}`} value={agent.vaAccountId}>
                    {agent.agentDid}
                  </option>
                ))}
              </datalist>
              <label className="flex items-center gap-2 text-sm text-slate-300">
                <input
                  type="checkbox"
                  checked={autoTopupEnabled}
                  onChange={(e) => setAutoTopupEnabled(e.target.checked)}
                />
                {t("recharge.autoTopupEnabled")}
              </label>
              <input
                value={thresholdAmount}
                onChange={(e) => setThresholdAmount(e.target.value)}
                placeholder={t("recharge.thresholdAmount")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <input
                value={targetAmount}
                onChange={(e) => setTargetAmount(e.target.value)}
                placeholder={t("recharge.targetAmount")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => {
                    if (!topupAccountId.trim()) {
                      showToast("error", t("validation.vaRequired"));
                      return;
                    }
                    topupConfigGetMutation.mutate();
                  }}
                  className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
                >
                  {t("transactions.queryStatus")}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    const parsed = topupConfigSchema.safeParse({
                      accountId: topupAccountId,
                      thresholdAmount,
                      targetAmount,
                    });
                    if (!parsed.success) {
                      const msg = parsed.error.issues[0]?.message ?? t("common.failed");
                      showToast("error", msg);
                      setMessage(msg);
                      return;
                    }
                    topupConfigSetMutation.mutate();
                  }}
                  className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white"
                >
                  {t("recharge.saveTopupConfig")}
                </button>
              </div>
            </div>
            {topupResult ? (
              <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
                <p>{t("recharge.interestAccountId")}: {topupResult.accountId}</p>
                <p>{t("recharge.autoTopupEnabled")}: {topupResult.autoTopupEnabled ? t("common.yes") : t("common.no")}</p>
                <p>{t("recharge.thresholdAmount")}: {topupResult.thresholdAmount}</p>
                <p>{t("recharge.targetAmount")}: {topupResult.targetAmount}</p>
                <p>{t("common.createdAt")}: {topupResult.updatedAt}</p>
              </div>
            ) : null}
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.vaTransferTitle")}</h3>
            <div className="mt-2 space-y-2">
              <input
                list="transfer-from-options"
                value={fromAccountId}
                onChange={(e) => setFromAccountId(e.target.value)}
                placeholder={t("recharge.fromAccountId")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <datalist id="transfer-from-options">
                {(agentsQuery.data ?? []).map((agent) => (
                  <option key={`f-${agent.vaAccountId}`} value={agent.vaAccountId}>
                    {agent.agentDid}
                  </option>
                ))}
              </datalist>
              <input
                list="transfer-to-options"
                value={toAccountId}
                onChange={(e) => setToAccountId(e.target.value)}
                placeholder={t("recharge.toAccountId")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <datalist id="transfer-to-options">
                {(agentsQuery.data ?? []).map((agent) => (
                  <option key={`to-${agent.vaAccountId}`} value={agent.vaAccountId}>
                    {agent.agentDid}
                  </option>
                ))}
              </datalist>
              <input
                value={transferAmount}
                onChange={(e) => setTransferAmount(e.target.value)}
                placeholder={t("recharge.transferAmount")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <button
                type="button"
                onClick={() => {
                  const parsed = vaTransferSchema.safeParse({
                    fromAccountId,
                    toAccountId,
                    amount: transferAmount,
                  });
                  if (!parsed.success) {
                    const msg = parsed.error.issues[0]?.message ?? t("common.failed");
                    showToast("error", msg);
                    setMessage(msg);
                    return;
                  }
                  transferMutation.mutate();
                }}
                className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white"
              >
                {t("recharge.transferSubmit")}
              </button>
            </div>
            {transferResult ? (
              <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
                <p>{t("recharge.fromAccountId")}: {transferResult.fromAccountId}</p>
                <p>{t("recharge.toAccountId")}: {transferResult.toAccountId}</p>
                <p>{t("recharge.transferAmount")}: {transferResult.amount}</p>
                <p>{t("common.createdAt")}: {transferResult.at}</p>
              </div>
            ) : null}
          </div>

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
            <h3 className="text-sm font-medium text-slate-200">Recharge Address</h3>
            <button
              type="button"
              onClick={() => rechargeAddressMutation.mutate()}
              className="mt-3 rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
            >
              Query Address
            </button>
            {rechargeAddressResult ? (
              <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
                <p>Mode: {rechargeAddressResult.mode}</p>
                <p>Currency: {rechargeAddressResult.currency}</p>
                <p>Chain: {rechargeAddressResult.chainId || "N/A"}</p>
                <p>Address: {rechargeAddressResult.address || "N/A"}</p>
              </div>
            ) : null}
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">Recharge Confirmations</h3>
            <div className="mt-2 flex gap-2">
              <input
                value={confirmRechargeId}
                onChange={(e) => setConfirmRechargeId(e.target.value)}
                placeholder="rechargeId"
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <button
                type="button"
                onClick={() => rechargeConfirmMutation.mutate()}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                Query
              </button>
            </div>
            {confirmResult ? (
              <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
                <p>ID: {confirmResult.rechargeId}</p>
                <p>{confirmResult.currency} {confirmResult.currentConfirmations}/{confirmResult.requiredConfirmations}</p>
                <p>Status: {confirmResult.status}</p>
              </div>
            ) : null}
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
                  {item.rechargeId} - {formatStatus(locale, item.status)} - {item.amount} {currency} ({item.vaAccountId})
                </li>
              ))}
              {(rechargesQuery.data ?? []).length === 0 ? (
                <li className="text-slate-500">{t("recharge.noRecords")}</li>
              ) : null}
            </ul>
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.transferHistory")}</h3>
            <div className="mt-2 grid gap-2 md:grid-cols-2">
              <select
                value={transferStatusFilter}
                onChange={(e) => setTransferStatusFilter(e.target.value)}
                className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              >
                <option value="">{t("recharge.transferFilterAllStatus")}</option>
                <option value="SETTLED">{formatStatus(locale, "SETTLED")}</option>
                <option value="FAILED">{formatStatus(locale, "FAILED")}</option>
              </select>
              <div className="grid grid-cols-2 gap-2">
                <input
                  type="datetime-local"
                  value={transferStartAt}
                  onChange={(e) => setTransferStartAt(e.target.value)}
                  title={t("recharge.transferFilterStart")}
                  className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
                />
                <input
                  type="datetime-local"
                  value={transferEndAt}
                  onChange={(e) => setTransferEndAt(e.target.value)}
                  title={t("recharge.transferFilterEnd")}
                  className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
                />
              </div>
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => {
                  setTransferPage(1);
                  timelineRechargesQuery.refetch();
                  transferHistoryQuery.refetch();
                }}
                className="rounded-md border border-slate-700 px-3 py-2 text-xs text-slate-200"
              >
                {t("recharge.transferApplyFilters")}
              </button>
              <button
                type="button"
                onClick={clearTransferFilters}
                className="rounded-md border border-slate-700 px-3 py-2 text-xs text-slate-200"
              >
                {t("recharge.transferClearFilters")}
              </button>
              <button
                type="button"
                onClick={exportTransferCsv}
                className="rounded-md border border-slate-700 px-3 py-2 text-xs text-slate-200"
              >
                {t("recharge.transferExportCsv")}
              </button>
            </div>
            <ul className="mt-3 space-y-2 text-sm text-slate-300">
              {(transferHistoryQuery.data ?? []).map((item) => (
                <li key={item.transferId} className="rounded border border-slate-800 p-2">
                  <p className="font-mono text-xs">{item.transferId}</p>
                  <p>{item.amount} {currency} · {formatStatus(locale, item.status)}</p>
                  <p className="text-xs text-slate-500">{item.fromAccountId} -&gt; {item.toAccountId}</p>
                  <p className="text-xs text-slate-500">{item.createdAt}</p>
                </li>
              ))}
              {(transferHistoryQuery.data ?? []).length === 0 ? (
                <li className="text-slate-500">{t("recharge.noTimeline")}</li>
              ) : null}
            </ul>
            <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
              <span>{t("recharge.transferPageInfo")}: {transferPage}</span>
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled={transferPage <= 1}
                  onClick={() => setTransferPage((v) => Math.max(1, v - 1))}
                  className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
                >
                  {t("transactions.prev")}
                </button>
                <button
                  type="button"
                  disabled={(transferHistoryQuery.data ?? []).length < transferPageSize}
                  onClick={() => setTransferPage((v) => v + 1)}
                  className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
                >
                  {t("transactions.next")}
                </button>
              </div>
            </div>
          </div>

          <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
            <h3 className="text-sm font-medium text-slate-200">{t("recharge.timelineTitle")}</h3>
            <div className="mt-2 flex gap-2">
              <input
                list="timeline-va-options"
                value={timelineAccountId}
                onChange={(e) => setTimelineAccountId(e.target.value)}
                placeholder={t("recharge.timelineAccountId")}
                className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
              <datalist id="timeline-va-options">
                {(agentsQuery.data ?? []).map((agent) => (
                  <option key={`timeline-${agent.vaAccountId}`} value={agent.vaAccountId}>
                    {agent.agentDid}
                  </option>
                ))}
              </datalist>
              <button
                type="button"
                onClick={() => {
                  timelineRechargesQuery.refetch();
                  rechargesQuery.refetch();
                  transferHistoryQuery.refetch();
                }}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                {t("developer.refresh")}
              </button>
            </div>
            <ul className="mt-3 space-y-2 text-sm text-slate-300">
              {timelineRows.map((item) => (
                <li key={`${item.type}-${item.id}`} className="rounded border border-slate-800 p-2">
                  <p className="text-xs text-slate-400">
                    {item.type === "RECHARGE"
                      ? t("recharge.timelineTypeRecharge")
                      : item.type === "TRANSFER_OUT"
                        ? t("recharge.timelineTypeTransferOut")
                        : t("recharge.timelineTypeTransferIn")}
                  </p>
                  <p className="font-mono text-xs">{item.id}</p>
                  <p>{item.amount} {currency} · {formatStatus(locale, item.status)}</p>
                  <p className="text-xs text-slate-500">{item.subtitle}</p>
                  <p className="text-xs text-slate-500">{item.createdAt}</p>
                </li>
              ))}
              {timelineRows.length === 0 ? (
                <li className="text-slate-500">{t("recharge.noTimeline")}</li>
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
