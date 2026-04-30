"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  createBillingIntent,
  getBillingCapabilities,
  getBillingReconciliation,
  getBillingSubscription,
  syncBillingCheckout,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { getValidationSchemas } from "@/lib/validation";

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
  const [planCode, setPlanCode] = useState<string>("starter");
  const [checkoutType, setCheckoutType] = useState<"subscription" | "payment_link">("subscription");
  const [vaAccountId, setVaAccountId] = useState("");
  const [customerHint, setCustomerHint] = useState("");
  const [checkout, setCheckout] = useState<{
    checkoutId: string;
    provider: string;
    paymentRail: string;
    currency: string;
    checkoutURL: string;
    checkoutMode?: string;
  } | null>(null);
  const [errorMessage, setErrorMessage] = useState("");

  const capabilitiesQuery = useQuery({
    queryKey: ["billing-capabilities"],
    queryFn: getBillingCapabilities,
  });

  const subscriptionQuery = useQuery({
    queryKey: ["billing-subscription"],
    queryFn: getBillingSubscription,
  });
  const reconciliationQuery = useQuery({
    queryKey: ["billing-reconciliation"],
    queryFn: () => getBillingReconciliation({ limit: 20 }),
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
    mutationFn: () =>
      createBillingIntent({
        currency,
        provider: currency === "USD" ? "stripe" : "bridge",
        paymentRail: currency === "USD" ? "fiat" : "stablecoin",
        checkoutType,
        planCode,
        amount,
        vaAccountId: checkoutType === "payment_link" ? vaAccountId.trim() || undefined : undefined,
        customerIdHint: customerHint.trim() || undefined,
      }),
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
  const sub = subscriptionQuery.data?.subscription ?? null;

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("billing.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("billing.subtitle")}</p>
      </div>

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
                value={planCode}
                onChange={(e) => setPlanCode(e.target.value)}
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
              {(reconciliationQuery.data?.items ?? []).map((item) => (
                <tr key={item.checkoutId} className="border-t border-slate-800">
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
              {(reconciliationQuery.data?.items?.length ?? 0) === 0 ? (
                <tr>
                  <td className="px-2 py-2 text-slate-500" colSpan={6}>
                    {t("billing.reconEmpty")}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
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
