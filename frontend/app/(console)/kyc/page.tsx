"use client";

import { useMutation } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  getBridgeCustomerStatus,
  getBridgeHostedKycLink,
  listAgents,
  syncBridgeCustomer,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { useQuery } from "@tanstack/react-query";

function formatKycHint(status: string, t: (k: string) => string) {
  const normalized = status.trim().toLowerCase();
  if (normalized === "approved") return t("kyc.hintApproved");
  if (normalized === "incomplete") return t("kyc.hintIncomplete");
  if (normalized === "pending") return t("kyc.hintPending");
  if (normalized === "rejected") return t("kyc.hintRejected");
  if (normalized === "error") return t("kyc.hintError");
  return t("kyc.hintUnknown");
}

export default function KycPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const [agentDid, setAgentDid] = useState("");
  const [status, setStatus] = useState<{
    agentDid: string;
    bridgeCustomerId: string;
    kycStatus: string;
    hostedKycUrl?: string;
    lastError?: string;
    updatedAt: string;
  } | null>(null);
  const [endorsement, setEndorsement] = useState("");

  const agentsQuery = useQuery({
    queryKey: ["agents-for-kyc"],
    queryFn: listAgents,
  });

  const agentOptions = useMemo(() => agentsQuery.data ?? [], [agentsQuery.data]);

  const syncMutation = useMutation({
    mutationFn: () => syncBridgeCustomer(agentDid.trim()),
    onSuccess: (data) => {
      setStatus(data);
      showToast("success", t("kyc.syncSuccess"));
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const queryMutation = useMutation({
    mutationFn: () => getBridgeCustomerStatus(agentDid.trim()),
    onSuccess: (data) => {
      setStatus(data);
      showToast("success", t("kyc.querySuccess"));
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const hostedKycMutation = useMutation({
    mutationFn: () => getBridgeHostedKycLink(agentDid.trim(), endorsement.trim() || undefined),
    onSuccess: (data) => {
      if (data.url) {
        setStatus((prev) => (prev ? { ...prev, hostedKycUrl: data.url } : prev));
        window.open(data.url, "_blank", "noopener,noreferrer");
        showToast("success", t("kyc.hostedLinkOpened"));
      } else {
        showToast("error", t("kyc.hostedLinkMissing"));
      }
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("kyc.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("kyc.subtitle")}</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("kyc.agentBinding")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-[1fr_220px_auto_auto_auto]">
          <input
            list="kyc-agent-options"
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            placeholder={t("kyc.agentDidPlaceholder")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
          />
          <datalist id="kyc-agent-options">
            {agentOptions.map((item) => (
              <option key={item.agentDid} value={item.agentDid}>
                {item.vaAccountId}
              </option>
            ))}
          </datalist>
          <input
            value={endorsement}
            onChange={(e) => setEndorsement(e.target.value)}
            placeholder={t("kyc.endorsementPlaceholder")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
          />
          <button
            type="button"
            onClick={() => {
              if (!agentDid.trim()) {
                showToast("error", t("kyc.agentRequired"));
                return;
              }
              syncMutation.mutate();
            }}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
          >
            {t("kyc.sync")}
          </button>
          <button
            type="button"
            onClick={() => {
              if (!agentDid.trim()) {
                showToast("error", t("kyc.agentRequired"));
                return;
              }
              queryMutation.mutate();
            }}
            className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
          >
            {t("kyc.query")}
          </button>
          <button
            type="button"
            onClick={() => {
              if (!agentDid.trim()) {
                showToast("error", t("kyc.agentRequired"));
                return;
              }
              hostedKycMutation.mutate();
            }}
            className="rounded-md border border-blue-600/60 px-3 py-2 text-sm text-blue-300"
          >
            {t("kyc.openHostedLink")}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("kyc.statusTitle")}</h3>
        {status ? (
          <div className="mt-3 rounded-md border border-slate-700 bg-slate-950/60 p-3 text-xs text-slate-200">
            <p>Agent: {status.agentDid}</p>
            <p>Bridge Customer: {status.bridgeCustomerId || "N/A"}</p>
            <p>KYC: {status.kycStatus}</p>
            {status.hostedKycUrl ? (
              <p className="mt-1 break-all text-sky-300">
                Hosted KYC URL: {status.hostedKycUrl}
              </p>
            ) : null}
            <p className="mt-2 text-slate-300">{formatKycHint(status.kycStatus, t)}</p>
            {status.lastError ? <p className="mt-2 text-rose-300">Error: {status.lastError}</p> : null}
            <p className="mt-2 text-slate-400">Updated: {status.updatedAt}</p>
          </div>
        ) : (
          <p className="mt-3 text-xs text-slate-500">{t("kyc.statusEmpty")}</p>
        )}
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("kyc.nextStepTitle")}</h3>
        <ol className="mt-2 list-decimal space-y-1 pl-5 text-xs text-slate-300">
          <li>{t("kyc.nextStep1")}</li>
          <li>{t("kyc.nextStep2")}</li>
          <li>{t("kyc.nextStep3")}</li>
        </ol>
      </div>
    </section>
  );
}
