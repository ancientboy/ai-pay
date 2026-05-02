"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  createBillingIntent,
  createBridgeKYCLink,
  createBridgeVirtualAccount,
  exportBillingReconciliationCsv,
  getAuthProfile,
  getBridgeVACountries,
  getBillingCapabilities,
  getBillingReconciliation,
  getBillingSubscription,
  syncBillingCheckout,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { getValidationSchemas } from "@/lib/validation";
import { canUseFeature } from "@/lib/rbac";

const PLANS = [
  { code: "starter", labelKey: "billing.planStarter" },
  { code: "growth", labelKey: "billing.planGrowth" },
] as const;

export default function BillingPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const searchParams = useSearchParams();
  const { amountSchema } = useMemo(() => getValidationSchemas(locale), [locale]);

  const [amount, setAmount] = useState("49.9");
  const [currency, setCurrency] = useState("USD");
  /** Shown in plan selector; null means fall back to URL `plan=` then default starter. */
  const [manualPlanCode, setManualPlanCode] = useState<"starter" | "growth" | null>(null);
  const [checkoutType, setCheckoutType] = useState<"subscription" | "payment_link">("subscription");
  const [vaAccountId, setVaAccountId] = useState("");
  const [customerHint, setCustomerHint] = useState("");
  const [reconCheckoutType, setReconCheckoutType] = useState(
    () => searchParams.get("checkoutType") ?? "",
  );
  const [reconStatus, setReconStatus] = useState(() => searchParams.get("status") ?? "");
  const [reconVA, setReconVA] = useState(() => searchParams.get("vaAccountId") ?? "");
  const [reconAnomalyOnly, setReconAnomalyOnly] = useState(
    () => searchParams.get("anomalyOnly") === "true",
  );
  const [reconOffset, setReconOffset] = useState(() => {
    const raw = searchParams.get("offset");
    if (!raw) {
      return 0;
    }
    const n = Number.parseInt(raw, 10);
    return Number.isFinite(n) && n >= 0 ? n : 0;
  });
  const reconLimit = 20;
  const [checkout, setCheckout] = useState<{
    checkoutId: string;
    provider: string;
    paymentRail: string;
    currency: string;
    checkoutURL: string;
    checkoutMode?: string;
  } | null>(null);
  const [errorMessage, setErrorMessage] = useState("");
  const [sessionRole, setSessionRole] = useState<"admin" | "operator" | "readonly">("operator");
  const [tenantId, setTenantId] = useState("default");
  const [subscriptionPlan, setSubscriptionPlan] = useState<"starter" | "growth" | "enterprise">("starter");
  const [planCapabilities, setPlanCapabilities] = useState<string[]>([]);
  const [bridgeKycName, setBridgeKycName] = useState("Demo User");
  const [bridgeKycEmail, setBridgeKycEmail] = useState("demo@example.com");
  const [bridgeCustomerId, setBridgeCustomerId] = useState("");
  const [bridgeWalletAddress, setBridgeWalletAddress] = useState("0xdeadbeef");
  const [bridgeCreateVAResult, setBridgeCreateVAResult] = useState("");

  const capabilitiesQuery = useQuery({
    queryKey: ["billing-capabilities"],
    queryFn: getBillingCapabilities,
  });

  const subscriptionQuery = useQuery({
    queryKey: ["billing-subscription"],
    queryFn: getBillingSubscription,
  });
  const bridgeCountriesQuery = useQuery({
    queryKey: ["bridge-va-countries"],
    queryFn: getBridgeVACountries,
  });

  useEffect(() => {
    void (async () => {
      try {
        const profile = await getAuthProfile();
        setSessionRole(profile.role);
        setTenantId(profile.tenantId);
        setSubscriptionPlan(profile.subscriptionPlan);
        setPlanCapabilities(profile.planCapabilities ?? []);
      } catch {
        // keep defaults on profile read failure
      }
    })();
  }, []);

  const planFromUrl = useMemo(() => {
    const p = searchParams.get("plan")?.trim().toLowerCase();
    if (p === "starter" || p === "growth") {
      return p;
    }
    return null;
  }, [searchParams]);

  const effectivePlanCode = manualPlanCode ?? planFromUrl ?? "starter";

  const reconciliationQuery = useQuery({
    queryKey: [
      "billing-reconciliation",
      reconCheckoutType,
      reconStatus,
      reconVA,
      reconAnomalyOnly,
      reconOffset,
    ],
    queryFn: () =>
      getBillingReconciliation({
        limit: reconLimit,
        offset: reconOffset,
        checkoutType: reconCheckoutType || undefined,
        status: reconStatus || undefined,
        vaAccountId: reconVA.trim() || undefined,
        anomalyOnly: reconAnomalyOnly || undefined,
      }),
  });

  useEffect(() => {
    const checkoutParam = searchParams.get("checkout");
    const sessionId = searchParams.get("session_id");
    if (checkoutParam !== "success" || !sessionId?.trim()) {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const data = await syncBillingCheckout(sessionId.trim());
        if (cancelled) {
          return;
        }
        await subscriptionQuery.refetch();
        if (data.subscription) {
          showToast("success", t("billing.syncSuccess"));
        }
      } catch (e) {
        if (!cancelled) {
          showToast("error", toReadableError(e, locale));
        }
      }
      window.history.replaceState({}, "", "/billing");
    })();
    return () => {
      cancelled = true;
    };
  }, [searchParams, subscriptionQuery, showToast, t, locale]);

  const checkoutMutation = useMutation({
    mutationFn: () => {
      if (!canUseFeature(sessionRole, subscriptionPlan, "billing.checkout.create")) {
        throw new Error(t("billing.featureCheckoutBlocked"));
      }
      return createBillingIntent({
        currency,
        provider: currency === "USD" ? "stripe" : "bridge",
        paymentRail: currency === "USD" ? "fiat" : "stablecoin",
        checkoutType,
        planCode: effectivePlanCode,
        amount,
        vaAccountId: checkoutType === "payment_link" ? vaAccountId.trim() || undefined : undefined,
        customerIdHint: customerHint.trim() || undefined,
      });
    },
    onSuccess: (data) => {
      setCheckout(data);
      setErrorMessage("");
      showToast("success", t("common.success"));
    },
    onError: (err) => {
      const msg = toReadableError(err, locale);
      setErrorMessage(msg);
      showToast("error", msg);
    },
  });

  const caps = capabilitiesQuery.data?.capabilities ?? [];
  const fiatCap = caps.find((c) => c.methods.includes("fiat_card"));
  const stableCap = caps.find((c) => c.methods.includes("stablecoin_transfer"));
  const canFiat = !!fiatCap;
  const canStablecoin = !!stableCap;

  const stripeReady = capabilitiesQuery.data?.stripeCheckoutConfigured === true;
  const bridgeConfigured = capabilitiesQuery.data?.bridgeConfigured === true;
  const sub = subscriptionQuery.data?.subscription ?? null;
  const reconItems = reconciliationQuery.data?.items ?? [];
  const reconMeta = reconciliationQuery.data?.meta;
  const canPrev = (reconMeta?.offset ?? 0) > 0;
  const canNext = (reconMeta?.offset ?? 0) + (reconMeta?.count ?? 0) < (reconMeta?.total ?? 0);
  const copyContextMutation = useMutation({
    mutationFn: async () => {
      const contextPayload = {
        page: "/billing",
        filters: {
          checkoutType: reconCheckoutType || "all",
          status: reconStatus || "all",
          vaAccountId: reconVA || "",
          anomalyOnly: reconAnomalyOnly,
          offset: reconOffset,
          limit: reconLimit,
        },
        summary: {
          count: reconMeta?.count ?? reconItems.length,
          total: reconMeta?.total ?? reconItems.length,
        },
        sampleItems: reconItems.slice(0, 5).map((item) => ({
          providerSessionId: item.providerSessionId,
          status: item.status,
          checkoutType: item.checkoutType,
          vaAccountId: item.vaAccountId,
          amountMinor: item.amountMinor,
          currency: item.currency,
          anomaly: item.anomaly ?? false,
          createdAt: item.createdAt,
        })),
      };
      const text = JSON.stringify(contextPayload, null, 2);
      if (!navigator.clipboard?.writeText) {
        throw new Error("clipboard unavailable");
      }
      await navigator.clipboard.writeText(text);
    },
    onSuccess: () => showToast("success", t("common.copySuccess")),
    onError: () => showToast("error", t("common.copyFailed")),
  });

  const exportMutation = useMutation({
    mutationFn: () =>
      exportBillingReconciliationCsv({
        limit: reconLimit,
        offset: reconOffset,
        checkoutType: reconCheckoutType || undefined,
        status: reconStatus || undefined,
        vaAccountId: reconVA.trim() || undefined,
        anomalyOnly: reconAnomalyOnly || undefined,
      }),
    onSuccess: (csv) => {
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
      const href = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = href;
      a.download = `billing_reconciliation_${Date.now()}.csv`;
      a.click();
      URL.revokeObjectURL(href);
      showToast("success", t("common.success"));
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });
  const bridgeKYCMutation = useMutation({
    mutationFn: () =>
      createBridgeKYCLink({
        fullName: bridgeKycName.trim(),
        email: bridgeKycEmail.trim(),
        type: "individual",
      }),
    onSuccess: (res) => {
      setBridgeCustomerId(res.result.customerId ?? "");
      setErrorMessage("");
      showToast("success", `${t("billing.bridgeKycCreated")}: ${res.mode}`);
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });
  const bridgeCreateVAMutation = useMutation({
    mutationFn: () =>
      createBridgeVirtualAccount({
        customerId: bridgeCustomerId.trim(),
        sourceCurrency: "usd",
        destinationCurrency: "usdc",
        paymentRail: "base",
        address: bridgeWalletAddress.trim(),
      }),
    onSuccess: (res) => {
      setBridgeCreateVAResult(JSON.stringify(res.result, null, 2));
      showToast("success", `${t("billing.bridgeVACreated")}: ${res.mode}`);
    },
    onError: (err) => {
      showToast("error", toReadableError(err, locale));
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("billing.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("billing.subtitle")}</p>
        <p className="mt-1 text-xs text-slate-500">
          {t("billing.tenantPlanHint")
            .replace("{tenant}", tenantId)
            .replace("{plan}", subscriptionPlan)
            .replace("{role}", sessionRole)}
        </p>
        <p className="mt-1 text-xs text-slate-500">
          {t("billing.planCapabilitiesHint").replace(
            "{caps}",
            planCapabilities.length > 0 ? planCapabilities.join(", ") : "none",
          )}
        </p>
      </div>

      <article className="rounded-xl border border-cyan-800/60 bg-cyan-950/20 p-4">
        <h3 className="text-sm font-medium text-cyan-200">{t("billing.bridgeTitle")}</h3>
        <p className="mt-1 text-xs text-slate-400">
          {t("billing.bridgeHint")}{" "}
          <span className="text-cyan-200">
            {bridgeConfigured ? t("billing.modeLive") : t("billing.modeMock")}
          </span>
        </p>
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <div className="rounded-lg border border-slate-800 bg-slate-900 p-3">
            <h4 className="text-xs font-medium text-slate-200">{t("billing.bridgeCountriesTitle")}</h4>
            <p className="mt-1 text-xs text-slate-500">
              {t("billing.bridgeCountriesMeta")
                .replace("{count}", String(bridgeCountriesQuery.data?.count ?? 0))
                .replace("{mode}", bridgeCountriesQuery.data?.mode ?? "mock")}
            </p>
            <div className="mt-2 max-h-32 overflow-auto text-xs text-slate-300">
              {(bridgeCountriesQuery.data?.countries ?? []).map((item) => (
                <div key={`${item.alpha3}-${item.sourceCurrency}`} className="border-b border-slate-800 py-1">
                  {item.name} ({item.alpha3}) · {item.sourceCurrency} · {item.rails.join(", ")}
                </div>
              ))}
            </div>
          </div>
          <div className="rounded-lg border border-slate-800 bg-slate-900 p-3">
            <h4 className="text-xs font-medium text-slate-200">{t("billing.bridgeKycTitle")}</h4>
            <input
              value={bridgeKycName}
              onChange={(e) => setBridgeKycName(e.target.value)}
              className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-100"
              placeholder={t("billing.bridgeKycName")}
            />
            <input
              value={bridgeKycEmail}
              onChange={(e) => setBridgeKycEmail(e.target.value)}
              className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-100"
              placeholder={t("billing.bridgeKycEmail")}
            />
            <button
              type="button"
              onClick={() => bridgeKYCMutation.mutate()}
              disabled={bridgeKYCMutation.isPending}
              className="mt-2 rounded-md border border-cyan-700/60 px-2 py-1 text-xs text-cyan-200 disabled:opacity-50"
            >
              {bridgeKYCMutation.isPending ? t("common.loading") : t("billing.bridgeCreateKyc")}
            </button>
            <input
              value={bridgeCustomerId}
              onChange={(e) => setBridgeCustomerId(e.target.value)}
              className="mt-3 w-full rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-100"
              placeholder={t("billing.bridgeCustomerId")}
            />
            <input
              value={bridgeWalletAddress}
              onChange={(e) => setBridgeWalletAddress(e.target.value)}
              className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-100"
              placeholder={t("billing.bridgeWalletAddress")}
            />
            <button
              type="button"
              onClick={() => bridgeCreateVAMutation.mutate()}
              disabled={bridgeCreateVAMutation.isPending}
              className="mt-2 rounded-md border border-cyan-700/60 px-2 py-1 text-xs text-cyan-200 disabled:opacity-50"
            >
              {bridgeCreateVAMutation.isPending ? t("common.loading") : t("billing.bridgeCreateVA")}
            </button>
          </div>
        </div>
        {bridgeCreateVAResult ? (
          <pre className="mt-3 overflow-auto rounded-md border border-slate-800 bg-slate-950 p-2 text-xs text-slate-300">
            {bridgeCreateVAResult}
          </pre>
        ) : null}
      </article>

      <div className="grid gap-4 md:grid-cols-2">
        <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("billing.capabilityTitle")}</h3>
          <ul className="mt-3 space-y-2 text-sm text-slate-300">
            <li>
              {t("billing.fiatChannel")}:{" "}
              <span className={canFiat ? "text-emerald-300" : "text-amber-300"}>
                {canFiat ? `${t("billing.enabled")} (${fiatCap?.provider ?? "-"})` : t("billing.disabled")}
              </span>
            </li>
            <li>
              {t("billing.stablecoinChannel")}:{" "}
              <span className={canStablecoin ? "text-emerald-300" : "text-amber-300"}>
                {canStablecoin
                  ? `${t("billing.enabled")} (${stableCap?.provider ?? "-"})`
                  : t("billing.disabled")}
              </span>
            </li>
            <li className="text-xs text-slate-500">
              Stripe {t("billing.liveCheckout")}: {stripeReady ? t("billing.configured") : t("billing.notConfigured")}
            </li>
          </ul>
        </article>

        <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("billing.checkoutTitle")}</h3>
          <div className="mt-3 space-y-2">
            <label className="block text-sm text-slate-300">
              {t("billing.checkoutTypeLabel")}
              <select
                value={checkoutType}
                onChange={(e) => setCheckoutType(e.target.value as "subscription" | "payment_link")}
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              >
                <option value="subscription">{t("billing.checkoutTypeSubscription")}</option>
                <option value="payment_link">{t("billing.checkoutTypePaymentLink")}</option>
              </select>
            </label>
            <label className="block text-sm text-slate-300">
              {t("billing.planLabel")}
              <select
                value={effectivePlanCode}
                onChange={(e) =>
                  setManualPlanCode(e.target.value as "starter" | "growth")
                }
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              >
                {PLANS.map((p) => (
                  <option key={p.code} value={p.code}>
                    {t(p.labelKey)}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm text-slate-300">
              {t("billing.amount")}
              <input
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
            </label>
            {checkoutType === "payment_link" ? (
              <label className="block text-sm text-slate-300">
                {t("billing.vaAccountId")}
                <input
                  value={vaAccountId}
                  onChange={(e) => setVaAccountId(e.target.value)}
                  placeholder={t("billing.vaAccountPlaceholder")}
                  className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
                />
              </label>
            ) : null}
            <label className="block text-sm text-slate-300">
              {t("billing.currency")}
              <select
                value={currency}
                onChange={(e) => setCurrency(e.target.value)}
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              >
                <option value="USD">USD</option>
                <option value="USDC">USDC</option>
                <option value="USDT">USDT</option>
              </select>
            </label>
            <label className="block text-sm text-slate-300">
              {t("billing.note")}
              <input
                value={customerHint}
                onChange={(e) => setCustomerHint(e.target.value)}
                placeholder={t("billing.notePlaceholder")}
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
            </label>
            <button
              type="button"
              onClick={() => {
                const valid = amountSchema.safeParse(amount);
                if (!valid.success) {
                  const message = valid.error.issues[0]?.message ?? t("common.failed");
                  setErrorMessage(message);
                  showToast("error", message);
                  return;
                }
                checkoutMutation.mutate();
              }}
              disabled={checkoutMutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {checkoutMutation.isPending ? t("common.loading") : t("billing.createCheckout")}
            </button>
          </div>
        </article>
      </div>

      {sub ? (
        <article className="rounded-xl border border-emerald-900/50 bg-emerald-950/20 p-4">
          <h3 className="text-sm font-medium text-emerald-200">{t("billing.currentSubscription")}</h3>
          <dl className="mt-2 grid gap-1 font-mono text-xs text-slate-300 sm:grid-cols-2">
            <div>
              <dt className="text-slate-500">{t("billing.subPlan")}</dt>
              <dd>{sub.planCode}</dd>
            </div>
            <div>
              <dt className="text-slate-500">{t("billing.subStatus")}</dt>
              <dd>{sub.status}</dd>
            </div>
            <div>
              <dt className="text-slate-500">{t("billing.subCurrency")}</dt>
              <dd>{sub.currency}</dd>
            </div>
            <div>
              <dt className="text-slate-500">{t("billing.subPeriodEnd")}</dt>
              <dd>{sub.currentPeriodEnd ?? "—"}</dd>
            </div>
            <div className="sm:col-span-2">
              <dt className="text-slate-500">{t("billing.subProviderSub")}</dt>
              <dd className="break-all">{sub.providerSubscriptionId ?? "—"}</dd>
            </div>
          </dl>
        </article>
      ) : null}

      <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <h3 className="text-sm font-medium text-slate-200">{t("billing.reconciliationTitle")}</h3>
        <p className="mt-1 text-xs text-slate-500">{t("billing.reconciliationDesc")}</p>
        <div className="mt-3 grid gap-2 md:grid-cols-5">
          <select
            value={reconCheckoutType}
            onChange={(e) => {
              setReconOffset(0);
              setReconCheckoutType(e.target.value);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-2 py-2 text-xs text-slate-100"
          >
            <option value="">{t("billing.reconFilterAllType")}</option>
            <option value="subscription">{t("billing.checkoutTypeSubscription")}</option>
            <option value="payment_link">{t("billing.checkoutTypePaymentLink")}</option>
          </select>
          <select
            value={reconStatus}
            onChange={(e) => {
              setReconOffset(0);
              setReconStatus(e.target.value);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-2 py-2 text-xs text-slate-100"
          >
            <option value="">{t("billing.reconFilterAllStatus")}</option>
            <option value="open">open</option>
            <option value="completed">completed</option>
            <option value="expired">expired</option>
          </select>
          <input
            value={reconVA}
            onChange={(e) => {
              setReconOffset(0);
              setReconVA(e.target.value);
            }}
            placeholder={t("billing.reconFilterVAPlaceholder")}
            className="rounded-md border border-slate-700 bg-slate-950 px-2 py-2 text-xs text-slate-100"
          />
          <label className="flex items-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-2 py-2 text-xs text-slate-200">
            <input
              type="checkbox"
              checked={reconAnomalyOnly}
              onChange={(e) => {
                setReconOffset(0);
                setReconAnomalyOnly(e.target.checked);
              }}
            />
            {t("billing.reconAnomalyOnly")}
          </label>
          <button
            type="button"
            onClick={() => exportMutation.mutate()}
            className="rounded-md border border-blue-700/60 px-2 py-2 text-xs text-blue-200 hover:bg-blue-950/40"
            disabled={exportMutation.isPending}
          >
            {exportMutation.isPending ? t("common.loading") : t("billing.reconExportCsv")}
          </button>
        </div>
        <div className="mt-2 flex justify-end">
          <button
            type="button"
            onClick={() => copyContextMutation.mutate()}
            disabled={copyContextMutation.isPending}
            className="rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-50"
          >
            {copyContextMutation.isPending ? t("common.loading") : t("billing.reconCopyContext")}
          </button>
        </div>
        <div className="mt-3 overflow-x-auto">
          <table className="min-w-full text-left text-xs text-slate-300">
            <thead className="text-slate-500">
              <tr>
                <th className="px-2 py-1">{t("billing.reconCreatedAt")}</th>
                <th className="px-2 py-1">{t("billing.reconStatus")}</th>
                <th className="px-2 py-1">{t("billing.reconType")}</th>
                <th className="px-2 py-1">VA</th>
                <th className="px-2 py-1">{t("billing.reconAmount")}</th>
                <th className="px-2 py-1">{t("billing.reconProviderSession")}</th>
              </tr>
            </thead>
            <tbody>
              {reconItems.map((item) => (
                <tr
                  key={item.providerSessionId || `${item.createdAt}-${item.amountMinor}`}
                  className={`border-t border-slate-800 ${item.anomaly ? "bg-rose-950/30" : ""}`}
                >
                  <td className="px-2 py-1">{item.createdAt ?? "-"}</td>
                  <td className="px-2 py-1">{item.status}</td>
                  <td className="px-2 py-1">{item.checkoutType || "-"}</td>
                  <td className="px-2 py-1">{item.vaAccountId || "-"}</td>
                  <td className="px-2 py-1">
                    {item.amountMinor !== null && item.amountMinor !== undefined
                      ? `${(item.amountMinor / 100).toFixed(2)} ${item.currency}`
                      : `- ${item.currency}`}
                  </td>
                  <td className="px-2 py-1 break-all">{item.providerSessionId || "-"}</td>
                </tr>
              ))}
              {reconItems.length === 0 ? (
                <tr>
                  <td className="px-2 py-2 text-slate-500" colSpan={6}>
                    {t("billing.reconEmpty")}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
          <span>
            {(t("billing.reconPagination") || "{offset}/{count}/{total}")
              .replace("{offset}", String(reconMeta?.offset ?? 0))
              .replace("{count}", String(reconMeta?.count ?? reconItems.length))
              .replace("{total}", String(reconMeta?.total ?? reconItems.length))}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={!canPrev}
              onClick={() => setReconOffset((v) => Math.max(0, v - reconLimit))}
              className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
            >
              {t("billing.reconPrev")}
            </button>
            <button
              type="button"
              disabled={!canNext}
              onClick={() => setReconOffset((v) => v + reconLimit)}
              className="rounded border border-slate-700 px-2 py-1 disabled:opacity-50"
            >
              {t("billing.reconNext")}
            </button>
          </div>
        </div>
      </article>

      {errorMessage ? (
        <div className="rounded-md border border-rose-700/60 bg-rose-950/30 p-3 text-sm text-rose-100">
          {errorMessage}
        </div>
      ) : null}

      {checkout ? (
        <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("billing.checkoutResult")}</h3>
          <div className="mt-2 space-y-1 font-mono text-xs text-slate-300">
            <p>checkoutId: {checkout.checkoutId}</p>
            <p>provider: {checkout.provider}</p>
            <p>rail: {checkout.paymentRail}</p>
            <p>currency: {checkout.currency}</p>
            <p>
              mode: {checkout.checkoutMode === "live" ? t("billing.modeLive") : t("billing.modeMock")}
            </p>
          </div>
          <a
            href={checkout.checkoutURL}
            target="_blank"
            rel="noreferrer"
            className="mt-3 inline-block rounded border border-blue-700/60 px-3 py-1 text-xs text-blue-200 hover:bg-blue-950/50"
          >
            {t("billing.openCheckout")}
          </a>
        </article>
      ) : null}
    </section>
  );
}
