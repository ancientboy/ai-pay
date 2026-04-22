"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useToast } from "@/components/toast-provider";
import { listRecharges, recharge } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { rechargeSchema } from "@/lib/validation";

export default function RechargePage() {
  const { showToast } = useToast();
  const [vaAccountId, setVaAccountId] = useState("");
  const [amount, setAmount] = useState("100");
  const [message, setMessage] = useState("");
  const [selected, setSelected] = useState<{
    rechargeId: string;
    vaAccountId: string;
    amount: number;
    status: string;
    createdAt: string;
  } | null>(null);

  const rechargesQuery = useQuery({
    queryKey: ["recharges", vaAccountId],
    queryFn: () => listRecharges(vaAccountId || undefined, 20),
  });

  const mutation = useMutation({
    mutationFn: () => recharge({ vaAccountId, amount }),
    onSuccess: () => {
      setMessage("Recharge settled");
      showToast("success", "Recharge settled");
      rechargesQuery.refetch();
    },
    onError: (err) => {
      const message = toReadableError(err);
      setMessage(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Recharge</h2>
        <p className="mt-1 text-sm text-slate-400">
          Submit recharge request and review settlement status.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <form
          className="rounded-xl border border-slate-800 bg-slate-900 p-4"
          onSubmit={(e) => {
            e.preventDefault();
            setMessage("");
            const parsed = rechargeSchema.safeParse({ vaAccountId, amount });
            if (!parsed.success) {
              const msg = parsed.error.issues[0]?.message ?? "参数不合法";
              setMessage(msg);
              showToast("error", msg);
              return;
            }
            mutation.mutate();
          }}
        >
          <h3 className="text-sm font-medium text-slate-200">New Recharge</h3>
          <label className="mt-4 block text-sm text-slate-300">
            VA Account ID
            <input
              value={vaAccountId}
              onChange={(e) => setVaAccountId(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
          </label>
          <label className="mt-3 block text-sm text-slate-300">
            Amount (GUSD)
            <input
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
            />
          </label>
          <button
            disabled={mutation.isPending}
            className="mt-4 rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
          >
            Submit
          </button>
          {message ? <p className="mt-3 text-sm text-slate-300">{message}</p> : null}
        </form>

        <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
          <h3 className="text-sm font-medium text-slate-200">Recent Recharges</h3>
          <ul className="mt-3 space-y-2 text-sm text-slate-300">
            {(rechargesQuery.data ?? []).map((item) => (
              <li
                key={item.rechargeId}
                className="cursor-pointer rounded px-2 py-1 hover:bg-slate-800/40"
                onClick={() => setSelected(item)}
              >
                {item.rechargeId} - {item.status} - {item.amount} GUSD ({item.vaAccountId})
              </li>
            ))}
            {(rechargesQuery.data ?? []).length === 0 ? (
              <li className="text-slate-500">No records yet.</li>
            ) : null}
          </ul>
        </div>
      </div>
      <DetailModal
        open={!!selected}
        title="Recharge Detail"
        onClose={() => setSelected(null)}
      >
        {selected ? (
          <div className="space-y-1 font-mono text-xs">
            <p>Recharge ID: {selected.rechargeId}</p>
            <p>VA: {selected.vaAccountId}</p>
            <p>Amount: {selected.amount}</p>
            <p>Status: {selected.status}</p>
            <p>CreatedAt: {selected.createdAt}</p>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
