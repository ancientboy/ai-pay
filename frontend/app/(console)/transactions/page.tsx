"use client";

import { useMutation } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { pay, queryBalance, queryLedger, queryTransaction } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";
import { getValidationSchemas } from "@/lib/validation";

type TxRow = {
  id: string;
  amount: string;
  fee: string;
  status: string;
  createdAt?: string;
};

export default function TransactionsPage() {
  const { t, locale } = useLocale();
  const { paySchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const { showToast } = useToast();
  const [payerDid, setPayerDid] = useState("");
  const [merchantId, setMerchantId] = useState("m1");
  const [amount, setAmount] = useState("1");
  const [queryTxId, setQueryTxId] = useState("");
  const [queryVa, setQueryVa] = useState("");
  const [message, setMessage] = useState("");
  const [rows, setRows] = useState<TxRow[]>([]);
  const [statusFilter, setStatusFilter] = useState<"ALL" | "SETTLED" | "FAILED">("ALL");
  const [keyword, setKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<TxRow | null>(null);
  const pageSize = 10;

  const filteredRows = useMemo(() => {
    return rows.filter((row) => {
      const matchStatus = statusFilter === "ALL" || row.status === statusFilter;
      const matchKeyword =
        keyword.trim() === "" ||
        row.id.toLowerCase().includes(keyword.toLowerCase()) ||
        row.status.toLowerCase().includes(keyword.toLowerCase());
      return matchStatus && matchKeyword;
    });
  }, [rows, statusFilter, keyword]);

  const pagedRows = useMemo(() => {
    const start = (page - 1) * pageSize;
    return filteredRows.slice(start, start + pageSize);
  }, [filteredRows, page]);

  const pageCount = Math.max(1, Math.ceil(filteredRows.length / pageSize));

  const payMutation = useMutation({
    mutationFn: () => pay({ payerDid, merchantId, amount }),
    onSuccess: (data) => {
      setRows((prev) => [
        {
          id: data.transactionId,
          amount,
          fee: (Number(amount) * 0.003).toFixed(4),
          status: data.status,
        },
        ...prev,
      ]);
      setMessage(`${t("transactions.paySuccess")}: ${data.transactionId}`);
      showToast("success", `${t("transactions.paySuccess")}: ${data.transactionId}`);
    },
    onError: (err) => {
      const msg = `${t("common.failed")}: ${toReadableError(err, locale)}`;
      setMessage(msg);
      showToast("error", msg);
    },
  });

  const statusMutation = useMutation({
    mutationFn: () => queryTransaction(queryTxId),
    onSuccess: (data) => {
      const localizedStatus = formatStatus(locale, data.Status);
      setMessage(`${t("transactions.status")}: ${localizedStatus}, ${t("transactions.amount")}=${data.Amount}`);
      showToast("info", `${t("transactions.status")}: ${localizedStatus}`);
    },
    onError: (err) => {
      const msg = `${t("transactions.queryStatus")} ${t("common.failed").toLowerCase()}: ${toReadableError(err, locale)}`;
      setMessage(msg);
      showToast("error", msg);
    },
  });

  const balanceMutation = useMutation({
    mutationFn: () => queryBalance(queryVa),
    onSuccess: (data) => {
      setMessage(`${t("agents.balance")}: ${data.balance}`);
      showToast("info", `${t("agents.balance")}: ${data.balance}`);
    },
    onError: (err) => {
      const msg = `${t("transactions.queryBalance")} ${t("common.failed").toLowerCase()}: ${toReadableError(err, locale)}`;
      setMessage(msg);
      showToast("error", msg);
    },
  });

  const ledgerMutation = useMutation({
    mutationFn: () => queryLedger(queryVa),
    onSuccess: (data) => {
      const mapped = data.map((tx) => ({
        id: tx.ID,
        amount: tx.Amount.toString(),
        fee: (tx.Fee ?? 0).toString(),
        status: tx.Status,
        createdAt: tx.CreatedAt,
      }));
      setRows(mapped);
      setMessage(`${t("transactions.queryLedger")}: ${mapped.length} ${t("transactions.records")}`);
      showToast("info", `${t("transactions.queryLedger")}: ${mapped.length}`);
    },
    onError: (err) => {
      const msg = `${t("transactions.queryLedger")} ${t("common.failed").toLowerCase()}: ${toReadableError(err, locale)}`;
      setMessage(msg);
      showToast("error", msg);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("transactions.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {t("transactions.subtitle")}
        </p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <div className="mb-4 grid gap-4 md:grid-cols-2">
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              const parsed = paySchema.safeParse({
                payerDid,
                merchantId,
                amount,
              });
              if (!parsed.success) {
                const msg = parsed.error.issues[0]?.message ?? `${t("common.failed")}`;
                setMessage(msg);
                showToast("error", msg);
                return;
              }
              payMutation.mutate();
            }}
          >
            <p className="text-xs uppercase tracking-wide text-slate-400">{t("transactions.createPayment")}</p>
            <input
              value={payerDid}
              onChange={(e) => setPayerDid(e.target.value)}
              placeholder={t("transactions.payerDid")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
            <input
              value={merchantId}
              onChange={(e) => setMerchantId(e.target.value)}
              placeholder={t("transactions.merchantId")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder={t("transactions.amount")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
            <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white">
              {t("transactions.pay")}
            </button>
          </form>
          <div className="space-y-2">
            <p className="text-xs uppercase tracking-wide text-slate-400">{t("transactions.queryTools")}</p>
            <input
              value={queryTxId}
              onChange={(e) => setQueryTxId(e.target.value)}
              placeholder={t("transactions.txId")}
              className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
            <button
              onClick={() => statusMutation.mutate()}
              className="mr-2 rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
            >
              {t("transactions.queryStatus")}
            </button>
            <input
              value={queryVa}
              onChange={(e) => setQueryVa(e.target.value)}
              placeholder={t("agents.va")}
              className="mt-2 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            />
            <div className="mt-2 flex gap-2">
              <button
                onClick={() => balanceMutation.mutate()}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                {t("transactions.queryBalance")}
              </button>
              <button
                onClick={() => ledgerMutation.mutate()}
                className="rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200"
              >
                {t("transactions.queryLedger")}
              </button>
            </div>
          </div>
        </div>
        {message ? <p className="mb-3 text-sm text-slate-300">{message}</p> : null}
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value as "ALL" | "SETTLED" | "FAILED");
              setPage(1);
            }}
            className="rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-sm"
          >
            <option value="ALL">{t("transactions.allStatus")}</option>
            <option value="SETTLED">{formatStatus(locale, "SETTLED")}</option>
            <option value="FAILED">{formatStatus(locale, "FAILED")}</option>
          </select>
          <input
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value);
              setPage(1);
            }}
            placeholder={t("transactions.searchPlaceholder")}
            className="rounded-md border border-slate-700 bg-slate-950 px-3 py-1 text-sm"
          />
        </div>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="pb-2">{t("transactions.txId")}</th>
              <th className="pb-2">{t("transactions.amount")}</th>
              <th className="pb-2">{t("transactions.fee")}</th>
              <th className="pb-2">{t("transactions.status")}</th>
            </tr>
          </thead>
          <tbody>
            {pagedRows.map((row) => (
              <tr
                key={row.id}
                className="cursor-pointer border-b border-slate-800/60 hover:bg-slate-800/40"
                onClick={() => setSelected(row)}
              >
                <td className="py-3 font-mono text-xs text-slate-200">{row.id}</td>
                <td className="py-3 text-slate-300">{row.amount}</td>
                <td className="py-3 text-slate-300">{row.fee}</td>
                <td className="py-3">
                  <span
                    className={`rounded-full px-2 py-1 text-xs ${
                      row.status === "SETTLED"
                        ? "bg-emerald-500/20 text-emerald-300"
                        : "bg-rose-500/20 text-rose-300"
                    }`}
                  >
                    {formatStatus(locale, row.status)}
                  </span>
                </td>
              </tr>
            ))}
            {pagedRows.length === 0 ? (
              <tr>
                <td className="py-4 text-slate-500" colSpan={4}>
                  {t("transactions.noData")}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
        <div className="mt-3 flex items-center justify-between text-sm text-slate-400">
          <p>
            {t("transactions.page")} {page}/{pageCount} · {filteredRows.length} {t("transactions.records")}
          </p>
          <div className="flex gap-2">
            <button
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              className="rounded-md border border-slate-700 px-2 py-1 disabled:opacity-50"
            >
              {t("transactions.prev")}
            </button>
            <button
              disabled={page >= pageCount}
              onClick={() => setPage((p) => Math.min(pageCount, p + 1))}
              className="rounded-md border border-slate-700 px-2 py-1 disabled:opacity-50"
            >
              {t("transactions.next")}
            </button>
          </div>
        </div>
      </div>
      <DetailModal
        open={!!selected}
        title={t("transactions.detail")}
        onClose={() => setSelected(null)}
      >
        {selected ? (
          <div className="space-y-1 font-mono text-xs">
            <p>{t("transactions.txId")}: {selected.id}</p>
            <p>{t("transactions.amount")}: {selected.amount}</p>
            <p>{t("transactions.fee")}: {selected.fee}</p>
            <p>{t("transactions.status")}: {formatStatus(locale, selected.status)}</p>
            <p>{t("common.createdAt")}: {selected.createdAt ?? t("common.notAvailable")}</p>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
