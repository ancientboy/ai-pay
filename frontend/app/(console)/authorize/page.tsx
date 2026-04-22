"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useToast } from "@/components/toast-provider";
import { setAuthorizeRule } from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { authorizeSchema } from "@/lib/validation";

export default function AuthorizePage() {
  const { showToast } = useToast();
  const [agentDid, setAgentDid] = useState("");
  const [singleLimit, setSingleLimit] = useState("50");
  const [dailyLimit, setDailyLimit] = useState("200");
  const [whitelist, setWhitelist] = useState("m1");
  const [result, setResult] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      setAuthorizeRule({
        agentDid,
        singleLimit,
        dailyLimit,
        whitelist: whitelist
          .split(",")
          .map((v) => v.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      setResult("Rule saved successfully");
      showToast("success", "Authorize rule saved");
    },
    onError: (err) => {
      const message = `Failed: ${toReadableError(err)}`;
      setResult(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">Authorize Rules</h2>
        <p className="mt-1 text-sm text-slate-400">
          Configure payment amount limits and merchant whitelist.
        </p>
      </div>

      <form
        className="grid gap-4 rounded-xl border border-slate-800 bg-slate-900 p-4 md:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault();
          setResult("");
          const parsed = authorizeSchema.safeParse({
            agentDid,
            singleLimit,
            dailyLimit,
            whitelist: whitelist
              .split(",")
              .map((v) => v.trim())
              .filter(Boolean),
          });
          if (!parsed.success) {
            const message = parsed.error.issues[0]?.message ?? "参数不合法";
            setResult(message);
            showToast("error", message);
            return;
          }
          mutation.mutate();
        }}
      >
        <label className="text-sm text-slate-300">
          Agent DID
          <input
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          Single Limit (GUSD)
          <input
            value={singleLimit}
            onChange={(e) => setSingleLimit(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          Daily Limit (GUSD)
          <input
            value={dailyLimit}
            onChange={(e) => setDailyLimit(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          Merchant Whitelist (comma separated)
          <input
            value={whitelist}
            onChange={(e) => setWhitelist(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <div className="md:col-span-2">
          <button
            disabled={mutation.isPending}
            className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
          >
            Save Rule
          </button>
          {result ? <p className="mt-3 text-sm text-slate-300">{result}</p> : null}
        </div>
      </form>
    </section>
  );
}
