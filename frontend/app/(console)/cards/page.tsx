"use client";

import { FormEvent, useMemo, useState } from "react";
import {
  applyVirtualCard,
  manageVirtualCard,
  payVirtualCard,
  type VirtualCard,
} from "@/lib/console-api";
import { toErrorMessage } from "@/lib/error-map";
import { useLocale } from "@/components/locale-provider";

type CardPayRecord = {
  transactionId: string;
  cardId: string;
  merchantId: string;
  amount: string;
  createdAt: string;
};

const CARD_OPERATIONS = ["FREEZE", "UNFREEZE", "ACTIVATE", "ADJUST_LIMIT"] as const;

export default function CardsPage() {
  const { t } = useLocale();
  const [agentDid, setAgentDid] = useState("");
  const [vaAccountId, setVaAccountId] = useState("");
  const [creditLimit, setCreditLimit] = useState("100");
  const [cardId, setCardId] = useState("");
  const [merchantId, setMerchantId] = useState("");
  const [amount, setAmount] = useState("1");
  const [operation, setOperation] = useState<(typeof CARD_OPERATIONS)[number]>("FREEZE");
  const [adjustAmount, setAdjustAmount] = useState("");
  const [cards, setCards] = useState<VirtualCard[]>([]);
  const [payments, setPayments] = useState<CardPayRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [statusMessage, setStatusMessage] = useState("");
  const latestCards = useMemo(() => cards.slice(0, 8), [cards]);
  const latestPayments = useMemo(() => payments.slice(0, 8), [payments]);

  async function handleApply(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setStatusMessage("");
    try {
      const record = await applyVirtualCard({
        agentDid: agentDid.trim(),
        vaAccountId: vaAccountId.trim(),
        creditLimit: creditLimit.trim(),
      });
      setCards((prev) => [record, ...prev.filter((item) => item.cardId !== record.cardId)]);
      setCardId(record.cardId);
      setStatusMessage(`${t("cards.applySuccess")}: ${record.cardId}`);
    } catch (error) {
      setStatusMessage(toErrorMessage(error, "操作失败"));
    } finally {
      setLoading(false);
    }
  }

  async function handlePay(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setStatusMessage("");
    try {
      const res = await payVirtualCard({
        agentDid: agentDid.trim(),
        cardId: cardId.trim(),
        merchantId: merchantId.trim(),
        amount: amount.trim(),
      });
      setPayments((prev) => [
        {
          transactionId: res.transactionId,
          cardId: cardId.trim(),
          merchantId: merchantId.trim(),
          amount: amount.trim(),
          createdAt: new Date().toISOString(),
        },
        ...prev,
      ]);
      setStatusMessage(`${t("cards.paySuccess")}: ${res.transactionId}`);
    } catch (error) {
      setStatusMessage(toErrorMessage(error, "操作失败"));
    } finally {
      setLoading(false);
    }
  }

  async function handleManage(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setStatusMessage("");
    try {
      const updated = await manageVirtualCard({
        cardId: cardId.trim(),
        operation,
        adjustAmount: operation === "ADJUST_LIMIT" ? adjustAmount.trim() : undefined,
      });
      setCards((prev) => [updated, ...prev.filter((item) => item.cardId !== updated.cardId)]);
      setStatusMessage(`${t("cards.manageSuccess")}: ${updated.status}`);
    } catch (error) {
      setStatusMessage(toErrorMessage(error, "操作失败"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-slate-100">{t("cards.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">{t("cards.subtitle")}</p>
      </div>

      {statusMessage ? (
        <div className="rounded border border-slate-700 bg-slate-900 px-4 py-3 text-sm text-slate-200">
          {statusMessage}
        </div>
      ) : null}

      <div className="grid gap-6 xl:grid-cols-3">
        <form onSubmit={handleApply} className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="mb-3 text-sm font-semibold text-slate-100">{t("cards.applyTitle")}</h3>
          <div className="space-y-3">
            <input
              value={agentDid}
              onChange={(e) => setAgentDid(e.target.value)}
              placeholder={t("cards.agentDid")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <input
              value={vaAccountId}
              onChange={(e) => setVaAccountId(e.target.value)}
              placeholder={t("cards.vaAccountId")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <input
              value={creditLimit}
              onChange={(e) => setCreditLimit(e.target.value)}
              placeholder={t("cards.creditLimit")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded bg-blue-600 px-3 py-2 text-sm font-medium hover:bg-blue-500 disabled:opacity-60"
            >
              {t("cards.apply")}
            </button>
          </div>
        </form>

        <form onSubmit={handlePay} className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="mb-3 text-sm font-semibold text-slate-100">{t("cards.payTitle")}</h3>
          <div className="space-y-3">
            <input
              value={cardId}
              onChange={(e) => setCardId(e.target.value)}
              placeholder={t("cards.cardId")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <input
              value={merchantId}
              onChange={(e) => setMerchantId(e.target.value)}
              placeholder={t("cards.merchantId")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={t("cards.amount")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded bg-indigo-600 px-3 py-2 text-sm font-medium hover:bg-indigo-500 disabled:opacity-60"
            >
              {t("cards.pay")}
            </button>
          </div>
        </form>

        <form onSubmit={handleManage} className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="mb-3 text-sm font-semibold text-slate-100">{t("cards.manageTitle")}</h3>
          <div className="space-y-3">
            <input
              value={cardId}
              onChange={(e) => setCardId(e.target.value)}
              placeholder={t("cards.cardId")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              required
            />
            <select
              value={operation}
              onChange={(e) => setOperation(e.target.value as (typeof CARD_OPERATIONS)[number])}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            >
              {CARD_OPERATIONS.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
            <input
              value={adjustAmount}
              onChange={(e) => setAdjustAmount(e.target.value)}
              placeholder={t("cards.adjustAmount")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
              disabled={operation !== "ADJUST_LIMIT"}
            />
            <button
              type="submit"
              disabled={loading}
              className="w-full rounded bg-emerald-600 px-3 py-2 text-sm font-medium hover:bg-emerald-500 disabled:opacity-60"
            >
              {t("cards.manage")}
            </button>
          </div>
        </form>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <section className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="mb-3 text-sm font-semibold text-slate-100">{t("cards.latestCards")}</h3>
          {latestCards.length === 0 ? (
            <p className="text-sm text-slate-400">{t("cards.noCards")}</p>
          ) : (
            <div className="space-y-2 text-sm">
              {latestCards.map((item) => (
                <div key={item.cardId} className="rounded border border-slate-800 bg-slate-950/60 px-3 py-2">
                  <div className="font-medium text-slate-100">{item.cardId}</div>
                  <div className="text-xs text-slate-400">
                    {item.maskedPan} · {item.status} · limit {item.creditLimit}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="rounded-lg border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="mb-3 text-sm font-semibold text-slate-100">{t("cards.latestPayments")}</h3>
          {latestPayments.length === 0 ? (
            <p className="text-sm text-slate-400">{t("cards.noPayments")}</p>
          ) : (
            <div className="space-y-2 text-sm">
              {latestPayments.map((item) => (
                <div
                  key={item.transactionId}
                  className="rounded border border-slate-800 bg-slate-950/60 px-3 py-2"
                >
                  <div className="font-medium text-slate-100">{item.transactionId}</div>
                  <div className="text-xs text-slate-400">
                    {item.cardId} · {item.merchantId} · {item.amount}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
