"use client";

import { FormEvent, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  bindWallet,
  createAuthSession,
  requestPaymentSign,
  revokeAuthSession,
  submitPaymentSign,
  walletUnbind,
  type AuthSessionRecord,
  type PaymentSignRequestRecord,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

export default function SelfHostedPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const [agentDid, setAgentDid] = useState("");
  const [walletAddress, setWalletAddress] = useState("");
  const [walletLabel, setWalletLabel] = useState("");
  const [ttlMinutes, setTtlMinutes] = useState("30");
  const [merchantId, setMerchantId] = useState("");
  const [amount, setAmount] = useState("1");
  const [currentSession, setCurrentSession] = useState<AuthSessionRecord | null>(null);
  const [pendingSign, setPendingSign] = useState<PaymentSignRequestRecord | null>(null);
  const [lastTxID, setLastTxID] = useState("");
  const [lastPayStatus, setLastPayStatus] = useState("");

  const bindMutation = useMutation({
    mutationFn: () =>
      bindWallet({
        agentDid: agentDid.trim(),
        walletAddress: walletAddress.trim(),
        label: walletLabel.trim() || undefined,
      }),
    onSuccess: () => showToast("success", t("selfHosted.walletBound")),
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const unbindMutation = useMutation({
    mutationFn: () => walletUnbind(agentDid.trim()),
    onSuccess: () => showToast("success", t("selfHosted.walletUnbound")),
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const createSessionMutation = useMutation({
    mutationFn: () =>
      createAuthSession({
        agentDid: agentDid.trim(),
        ttlMinutes: Number(ttlMinutes) || 30,
      }),
    onSuccess: (session) => {
      setCurrentSession(session);
      showToast("success", `${t("selfHosted.sessionCreated")}: ${session.sessionId}`);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const revokeSessionMutation = useMutation({
    mutationFn: () => {
      if (!currentSession?.sessionId) {
        throw new Error(t("selfHosted.noSession"));
      }
      return revokeAuthSession({
        agentDid: agentDid.trim(),
        sessionId: currentSession.sessionId,
      });
    },
    onSuccess: () => {
      setCurrentSession((prev) => (prev ? { ...prev, status: "REVOKED" } : prev));
      showToast("success", t("selfHosted.sessionRevoked"));
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const requestSignMutation = useMutation({
    mutationFn: () =>
      requestPaymentSign({
        agentDid: agentDid.trim(),
        merchantId: merchantId.trim(),
        amount: amount.trim(),
        sessionId: currentSession?.sessionId || undefined,
      }),
    onSuccess: (record) => {
      setPendingSign(record);
      showToast("success", `${t("selfHosted.signRequested")}: ${record.signId}`);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const submitSignMutation = useMutation({
    mutationFn: () => {
      if (!pendingSign?.signId) {
        throw new Error(t("selfHosted.noSignRequest"));
      }
      return submitPaymentSign({
        signId: pendingSign.signId,
        payerDid: agentDid.trim(),
        merchantId: merchantId.trim(),
        amount: amount.trim(),
      });
    },
    onSuccess: (resp) => {
      setLastTxID(resp.transactionId);
      setLastPayStatus(resp.status);
      setPendingSign((prev) => (prev ? { ...prev, status: "COMPLETED" } : prev));
      showToast("success", `${t("selfHosted.signSubmitted")}: ${resp.transactionId}`);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  function ensureAgentDid() {
    if (!agentDid.trim()) {
      showToast("error", t("selfHosted.requiredError"));
      return false;
    }
    return true;
  }

  function onWalletBind(e: FormEvent) {
    e.preventDefault();
    if (!ensureAgentDid() || !walletAddress.trim()) {
      showToast("error", t("selfHosted.requiredError"));
      return;
    }
    bindMutation.mutate();
  }

  function onSessionCreate(e: FormEvent) {
    e.preventDefault();
    if (!ensureAgentDid()) {
      return;
    }
    createSessionMutation.mutate();
  }

  function onSignRequest(e: FormEvent) {
    e.preventDefault();
    if (!ensureAgentDid() || !merchantId.trim() || !amount.trim()) {
      showToast("error", t("selfHosted.requiredError"));
      return;
    }
    requestSignMutation.mutate();
  }

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("selfHosted.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("selfHosted.subtitle")}</p>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <form onSubmit={onWalletBind} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-100">{t("selfHosted.walletTitle")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("transactions.payerDid")}
            />
            <input
              value={walletAddress}
              onChange={(e) => setWalletAddress(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("selfHosted.walletAddress")}
            />
            <input
              value={walletLabel}
              onChange={(e) => setWalletLabel(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("selfHosted.walletLabel")}
            />
            <div className="grid gap-2 md:grid-cols-2">
              <button
                type="submit"
                disabled={bindMutation.isPending}
                className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white"
              >
                {t("selfHosted.bindWallet")}
              </button>
              <button
                type="button"
                onClick={() => unbindMutation.mutate()}
                disabled={unbindMutation.isPending}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                {t("selfHosted.unbindWallet")}
              </button>
            </div>
          </div>
        </form>

        <form onSubmit={onSessionCreate} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-100">{t("selfHosted.sessionTitle")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("transactions.payerDid")}
            />
            <input
              value={ttlMinutes}
              onChange={(e) => setTtlMinutes(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("selfHosted.ttlMinutes")}
            />
            <div className="grid gap-2 md:grid-cols-2">
              <button
                type="submit"
                disabled={createSessionMutation.isPending}
                className="rounded-md bg-indigo-600 px-3 py-2 text-sm font-medium text-white"
              >
                {t("selfHosted.createSession")}
              </button>
              <button
                type="button"
                onClick={() => revokeSessionMutation.mutate()}
                disabled={revokeSessionMutation.isPending || !currentSession?.sessionId}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 disabled:opacity-60"
              >
                {t("selfHosted.revokeSession")}
              </button>
            </div>
          </div>
          <div className="mt-3 rounded-md border border-slate-700 bg-slate-950 p-3 text-xs text-slate-300">
            <p className="font-medium text-slate-100">{t("selfHosted.currentSession")}</p>
            {currentSession ? (
              <>
                <p>{currentSession.sessionId}</p>
                <p>
                  {t("selfHosted.status")}: {currentSession.status}
                </p>
                <p>{currentSession.expiresAt}</p>
              </>
            ) : (
              <p>{t("selfHosted.noSession")}</p>
            )}
          </div>
        </form>

        <form onSubmit={onSignRequest} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-100">{t("selfHosted.signTitle")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("transactions.payerDid")}
            />
            <input
              value={merchantId}
              onChange={(e) => setMerchantId(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("selfHosted.merchantId")}
            />
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("selfHosted.amount")}
            />
            <div className="grid gap-2 md:grid-cols-2">
              <button
                type="submit"
                disabled={requestSignMutation.isPending}
                className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white"
              >
                {t("selfHosted.requestSign")}
              </button>
              <button
                type="button"
                onClick={() => submitSignMutation.mutate()}
                disabled={submitSignMutation.isPending || !pendingSign?.signId}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 disabled:opacity-60"
              >
                {t("selfHosted.submitSigned")}
              </button>
            </div>
          </div>
          <div className="mt-3 rounded-md border border-slate-700 bg-slate-950 p-3 text-xs text-slate-300">
            <p className="font-medium text-slate-100">{t("selfHosted.pendingSign")}</p>
            {pendingSign ? (
              <>
                <p>
                  {t("selfHosted.signId")}: {pendingSign.signId}
                </p>
                <p>
                  {t("selfHosted.status")}: {pendingSign.status}
                </p>
              </>
            ) : (
              <p>{t("selfHosted.noSignRequest")}</p>
            )}
            {lastTxID ? (
              <p className="mt-2">
                {t("selfHosted.transactionId")}: {lastTxID} · {t("selfHosted.status")}: {lastPayStatus}
              </p>
            ) : null}
          </div>
        </form>
      </div>
    </section>
  );
}
