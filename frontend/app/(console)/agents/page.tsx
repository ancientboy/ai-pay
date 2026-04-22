"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import { ensureAgentSigningPublicKey } from "@/lib/agent-signature";
import { createAccount, listAgents, registerAgent } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { formatStatus } from "@/lib/i18n";
import { getValidationSchemas } from "@/lib/validation";

type AgentRow = {
  did: string;
  status: string;
  account: string;
  cardNo: string;
  wallet: string;
  balance: number;
};

export default function AgentsPage() {
  const { t, locale } = useLocale();
  const { agentDidSchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [agentDid, setAgentDid] = useState("");
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<AgentRow | null>(null);

  const agentsQuery = useQuery({
    queryKey: ["agents"],
    queryFn: async () => {
      const list = await listAgents();
      return list.map<AgentRow>((item) => ({
        did: item.agentDid,
        status: item.status,
        account: item.vaAccountId,
        cardNo: item.vaCardNo,
        wallet: item.walletAddress,
        balance: item.balance,
      }));
    },
  });

  const createMutation = useMutation({
    mutationFn: async (did: string) => {
      const didPubKey = await ensureAgentSigningPublicKey(did);
      await registerAgent(did, didPubKey);
      const account = await createAccount(did);
      return {
        did: account.AgentDID,
        status: "ACTIVE",
        account: account.VAAccountID,
        cardNo: account.VACardNo,
        wallet: account.WalletAddress,
        balance: 0,
      };
    },
    onSuccess: (row) => {
      setAgentDid("");
      setError("");
      showToast("success", `${t("agents.createSuccess")}: ${row.did}`);
      queryClient.setQueryData<AgentRow[]>(["agents"], (prev) => [row, ...(prev ?? [])]);
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
    onError: (err) => {
      const message = toReadableError(err, locale);
      setError(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("agents.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {t("agents.subtitle")}
        </p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <form
          className="mb-4 flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const parsed = agentDidSchema.safeParse(agentDid);
            if (!parsed.success) {
              setError(parsed.error.issues[0]?.message ?? t("validation.agentDidFormat"));
              return;
            }
            createMutation.mutate(parsed.data);
          }}
        >
          <input
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            className="flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
            placeholder={t("agents.createPlaceholder")}
          />
          <button
            disabled={createMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
          >
            {t("agents.create")}
          </button>
        </form>
        {error ? <p className="mb-3 text-sm text-rose-400">{error}</p> : null}
        {agentsQuery.isLoading ? (
          <p className="mb-3 text-sm text-slate-400">{t("common.loading")}</p>
        ) : null}
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="pb-2">{t("agents.did")}</th>
              <th className="pb-2">{t("agents.status")}</th>
              <th className="pb-2">{t("agents.va")}</th>
              <th className="pb-2">{t("agents.vaCardNo")}</th>
              <th className="pb-2">{t("agents.wallet")}</th>
              <th className="pb-2">{t("agents.balance")}</th>
            </tr>
          </thead>
          <tbody>
            {(agentsQuery.data ?? []).map((row) => (
              <tr
                key={row.did}
                className="cursor-pointer border-b border-slate-800/60 hover:bg-slate-800/40"
                onClick={() => setSelected(row)}
              >
                <td className="py-3 font-mono text-xs text-slate-200">{row.did}</td>
                <td className="py-3 text-slate-300">{formatStatus(locale, row.status)}</td>
                <td className="py-3 font-mono text-xs text-slate-300">{row.account}</td>
                <td className="py-3 font-mono text-xs text-slate-300">{row.cardNo}</td>
                <td className="py-3 font-mono text-xs text-slate-400">{row.wallet}</td>
                <td className="py-3 text-slate-300">{row.balance.toFixed(2)}</td>
              </tr>
            ))}
            {(agentsQuery.data ?? []).length === 0 ? (
              <tr>
                <td className="py-4 text-slate-500" colSpan={6}>
                  {t("agents.noAgents")}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <DetailModal
        open={!!selected}
        title={t("agents.detail")}
        onClose={() => setSelected(null)}
      >
        {selected ? (
          <div className="space-y-1 font-mono text-xs">
            <p>{t("agents.did")}: {selected.did}</p>
            <p>{t("agents.status")}: {formatStatus(locale, selected.status)}</p>
            <p>{t("agents.va")}: {selected.account}</p>
            <p>{t("agents.vaCardNo")}: {selected.cardNo}</p>
            <p>{t("agents.wallet")}: {selected.wallet}</p>
            <p>{t("agents.balance")}: {selected.balance.toFixed(4)}</p>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
