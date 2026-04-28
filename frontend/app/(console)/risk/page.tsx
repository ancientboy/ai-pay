"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { FormEvent, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  riskAuditQuery,
  riskKYCVerify,
  riskTransactionCheck,
  type PartyKYC,
  type RiskAuditEntry,
  type RiskDecision,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

export default function RiskPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const [agentDid, setAgentDid] = useState("");
  const [merchantId, setMerchantId] = useState("");
  const [amount, setAmount] = useState("10");
  const [transactionId, setTransactionId] = useState("");
  const [documentReference, setDocumentReference] = useState("");
  const [auditAgentDid, setAuditAgentDid] = useState("");
  const [auditMerchantId, setAuditMerchantId] = useState("");
  const [auditLimit, setAuditLimit] = useState("20");
  const [auditOffset, setAuditOffset] = useState("0");
  const [decision, setDecision] = useState<RiskDecision | null>(null);
  const [kycResult, setKycResult] = useState<PartyKYC | null>(null);
  const [selectedAudit, setSelectedAudit] = useState<RiskAuditEntry | null>(null);
  const [auditSearchToken, setAuditSearchToken] = useState(0);

  const auditQuery = useQuery({
    queryKey: ["risk", "audit", auditSearchToken, auditAgentDid, auditMerchantId, auditLimit, auditOffset],
    queryFn: () =>
      riskAuditQuery({
        agentDid: auditAgentDid.trim() || undefined,
        merchantId: auditMerchantId.trim() || undefined,
        limit: Number(auditLimit) || 20,
        offset: Number(auditOffset) || 0,
      }),
  });

  const txMutation = useMutation({
    mutationFn: () =>
      riskTransactionCheck({
        agentDid: agentDid.trim(),
        merchantId: merchantId.trim(),
        amount: amount.trim(),
        transactionId: transactionId.trim() || undefined,
      }),
    onSuccess: (res) => {
      setDecision(res);
      showToast("success", `${t("risk.txCheckSuccess")}: ${res.decision}`);
      setAuditSearchToken((v) => v + 1);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const kycMutation = useMutation({
    mutationFn: () =>
      riskKYCVerify({
        agentDid: agentDid.trim(),
        documentReference: documentReference.trim() || undefined,
      }),
    onSuccess: (res) => {
      setKycResult(res);
      showToast("success", `${t("risk.kycSuccess")}: ${res.status}`);
      setAuditSearchToken((v) => v + 1);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  function submitTxCheck(e: FormEvent) {
    e.preventDefault();
    if (!agentDid.trim() || !merchantId.trim() || !amount.trim()) {
      showToast("error", t("risk.requiredError"));
      return;
    }
    txMutation.mutate();
  }

  function submitKYC(e: FormEvent) {
    e.preventDefault();
    if (!agentDid.trim()) {
      showToast("error", t("risk.requiredError"));
      return;
    }
    kycMutation.mutate();
  }

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("risk.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("risk.subtitle")}</p>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <form onSubmit={submitTxCheck} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-100">{t("risk.txCheckTitle")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.agentDid")}
            />
            <input
              value={merchantId}
              onChange={(e) => setMerchantId(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.merchantId")}
            />
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.amount")}
            />
            <input
              value={transactionId}
              onChange={(e) => setTransactionId(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.transactionId")}
            />
            <button
              type="submit"
              disabled={txMutation.isPending}
              className="w-full rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white"
            >
              {t("risk.runTxCheck")}
            </button>
          </div>
          {decision ? (
            <div className="mt-3 rounded-md border border-slate-700 bg-slate-950 p-3 text-xs text-slate-300">
              <p>
                {t("risk.decision")}:{" "}
                <span className={decision.decision === "ALLOW" ? "text-emerald-300" : "text-rose-300"}>
                  {decision.decision}
                </span>
              </p>
              <p>{t("risk.code")}: {decision.code ?? "-"}</p>
              <p>{t("risk.message")}: {decision.message ?? "-"}</p>
            </div>
          ) : null}
        </form>

        <form onSubmit={submitKYC} className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-100">{t("risk.kycTitle")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.agentDid")}
            />
            <input
              value={documentReference}
              onChange={(e) => setDocumentReference(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("risk.documentReference")}
            />
            <button
              type="submit"
              disabled={kycMutation.isPending}
              className="w-full rounded-md bg-indigo-600 px-3 py-2 text-sm font-medium text-white"
            >
              {t("risk.runKycVerify")}
            </button>
          </div>
          {kycResult ? (
            <div className="mt-3 rounded-md border border-slate-700 bg-slate-950 p-3 text-xs text-slate-300">
              <p>{t("risk.status")}: <span className="text-emerald-300">{kycResult.status}</span></p>
              <p>{t("risk.tier")}: {kycResult.tier}</p>
              <p className="break-all">{t("risk.externalReference")}: {kycResult.externalReference}</p>
            </div>
          ) : null}
        </form>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <div className="flex flex-wrap items-end gap-2">
          <div className="min-w-48 flex-1">
            <p className="mb-1 text-xs text-slate-400">{t("risk.auditAgentDid")}</p>
            <input
              value={auditAgentDid}
              onChange={(e) => setAuditAgentDid(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
          </div>
          <div className="min-w-40 flex-1">
            <p className="mb-1 text-xs text-slate-400">{t("risk.auditMerchantId")}</p>
            <input
              value={auditMerchantId}
              onChange={(e) => setAuditMerchantId(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <p className="mb-1 text-xs text-slate-400">{t("risk.limit")}</p>
            <input
              value={auditLimit}
              onChange={(e) => setAuditLimit(e.target.value)}
              className="w-24 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <p className="mb-1 text-xs text-slate-400">{t("risk.offset")}</p>
            <input
              value={auditOffset}
              onChange={(e) => setAuditOffset(e.target.value)}
              className="w-24 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
          </div>
          <button
            type="button"
            onClick={() => setAuditSearchToken((v) => v + 1)}
            className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-100"
          >
            {t("risk.queryAudit")}
          </button>
        </div>

        <h3 className="mt-4 text-sm font-medium text-slate-100">{t("risk.auditTitle")}</h3>
        {auditQuery.isLoading ? <p className="mt-2 text-sm text-slate-400">{t("common.loading")}</p> : null}
        <ul className="mt-2 space-y-2 text-xs text-slate-300">
          {(auditQuery.data ?? []).map((item) => (
            <li
              key={`${item.id}-${item.createdAt}`}
              className="cursor-pointer rounded border border-slate-800 p-2 hover:bg-slate-800/40"
              onClick={() => setSelectedAudit(item)}
            >
              <p>{item.category} · {item.agentDid} · {item.merchantId || "-"}</p>
              <p className="text-slate-500">
                {item.transactionId || "-"} · {new Date(item.createdAt).toLocaleString()}
              </p>
            </li>
          ))}
          {(auditQuery.data ?? []).length === 0 && !auditQuery.isLoading ? (
            <li className="text-slate-500">{t("risk.noAudits")}</li>
          ) : null}
        </ul>
      </div>

      <DetailModal
        open={!!selectedAudit}
        title={t("risk.auditDetail")}
        onClose={() => setSelectedAudit(null)}
      >
        {selectedAudit ? (
          <div className="space-y-2 text-xs text-slate-300">
            <p>{t("risk.category")}: {selectedAudit.category}</p>
            <p>{t("risk.agentDid")}: {selectedAudit.agentDid}</p>
            <p>{t("risk.merchantId")}: {selectedAudit.merchantId || "-"}</p>
            <p>{t("risk.transactionId")}: {selectedAudit.transactionId || "-"}</p>
            <pre className="max-h-72 overflow-auto rounded bg-slate-950 p-2 text-[11px]">
              {JSON.stringify(selectedAudit.detail ?? {}, null, 2)}
            </pre>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
