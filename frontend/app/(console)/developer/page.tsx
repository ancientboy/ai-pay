"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  createDeveloperApiKey,
  createDeveloperWebhook,
  deleteChannelRoute,
  getRiskConfig,
  getDeveloperWebhookDeliveryStats,
  listAuditLogs,
  listChannelRoutes,
  listDeveloperApiKeys,
  listDeveloperWebhookDeliveries,
  listDeveloperWebhooks,
  replayWebhookDelivery,
  setChannelRoute,
  setRiskConfig,
  listStablecoinConfigs,
  setStablecoinConfig,
  checkStablecoinProviderHealth,
  getBridgeCustomerStatus,
  syncBridgeCustomer,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";

type ApiKeyItem = { id: string; name: string; key: string; createdAt: string };
type WebhookItem = { id: string; url: string; event: string; createdAt: string };
type DeliveryItem = {
  id: number;
  webhookId: string;
  url: string;
  event: string;
  dedupeKey: string;
  payload: Record<string, unknown>;
  status: "PENDING" | "RETRYING" | "SENT" | "DEAD";
  attempts: number;
  maxAttempts: number;
  nextRetryAt: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
};

export default function DeveloperPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [apiKeyName, setApiKeyName] = useState("default");
  const [webhookURL, setWebhookURL] = useState("");
  const [webhookEvent, setWebhookEvent] = useState("payment.settled");
  const [deliveryStatus, setDeliveryStatus] = useState<"" | DeliveryItem["status"]>("");
  const [deliveryEvent, setDeliveryEvent] = useState("");
  const [deliveryWebhookId, setDeliveryWebhookId] = useState("");
  const [deliveryPage, setDeliveryPage] = useState(1);
  const [selectedDelivery, setSelectedDelivery] = useState<DeliveryItem | null>(null);
  const [riskEnabled, setRiskEnabled] = useState<boolean | null>(null);
  const [riskSingleLimit, setRiskSingleLimit] = useState<string | null>(null);
  const [riskBlockedMerchants, setRiskBlockedMerchants] = useState<string | null>(null);
  const [routeMerchantId, setRouteMerchantId] = useState("");
  const [routeMode, setRouteMode] = useState<"SETTLE" | "ASYNC" | "FAIL">("SETTLE");
  const [auditAction, setAuditAction] = useState("");
  const [auditResource, setAuditResource] = useState("");
  const [scCurrency, setScCurrency] = useState<"GUSD" | "USDC" | "USDT">("GUSD");
  const [scProvider, setScProvider] = useState("mock");
  const [scEnabled, setScEnabled] = useState(true);
  const [scChainId, setScChainId] = useState("eth-mainnet");
  const [scRpcUrl, setScRpcUrl] = useState("");
  const [scTokenContract, setScTokenContract] = useState("");
  const [scDecimals, setScDecimals] = useState("6");
  const [scHotWallet, setScHotWallet] = useState("");
  const [scMinConfirmations, setScMinConfirmations] = useState("12");
  const [scRiskThreshold, setScRiskThreshold] = useState("10000");
  const [bridgeAgentDid, setBridgeAgentDid] = useState("");
  const [bridgeStatus, setBridgeStatus] = useState<{ agentDid: string; bridgeCustomerId: string; kycStatus: string; lastError?: string; updatedAt: string } | null>(null);
  const pageSize = 10;

  const apiKeysQuery = useQuery({
    queryKey: ["developer", "apiKeys"],
    queryFn: listDeveloperApiKeys,
  });

  const webhooksQuery = useQuery({
    queryKey: ["developer", "webhooks"],
    queryFn: listDeveloperWebhooks,
  });

  const deliveryStatsQuery = useQuery({
    queryKey: ["developer", "deliveries", "stats"],
    queryFn: getDeveloperWebhookDeliveryStats,
  });

  const deliveriesQuery = useQuery({
    queryKey: ["developer", "deliveries", deliveryStatus, deliveryEvent, deliveryWebhookId, deliveryPage],
    queryFn: () =>
      listDeveloperWebhookDeliveries({
        status: deliveryStatus,
        event: deliveryEvent.trim(),
        webhookId: deliveryWebhookId.trim(),
        limit: pageSize,
        offset: (deliveryPage - 1) * pageSize,
      }),
  });
  const riskConfigQuery = useQuery({
    queryKey: ["developer", "riskConfig"],
    queryFn: getRiskConfig,
  });
  const channelRoutesQuery = useQuery({
    queryKey: ["developer", "channelRoutes"],
    queryFn: listChannelRoutes,
  });
  const stablecoinConfigsQuery = useQuery({
    queryKey: ["developer", "stablecoinConfigs"],
    queryFn: listStablecoinConfigs,
  });
  const auditLogsQuery = useQuery({
    queryKey: ["developer", "auditLogs", auditAction, auditResource],
    queryFn: () => listAuditLogs({ action: auditAction.trim(), resource: auditResource.trim(), limit: 20, offset: 0 }),
  });

  const createApiKeyMutation = useMutation({
    mutationFn: (name: string) => createDeveloperApiKey(name),
    onSuccess: (item) => {
      setApiKeyName("default");
      showToast("success", t("developer.keyCreated"));
      queryClient.setQueryData<ApiKeyItem[]>(["developer", "apiKeys"], (prev) => [
        item,
        ...(prev ?? []),
      ]);
      queryClient.invalidateQueries({ queryKey: ["developer", "apiKeys"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  const createWebhookMutation = useMutation({
    mutationFn: (input: { url: string; event: string }) =>
      createDeveloperWebhook({ url: input.url, event: input.event }),
    onSuccess: (item) => {
      setWebhookURL("");
      setWebhookEvent("payment.settled");
      showToast("success", t("developer.webhookCreated"));
      queryClient.setQueryData<WebhookItem[]>(["developer", "webhooks"], (prev) => [
        item,
        ...(prev ?? []),
      ]);
      queryClient.invalidateQueries({ queryKey: ["developer", "webhooks"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  const replayDeliveryMutation = useMutation({
    mutationFn: (id: number) => replayWebhookDelivery(id),
    onSuccess: () => {
      showToast("success", t("developer.replaySuccess"));
      queryClient.invalidateQueries({ queryKey: ["developer", "deliveries"] });
      queryClient.invalidateQueries({ queryKey: ["developer", "deliveries", "stats"] });
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });
  const setRiskEnabledValue = riskEnabled ?? riskConfigQuery.data?.enabled ?? true;
  const riskSingleLimitValue = riskSingleLimit ?? String(riskConfigQuery.data?.singleAmountLimit ?? 1000);
  const riskBlockedMerchantsValue =
    riskBlockedMerchants ?? (riskConfigQuery.data?.blockedMerchants ?? ["m_risk_block"]).join(",");

  const setRiskConfigMutation = useMutation({
    mutationFn: () =>
      setRiskConfig({
        enabled: setRiskEnabledValue,
        singleAmountLimit: riskSingleLimitValue,
        blockedMerchants: riskBlockedMerchantsValue
          .split(",")
          .map((v) => v.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      showToast("success", t("developer.riskConfigSaved"));
      setRiskEnabled(null);
      setRiskSingleLimit(null);
      setRiskBlockedMerchants(null);
      queryClient.invalidateQueries({ queryKey: ["developer", "riskConfig"] });
      queryClient.invalidateQueries({ queryKey: ["developer", "auditLogs"] });
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });
  const setChannelRouteMutation = useMutation({
    mutationFn: () => setChannelRoute({ merchantId: routeMerchantId.trim(), mode: routeMode }),
    onSuccess: () => {
      setRouteMerchantId("");
      showToast("success", t("developer.channelRouteSaved"));
      queryClient.invalidateQueries({ queryKey: ["developer", "channelRoutes"] });
      queryClient.invalidateQueries({ queryKey: ["developer", "auditLogs"] });
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });
  function applyStablecoinFromList() {
    const selected = (stablecoinConfigsQuery.data ?? []).find((item) => item.currency === scCurrency);
    if (!selected) {
      showToast("error", "stablecoin config not found");
      return;
    }
    setScProvider(selected.provider || "mock");
    setScEnabled(selected.enabled);
    setScChainId(selected.chainId ?? "");
    setScRpcUrl(selected.rpcUrl ?? "");
    setScTokenContract(selected.tokenContract ?? "");
    setScDecimals(String(selected.decimals ?? 6));
    setScHotWallet(selected.hotWallet ?? "");
    setScMinConfirmations(String(selected.minConfirmations ?? 12));
    setScRiskThreshold(String(selected.riskThreshold ?? 10000));
    showToast("info", `loaded ${selected.currency} config`);
  }

  const syncBridgeCustomerMutation = useMutation({
    mutationFn: () => syncBridgeCustomer(bridgeAgentDid.trim()),
    onSuccess: (data) => {
      setBridgeStatus(data);
      showToast("success", `bridge customer synced: ${data.bridgeCustomerId}`);
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const getBridgeCustomerStatusMutation = useMutation({
    mutationFn: () => getBridgeCustomerStatus(bridgeAgentDid.trim()),
    onSuccess: (data) => setBridgeStatus(data),
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const checkStablecoinProviderMutation = useMutation({
    mutationFn: () => checkStablecoinProviderHealth(scProvider),
    onSuccess: () => showToast("success", `provider ${scProvider} healthy`),
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const setStablecoinConfigMutation = useMutation({
    mutationFn: () =>
      setStablecoinConfig({
        currency: scCurrency,
        provider: scProvider,
        enabled: scEnabled,
        chainId: scChainId,
        rpcUrl: scRpcUrl,
        tokenContract: scTokenContract,
        decimals: Number(scDecimals) || 6,
        hotWallet: scHotWallet,
        minConfirmations: Number(scMinConfirmations) || 12,
        riskThreshold: scRiskThreshold,
      }),
    onSuccess: () => {
      showToast("success", "stablecoin config saved");
      queryClient.invalidateQueries({ queryKey: ["developer", "stablecoinConfigs"] });
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });
  const deleteChannelRouteMutation = useMutation({
    mutationFn: (merchantId: string) => deleteChannelRoute(merchantId),
    onSuccess: () => {
      showToast("success", t("developer.channelRouteDeleted"));
      queryClient.invalidateQueries({ queryKey: ["developer", "channelRoutes"] });
      queryClient.invalidateQueries({ queryKey: ["developer", "auditLogs"] });
    },
    onError: (err) => showToast("error", toReadableError(err, locale)),
  });

  const stats = deliveryStatsQuery.data ?? {
    pending: 0,
    retrying: 0,
    sent: 0,
    dead: 0,
    total: 0,
  };

  const selectedRequestID = extractRequestID(selectedDelivery?.payload);
  const requestSearchURL = buildRequestSearchURL(selectedRequestID);

  async function copyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      showToast("success", t("common.copySuccess"));
    } catch {
      showToast("error", t("common.copyFailed"));
    }
  }

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("developer.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("developer.subtitle")}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.apiKeys")}</h3>
          <div className="mt-3 flex gap-2">
            <input
              value={apiKeyName}
              onChange={(e) => setApiKeyName(e.target.value)}
              className="flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.keyName")}
            />
            <button
              onClick={() => {
                if (!apiKeyName.trim()) {
                  showToast("error", t("developer.keyNameRequired"));
                  return;
                }
                createApiKeyMutation.mutate(apiKeyName.trim());
              }}
              disabled={createApiKeyMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              {t("common.create")}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {(apiKeysQuery.data ?? []).map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p>{item.name}</p>
                <p className="font-mono text-slate-400">{item.key}</p>
              </li>
            ))}
            {(apiKeysQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("developer.noKeys")}</li>
            ) : null}
          </ul>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.webhooks")}</h3>
          <div className="mt-3 space-y-2">
            <input
              value={webhookURL}
              onChange={(e) => setWebhookURL(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.webhookUrl")}
            />
            <input
              value={webhookEvent}
              onChange={(e) => setWebhookEvent(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.webhookEvent")}
            />
            <button
              onClick={() => {
                if (!webhookURL.trim()) {
                  showToast("error", t("developer.webhookUrlRequired"));
                  return;
                }
                createWebhookMutation.mutate({
                  url: webhookURL.trim(),
                  event: webhookEvent.trim() || "payment.settled",
                });
              }}
              disabled={createWebhookMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              {t("developer.addWebhook")}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {(webhooksQuery.data ?? []).map((item) => (
              <li key={item.id} className="rounded border border-slate-800 p-2">
                <p className="font-mono text-slate-400">{item.url}</p>
                <p>{item.event}</p>
              </li>
            ))}
            {(webhooksQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">{t("developer.noWebhooks")}</li>
            ) : null}
          </ul>
        </div>
      </div>

      <div className="space-y-4 rounded-xl border border-slate-800 bg-slate-900 p-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-medium text-slate-200">{t("developer.deliveryCenter")}</h3>
            <p className="mt-1 text-xs text-slate-400">{t("developer.deliverySubtitle")}</p>
          </div>
          <button
            onClick={() => {
              deliveriesQuery.refetch();
              deliveryStatsQuery.refetch();
            }}
            className="rounded-md border border-slate-700 px-3 py-1.5 text-xs text-slate-200"
          >
            {t("developer.refresh")}
          </button>
        </div>

        <div className="grid gap-2 sm:grid-cols-5">
          {([
            ["pending", stats.pending, "PENDING"],
            ["retrying", stats.retrying, "RETRYING"],
            ["sent", stats.sent, "SENT"],
            ["dead", stats.dead, "DEAD"],
            ["total", stats.total, "TOTAL"],
          ] as const).map(([key, value, status]) => (
            <div key={key} className="rounded-md border border-slate-800 bg-slate-950 p-3">
              <p className="text-xs text-slate-400">
                {status === "TOTAL" ? t("developer.total") : formatStatus(locale, status)}
              </p>
              <p className="mt-1 text-lg font-semibold text-slate-100">{value}</p>
            </div>
          ))}
        </div>

        <div className="grid gap-2 md:grid-cols-3">
          <select
            value={deliveryStatus}
            onChange={(e) => {
              setDeliveryStatus(e.target.value as "" | DeliveryItem["status"]);
              setDeliveryPage(1);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
          >
            <option value="">{t("developer.allStatus")}</option>
            <option value="PENDING">{formatStatus(locale, "PENDING")}</option>
            <option value="RETRYING">{formatStatus(locale, "RETRYING")}</option>
            <option value="SENT">{formatStatus(locale, "SENT")}</option>
            <option value="DEAD">{formatStatus(locale, "DEAD")}</option>
          </select>
          <input
            value={deliveryEvent}
            onChange={(e) => {
              setDeliveryEvent(e.target.value);
              setDeliveryPage(1);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.eventFilter")}
          />
          <input
            value={deliveryWebhookId}
            onChange={(e) => {
              setDeliveryWebhookId(e.target.value);
              setDeliveryPage(1);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.webhookIdFilter")}
          />
        </div>
        <div className="grid gap-2 md:grid-cols-2">
          <button
            onClick={() => deliveriesQuery.refetch()}
            className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
          >
            {t("developer.refresh")}
          </button>
          <div className="flex items-center justify-end gap-2 text-xs text-slate-300">
            <button
              onClick={() => setDeliveryPage((prev) => Math.max(1, prev - 1))}
              disabled={deliveryPage === 1}
              className="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            >
              {t("transactions.prev")}
            </button>
            <span>
              {t("transactions.page")} {deliveryPage}
            </span>
            <button
              onClick={() => {
                if ((deliveriesQuery.data ?? []).length >= pageSize) {
                  setDeliveryPage((prev) => prev + 1);
                }
              }}
              disabled={(deliveriesQuery.data ?? []).length < pageSize}
              className="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            >
              {t("transactions.next")}
            </button>
          </div>
        </div>

        <ul className="space-y-2 text-xs text-slate-300">
          {(deliveriesQuery.data ?? []).map((item) => (
            <li key={item.id} className="rounded border border-slate-800 p-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="font-mono text-slate-400">#{item.id} · {item.event}</p>
                <span className="rounded border border-slate-700 px-2 py-0.5 text-[11px]">
                  {formatStatus(locale, item.status)}
                </span>
              </div>
              <p className="mt-1 truncate font-mono text-[11px] text-slate-500">{item.url}</p>
              <p className="mt-1 text-[11px] text-slate-400">
                {t("developer.attempts")}: {item.attempts}/{item.maxAttempts} · {t("developer.nextRetryAt")}:{" "}
                {item.nextRetryAt ? new Date(item.nextRetryAt).toLocaleString() : "-"}
              </p>
              <p className="mt-1 break-all text-[11px] text-slate-400">
                {t("developer.dedupeKey")}: {item.dedupeKey}
              </p>
              {item.lastError ? (
                <p className="mt-1 break-all text-[11px] text-red-300">
                  {t("developer.lastError")}: {item.lastError}
                </p>
              ) : null}
              <details className="mt-2">
                <summary className="cursor-pointer text-[11px] text-slate-400">
                  {t("developer.payload")}
                </summary>
                <pre className="mt-1 overflow-x-auto rounded bg-slate-950 p-2 text-[11px] text-slate-300">
                  {JSON.stringify(item.payload, null, 2)}
                </pre>
              </details>
              {item.status === "DEAD" ? (
                <button
                  onClick={() => replayDeliveryMutation.mutate(item.id)}
                  disabled={replayDeliveryMutation.isPending}
                  className="mt-2 rounded-md bg-amber-600 px-2 py-1 text-[11px] text-white"
                >
                  {t("developer.replay")}
                </button>
              ) : null}
              <button
                onClick={() => setSelectedDelivery(item)}
                className="mt-2 ml-2 rounded-md border border-slate-700 px-2 py-1 text-[11px] text-slate-200"
              >
                {t("developer.viewDetail")}
              </button>
            </li>
          ))}
          {(deliveriesQuery.data ?? []).length === 0 ? (
            <li className="text-slate-500">{t("developer.noDeliveries")}</li>
          ) : null}
        </ul>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.riskConfig")}</h3>
          <label className="mt-3 flex items-center gap-2 text-sm text-slate-300">
            <input type="checkbox" checked={setRiskEnabledValue} onChange={(e) => setRiskEnabled(e.target.checked)} />
            {t("developer.riskEnabled")}
          </label>
          <input
            value={riskSingleLimitValue}
            onChange={(e) => setRiskSingleLimit(e.target.value)}
            className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.riskSingleLimit")}
          />
          <input
            value={riskBlockedMerchantsValue}
            onChange={(e) => setRiskBlockedMerchants(e.target.value)}
            className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.riskBlockedMerchants")}
          />
          <button
            onClick={() => setRiskConfigMutation.mutate()}
            disabled={setRiskConfigMutation.isPending}
            className="mt-2 rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
          >
            {t("common.save")}
          </button>
        </div>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("developer.channelRoutes")}</h3>
          <div className="mt-2 space-y-2">
            <input
              value={routeMerchantId}
              onChange={(e) => setRouteMerchantId(e.target.value)}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              placeholder={t("developer.channelMerchantId")}
            />
            <select
              value={routeMode}
              onChange={(e) => setRouteMode(e.target.value as "SETTLE" | "ASYNC" | "FAIL")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            >
              <option value="SETTLE">{t("developer.channelSettle")}</option>
              <option value="ASYNC">{t("developer.channelAsync")}</option>
              <option value="FAIL">{t("developer.channelFail")}</option>
            </select>
            <button
              onClick={() => {
                if (!routeMerchantId.trim()) {
                  showToast("error", t("developer.channelMerchantId"));
                  return;
                }
                setChannelRouteMutation.mutate();
              }}
              disabled={setChannelRouteMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white"
            >
              {t("common.save")}
            </button>
          </div>
          <ul className="mt-3 space-y-2 text-xs text-slate-300">
            {(channelRoutesQuery.data ?? []).map((item) => (
              <li key={item.merchantId} className="flex items-center justify-between rounded border border-slate-800 p-2">
                <span>{item.merchantId} · {item.mode}</span>
                <button
                  onClick={() => deleteChannelRouteMutation.mutate(item.merchantId)}
                  className="rounded border border-slate-700 px-2 py-1"
                >
                  {t("common.close")}
                </button>
              </li>
            ))}
          </ul>
        </div>
      </div>


      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">Stablecoin Config</h3>
        <div className="mt-2 grid gap-2 md:grid-cols-3">
          <select value={scCurrency} onChange={(e)=>setScCurrency(e.target.value as "GUSD" | "USDC" | "USDT")} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm">
            <option value="GUSD">GUSD</option><option value="USDC">USDC</option><option value="USDT">USDT</option>
          </select>
          <input value={scProvider} onChange={(e)=>setScProvider(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="provider (mock/bridge/stripe)" />
          <input value={scChainId} onChange={(e)=>setScChainId(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="chainId" />
          <input value={scRpcUrl} onChange={(e)=>setScRpcUrl(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="rpcUrl" />
          <input value={scTokenContract} onChange={(e)=>setScTokenContract(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="tokenContract" />
          <input value={scDecimals} onChange={(e)=>setScDecimals(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="decimals" />
          <input value={scHotWallet} onChange={(e)=>setScHotWallet(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="hotWallet" />
          <input value={scMinConfirmations} onChange={(e)=>setScMinConfirmations(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="minConfirmations" />
          <input value={scRiskThreshold} onChange={(e)=>setScRiskThreshold(e.target.value)} className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="riskThreshold" />
          <label className="flex items-center gap-2 text-sm text-slate-300"><input type="checkbox" checked={scEnabled} onChange={(e)=>setScEnabled(e.target.checked)} />enabled</label>
        </div>
        <div className="mt-3 flex gap-2">
          <button onClick={applyStablecoinFromList} className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200">Load Selected</button>
          <button onClick={()=>checkStablecoinProviderMutation.mutate()} className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200">Check Provider</button>
          <button onClick={()=>setStablecoinConfigMutation.mutate()} className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white">Save Stablecoin Config</button>
        </div>
        <ul className="mt-3 space-y-2 text-xs text-slate-300">
          {(stablecoinConfigsQuery.data ?? []).map((item)=> (
            <li key={item.currency} className="rounded border border-slate-800 p-2">{item.currency} · {item.provider || "mock"} · {item.chainId || "-"} · conf={item.minConfirmations} · {item.enabled ? "enabled" : "disabled"}</li>
          ))}
        </ul>
      </div>


      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">Bridge Customer/KYC Sync</h3>
        <div className="mt-2 flex gap-2">
          <input value={bridgeAgentDid} onChange={(e)=>setBridgeAgentDid(e.target.value)} className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm" placeholder="agentDid" />
          <button onClick={()=>syncBridgeCustomerMutation.mutate()} className="rounded-md bg-blue-600 px-3 py-2 text-sm text-white">Sync</button>
          <button onClick={()=>getBridgeCustomerStatusMutation.mutate()} className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200">Status</button>
        </div>
        {bridgeStatus ? (
          <div className="mt-3 rounded border border-slate-800 p-2 text-xs text-slate-300">
            <p>agent: {bridgeStatus.agentDid}</p>
            <p>customer: {bridgeStatus.bridgeCustomerId}</p>
            <p>kyc: {bridgeStatus.kycStatus}</p>
            <p>error: {bridgeStatus.lastError || '-'}</p>
            <p>updated: {bridgeStatus.updatedAt}</p>
          </div>
        ) : null}
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("developer.auditLogs")}</h3>
        <div className="mt-2 grid gap-2 md:grid-cols-3">
          <input
            value={auditAction}
            onChange={(e) => setAuditAction(e.target.value)}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.auditActionFilter")}
          />
          <input
            value={auditResource}
            onChange={(e) => setAuditResource(e.target.value)}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            placeholder={t("developer.auditResourceFilter")}
          />
          <button onClick={() => auditLogsQuery.refetch()} className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200">
            {t("developer.refresh")}
          </button>
        </div>
        <div className="mt-2 flex flex-wrap gap-2 text-xs">
          <button
            onClick={() => {
              setAuditAction("risk_config_set");
              setAuditResource("global");
            }}
            className="rounded border border-slate-700 px-2 py-1 text-slate-200"
          >
            Risk Config Audit
          </button>
          <button
            onClick={() => {
              setAuditAction("channel_route_set");
              setAuditResource("");
            }}
            className="rounded border border-slate-700 px-2 py-1 text-slate-200"
          >
            Channel Route Audit
          </button>
          <button
            onClick={() => {
              setAuditAction("");
              setAuditResource("");
            }}
            className="rounded border border-slate-700 px-2 py-1 text-slate-200"
          >
            Clear Audit Filters
          </button>
        </div>
        <ul className="mt-3 space-y-2 text-xs text-slate-300">
          {(auditLogsQuery.data ?? []).map((item) => (
            <li key={item.id} className="rounded border border-slate-800 p-2">
              <p>#{item.id} · {item.action} · {item.resource}</p>
              <p className="text-slate-500">{item.actor} / {item.role} / {item.requestId}</p>
              <pre className="mt-1 overflow-x-auto rounded bg-slate-950 p-2 text-[11px]">{JSON.stringify(item.detail ?? {}, null, 2)}</pre>
            </li>
          ))}
          {(auditLogsQuery.data ?? []).length === 0 ? <li className="text-slate-500">{t("developer.noAuditLogs")}</li> : null}
        </ul>
      </div>
      <DetailModal
        open={!!selectedDelivery}
        title={t("developer.deliveryDetail")}
        onClose={() => setSelectedDelivery(null)}
      >
        {selectedDelivery ? (
          <div className="space-y-2 text-xs">
            <p className="font-mono text-slate-300">#{selectedDelivery.id}</p>
            <p>{t("developer.webhookIdLabel")}: {selectedDelivery.webhookId}</p>
            <p className="break-all font-mono text-slate-400">{selectedDelivery.url}</p>
            <p>{t("transactions.status")}: {formatStatus(locale, selectedDelivery.status)}</p>
            <p>
              {t("developer.attempts")}: {selectedDelivery.attempts}/{selectedDelivery.maxAttempts}
            </p>
            <p>
              {t("common.requestId")}: {selectedRequestID ?? t("common.notAvailable")}
            </p>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => copyText(buildReplayCurl(selectedDelivery))}
                className="rounded border border-slate-700 px-2 py-1"
              >
                {t("developer.copyReplayCurl")}
              </button>
              <button
                onClick={() =>
                  copyText(JSON.stringify(selectedDelivery.payload ?? {}, null, 2))
                }
                className="rounded border border-slate-700 px-2 py-1"
              >
                {t("developer.copyPayload")}
              </button>
              {selectedRequestID ? (
                <button
                  onClick={() => copyText(selectedRequestID)}
                  className="rounded border border-slate-700 px-2 py-1"
                >
                  {t("developer.copyRequestId")}
                </button>
              ) : null}
              {requestSearchURL ? (
                <a
                  href={requestSearchURL}
                  target="_blank"
                  rel="noreferrer"
                  className="rounded border border-blue-600/60 px-2 py-1 text-blue-300"
                >
                  {t("developer.searchRequestId")}
                </a>
              ) : null}
            </div>
            <pre className="max-h-72 overflow-auto rounded bg-slate-950 p-2 text-[11px] text-slate-300">
              {JSON.stringify(selectedDelivery.payload ?? {}, null, 2)}
            </pre>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}

function extractRequestID(payload?: Record<string, unknown> | null): string | null {
  if (!payload) {
    return null;
  }
  const direct = payload.requestId ?? payload.request_id;
  if (typeof direct === "string" && direct.trim()) {
    return direct.trim();
  }
  const meta = payload.meta;
  if (meta && typeof meta === "object") {
    const maybe = (meta as Record<string, unknown>).requestId;
    if (typeof maybe === "string" && maybe.trim()) {
      return maybe.trim();
    }
  }
  return null;
}

function buildRequestSearchURL(requestID: string | null): string | null {
  if (!requestID) {
    return null;
  }
  const template = process.env.NEXT_PUBLIC_LOG_SEARCH_URL_TEMPLATE;
  if (!template || !template.trim()) {
    return null;
  }
  return template.replaceAll("{requestId}", encodeURIComponent(requestID));
}

function buildReplayCurl(item: DeliveryItem): string {
  const payload = JSON.stringify(item.payload ?? {}, null, 2);
  const escapedPayload = payload.replace(/'/g, "'\"'\"'");
  return [
    `curl -X POST '${item.url}' \\`,
    "  -H 'Content-Type: application/json' \\",
    `  -H 'X-Webhook-Event: ${item.event}' \\`,
    `  -H 'X-Webhook-Delivery-Id: ${item.id}' \\`,
    "  -d '" + escapedPayload + "'",
  ].join("\n");
}
