"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { createBillingIntent, getBillingCapabilities } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { getValidationSchemas } from "@/lib/validation";

export default function BillingPage() {
  const { t, locale } = useLocale();
  const { showToast } = useToast();
  const { amountSchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const [amount, setAmount] = useState("49.9");
  const [currency, setCurrency] = useState("USD");
  const [note, setNote] = useState("");
  const [checkout, setCheckout] = useState<{
    checkoutId: string;
    provider: string;
    paymentRail: string;
    currency: string;
    checkoutURL: string;
  } | null>(null);
  const [errorMessage, setErrorMessage] = useState("");

  const capabilitiesQuery = useQuery({
    queryKey: ["billing-capabilities"],
    queryFn: getBillingCapabilities,
  });

  const checkoutMutation = useMutation({
    mutationFn: () =>
      createBillingIntent({
        currency,
        provider: currency === "USD" ? "stripe" : "bridge",
        paymentRail: currency === "USD" ? "fiat" : "stablecoin",
        planCode: note.trim() || "starter",
        customerIdHint: note.trim() || undefined,
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
          </ul>
        </article>

        <article className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">{t("billing.checkoutTitle")}</h3>
          <div className="mt-3 space-y-2">
            <label className="block text-sm text-slate-300">
              {t("billing.amount")}
              <input
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
              />
            </label>
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
                value={note}
                onChange={(e) => setNote(e.target.value)}
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
