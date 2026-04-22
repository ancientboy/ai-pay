"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { DetailModal } from "@/components/detail-modal";
import { useToast } from "@/components/toast-provider";
import { createAccount, listAgents, registerAgent } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { agentDidSchema } from "@/lib/validation";

type AgentRow = {
  did: string;
  status: string;
  account: string;
  wallet: string;
  balance: number;
};

export default function AgentsPage() {
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
        wallet: item.walletAddress,
        balance: item.balance,
      }));
    },
  });

  const createMutation = useMutation({
    mutationFn: async (did: string) => {
      await registerAgent(did);
      const account = await createAccount(did);
      return {
        did: account.AgentDID,
        status: "ACTIVE",
        account: account.VAAccountID,
        wallet: account.WalletAddress,
        balance: 0,
      };
    },
    onSuccess: (row) => {
      setAgentDid("");
      setError("");
      showToast("success", `Agent created: ${row.did}`);
      queryClient.setQueryData<AgentRow[]>(["agents"], (prev) => [row, ...(prev ?? [])]);
      queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
    onError: (err) => {
      const message = toReadableError(err);
      setError(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Agents</h2>
        <p className="mt-1 text-sm text-slate-400">
          Manage Agent DID and linked payment accounts.
        </p>
      </div>

      <div className="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <form
          className="mb-4 flex gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            const parsed = agentDidSchema.safeParse(agentDid);
            if (!parsed.success) {
              setError(parsed.error.issues[0]?.message ?? "agent did invalid");
              return;
            }
            createMutation.mutate(parsed.data);
          }}
        >
          <input
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            className="flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-100"
            placeholder="did:gusd:agent:your_agent_name"
          />
          <button
            disabled={createMutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
          >
            Create Agent
          </button>
        </form>
        {error ? <p className="mb-3 text-sm text-rose-400">{error}</p> : null}
        {agentsQuery.isLoading ? (
          <p className="mb-3 text-sm text-slate-400">Loading agents...</p>
        ) : null}
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-800 text-left text-slate-400">
              <th className="pb-2">Agent DID</th>
              <th className="pb-2">Status</th>
              <th className="pb-2">VA Account</th>
              <th className="pb-2">Wallet</th>
              <th className="pb-2">Balance</th>
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
                <td className="py-3 text-slate-300">{row.status}</td>
                <td className="py-3 font-mono text-xs text-slate-300">{row.account}</td>
                <td className="py-3 font-mono text-xs text-slate-400">{row.wallet}</td>
                <td className="py-3 text-slate-300">{row.balance.toFixed(2)}</td>
              </tr>
            ))}
            {(agentsQuery.data ?? []).length === 0 ? (
              <tr>
                <td className="py-4 text-slate-500" colSpan={5}>
                  No agents yet. Create your first agent.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <DetailModal
        open={!!selected}
        title="Agent Detail"
        onClose={() => setSelected(null)}
      >
        {selected ? (
          <div className="space-y-1 font-mono text-xs">
            <p>DID: {selected.did}</p>
            <p>Status: {selected.status}</p>
            <p>VA: {selected.account}</p>
            <p>Wallet: {selected.wallet}</p>
            <p>Balance: {selected.balance.toFixed(4)}</p>
          </div>
        ) : null}
      </DetailModal>
    </section>
  );
}
