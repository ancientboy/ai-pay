"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  bindProviderAccount,
  createPaymentIntent,
  executePaymentIntent,
  getPaymentIntentStatus,
  listAgents,
  listProviderAccounts,
  type PaymentIntentExecutionRecord,
  type PaymentIntentRecord,
  type ProviderAccountBinding,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";

export default function OrchestrationPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();

  const [platformVaAccountId, setPlatformVaAccountId] = useState("");
  const [provider, setProvider] = useState("bridge");
  const [providerCustomerId, setProviderCustomerId] = useState("");
  const [providerAccountId, setProviderAccountId] = useState("");
  const [currency, setCurrency] = useState<"GUSD" | "USDC" | "USDT">("USDC");
  const [selectedAgentDid, setSelectedAgentDid] = useState("");

  const [intentVaAccountId, setIntentVaAccountId] = useState("");
  const [agentDid, setAgentDid] = useState("");
  const [merchantId, setMerchantId] = useState("");
  const [intentAmount, setIntentAmount] = useState("10");
  const [intentCurrency, setIntentCurrency] = useState<"GUSD" | "USDC" | "USDT">("USDC");
  const [intentMetadata, setIntentMetadata] = useState("{}");

  const [executionIntentId, setExecutionIntentId] = useState("");
  const [executionProvider, setExecutionProvider] = useState("bridge");
  const [statusIntentId, setStatusIntentId] = useState("");

  const [intentStatus, setIntentStatus] = useState<{
    intent: PaymentIntentRecord;
    executions: PaymentIntentExecutionRecord[];
  } | null>(null);

  const providerAccountsQuery = useQuery({
    queryKey: ["orchestration", "provider-accounts", platformVaAccountId],
    queryFn: () => listProviderAccounts(platformVaAccountId.trim()),
    enabled: platformVaAccountId.trim().length > 0,
  });
  const agentsQuery = useQuery({
    queryKey: ["orchestration", "agents"],
    queryFn: listAgents,
  });

  const bindMutation = useMutation({
    mutationFn: () =>
      bindProviderAccount({
        platformVaAccountId: platformVaAccountId.trim(),
        provider: provider.trim().toLowerCase(),
        providerCustomerId: providerCustomerId.trim(),
        providerAccountId: providerAccountId.trim(),
        currency,
        metadata: "{}",
      }),
    onSuccess: () => {
      showToast("success", t("orchestration.bindSuccess"));
      providerAccountsQuery.refetch();
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const createIntentMutation = useMutation({
    mutationFn: () =>
      createPaymentIntent({
        platformVaAccountId: intentVaAccountId.trim(),
        agentDid: agentDid.trim(),
        merchantId: merchantId.trim(),
        currency: intentCurrency,
        amount: intentAmount.trim(),
        metadata: intentMetadata.trim() || "{}",
      }),
    onSuccess: (data) => {
      showToast("success", t("orchestration.intentCreateSuccess"));
      setExecutionIntentId(data.intentId);
      setStatusIntentId(data.intentId);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const executeIntentMutation = useMutation({
    mutationFn: () =>
      executePaymentIntent({
        intentId: executionIntentId.trim(),
        provider: executionProvider.trim().toLowerCase(),
      }),
    onSuccess: () => {
      showToast("success", t("orchestration.intentExecuteSuccess"));
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const queryStatusMutation = useMutation({
    mutationFn: () => getPaymentIntentStatus(statusIntentId.trim()),
    onSuccess: (data) => {
      setIntentStatus(data);
      showToast("success", t("orchestration.intentStatusSuccess"));
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("orchestration.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("orchestration.subtitle")}</p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("orchestration.quickFillTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-[1fr_auto]">
          <select
            value={selectedAgentDid}
            onChange={(e) => {
              const nextDid = e.target.value;
              setSelectedAgentDid(nextDid);
              if (!nextDid.trim()) {
                return;
              }
              const selected = (agentsQuery.data ?? []).find((item) => item.agentDid === nextDid.trim());
              if (!selected) {
                return;
              }
              setAgentDid(selected.agentDid);
              setIntentVaAccountId(selected.vaAccountId);
              setPlatformVaAccountId(selected.vaAccountId);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            <option value="">{t("orchestration.selectAgent")}</option>
            {(agentsQuery.data ?? []).map((item) => (
              <option key={item.agentDid} value={item.agentDid}>
                {item.agentDid} · {item.vaAccountId}
              </option>
            ))}
          </select>
          <button
            onClick={() => agentsQuery.refetch()}
            className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
          >
            {t("orchestration.refreshAgents")}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("orchestration.bindTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <input
            value={platformVaAccountId}
            onChange={(e) => setPlatformVaAccountId(e.target.value)}
            placeholder={t("orchestration.platformVaAccountId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            placeholder={t("orchestration.provider")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={providerCustomerId}
            onChange={(e) => setProviderCustomerId(e.target.value)}
            placeholder={t("orchestration.providerCustomerId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={providerAccountId}
            onChange={(e) => setProviderAccountId(e.target.value)}
            placeholder={t("orchestration.providerAccountId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value as "GUSD" | "USDC" | "USDT")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            <option value="GUSD">GUSD</option>
            <option value="USDC">USDC</option>
            <option value="USDT">USDT</option>
          </select>
          <button
            onClick={() => bindMutation.mutate()}
            disabled={bindMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white disabled:opacity-60"
          >
            {t("orchestration.bindAction")}
          </button>
        </div>

        <div className="mt-3">
          <button
            onClick={() =>
              providerAccountsQuery.refetch().then((res) => {
                const first = (res.data ?? [])[0];
                if (first && !executionProvider.trim()) {
                  setExecutionProvider(first.provider);
                }
              })
            }
            disabled={!platformVaAccountId.trim()}
            className="rounded-md border border-slate-700 px-3 py-1 text-xs text-slate-200 disabled:opacity-60"
          >
            {t("orchestration.refreshBindings")}
          </button>
          <ul className="mt-2 space-y-2 text-xs text-slate-300">
            {(providerAccountsQuery.data ?? []).map((item: ProviderAccountBinding) => (
              <li key={`${item.provider}-${item.providerAccountId}-${item.createdAt}`} className="rounded border border-slate-800 p-2">
                {item.provider} · {item.currency} · {item.providerAccountId} · {item.status}
              </li>
            ))}
            {(providerAccountsQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("orchestration.noBindings")}</li>
            ) : null}
          </ul>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("orchestration.intentTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <input
            value={intentVaAccountId}
            onChange={(e) => setIntentVaAccountId(e.target.value)}
            placeholder={t("orchestration.platformVaAccountId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            placeholder={t("orchestration.agentDid")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={merchantId}
            onChange={(e) => setMerchantId(e.target.value)}
            placeholder={t("orchestration.merchantId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <select
            value={intentCurrency}
            onChange={(e) => setIntentCurrency(e.target.value as "GUSD" | "USDC" | "USDT")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            <option value="GUSD">GUSD</option>
            <option value="USDC">USDC</option>
            <option value="USDT">USDT</option>
          </select>
          <input
            value={intentAmount}
            onChange={(e) => setIntentAmount(e.target.value)}
            placeholder={t("orchestration.amount")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <button
            onClick={() => createIntentMutation.mutate()}
            disabled={createIntentMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white disabled:opacity-60"
          >
            {t("orchestration.intentCreateAction")}
          </button>
        </div>
        <textarea
          value={intentMetadata}
          onChange={(e) => setIntentMetadata(e.target.value)}
          className="mt-2 min-h-[80px] w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          placeholder={t("orchestration.metadata")}
        />
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("orchestration.executeTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          <input
            value={executionIntentId}
            onChange={(e) => setExecutionIntentId(e.target.value)}
            placeholder={t("orchestration.intentId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <input
            value={executionProvider}
            onChange={(e) => setExecutionProvider(e.target.value)}
            placeholder={t("orchestration.provider")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <button
            onClick={() => executeIntentMutation.mutate()}
            disabled={executeIntentMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white disabled:opacity-60"
          >
            {t("orchestration.intentExecuteAction")}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("orchestration.statusTitle")}</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-[1fr_auto]">
          <input
            value={statusIntentId}
            onChange={(e) => setStatusIntentId(e.target.value)}
            placeholder={t("orchestration.intentId")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          />
          <button
            onClick={() => queryStatusMutation.mutate()}
            disabled={queryStatusMutation.isPending}
            className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 disabled:opacity-60"
          >
            {t("orchestration.intentStatusAction")}
          </button>
        </div>
        {intentStatus ? (
          <div className="mt-3 rounded border border-slate-800 p-3 text-xs text-slate-200">
            <p>
              {t("orchestration.intentId")}: {intentStatus.intent.intentId}
            </p>
            <p>
              {t("orchestration.provider")}: {intentStatus.intent.selectedProvider || "-"}
            </p>
            <p>
              {t("orchestration.currency")}: {intentStatus.intent.currency}
            </p>
            <p>
              {t("orchestration.amount")}: {intentStatus.intent.amount}
            </p>
            <p>
              {t("orchestration.status")}: {intentStatus.intent.status}
            </p>
            <div className="mt-2">
              <p className="text-slate-400">{t("orchestration.executions")}:</p>
              <ul className="mt-1 space-y-1">
                {intentStatus.executions.map((exec) => (
                  <li key={`${exec.id}-${exec.providerTxnId}`} className="rounded border border-slate-800 bg-slate-950 p-2">
                    {exec.provider} · {exec.status} · txn={exec.providerTxnId || "-"}
                  </li>
                ))}
                {intentStatus.executions.length === 0 ? (
                  <li className="text-slate-500">{t("orchestration.noExecutions")}</li>
                ) : null}
              </ul>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}
