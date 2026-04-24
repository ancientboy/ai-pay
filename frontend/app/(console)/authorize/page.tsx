"use client";

import { useMutation } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { useLocale } from "@/components/locale-provider";
import { useToast } from "@/components/toast-provider";
import {
  activateAuthorizeRule,
  freezeAuthorizeRule,
  setAuthorizeRule,
  updateAuthorizeRule,
} from "@/lib/console-api";
import { toReadableError } from "@/lib/error-map";
import { getValidationSchemas } from "@/lib/validation";

export default function AuthorizePage() {
  const { t, locale } = useLocale();
  const { authorizeSchema } = useMemo(() => getValidationSchemas(locale), [locale]);
  const { showToast } = useToast();
  const [agentDid, setAgentDid] = useState("");
  const [singleLimit, setSingleLimit] = useState("50");
  const [dailyLimit, setDailyLimit] = useState("200");
  const [whitelist, setWhitelist] = useState("m1");
  const [result, setResult] = useState("");
  const whitelistList = whitelist
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);

  const mutation = useMutation({
    mutationFn: () =>
      setAuthorizeRule({
        agentDid,
        singleLimit,
        dailyLimit,
        whitelist: whitelistList,
      }),
    onSuccess: () => {
      setResult(t("authorize.saveSuccess"));
      showToast("success", t("authorize.saveSuccess"));
    },
    onError: (err) => {
      const message = `${t("common.failed")}: ${toReadableError(err, locale)}`;
      setResult(message);
      showToast("error", message);
    },
  });

  const updateMutation = useMutation({
    mutationFn: () =>
      updateAuthorizeRule({
        agentDid,
        singleLimit,
        dailyLimit,
        whitelist: whitelistList,
      }),
    onSuccess: () => {
      setResult(t("authorize.updateSuccess"));
      showToast("success", t("authorize.updateSuccess"));
    },
    onError: (err) => {
      const message = `${t("common.failed")}: ${toReadableError(err, locale)}`;
      setResult(message);
      showToast("error", message);
    },
  });

  const freezeMutation = useMutation({
    mutationFn: () => freezeAuthorizeRule(agentDid),
    onSuccess: () => {
      setResult(t("authorize.freezeSuccess"));
      showToast("success", t("authorize.freezeSuccess"));
    },
    onError: (err) => {
      const message = `${t("common.failed")}: ${toReadableError(err, locale)}`;
      setResult(message);
      showToast("error", message);
    },
  });

  const activateMutation = useMutation({
    mutationFn: () => activateAuthorizeRule(agentDid),
    onSuccess: () => {
      setResult(t("authorize.activateSuccess"));
      showToast("success", t("authorize.activateSuccess"));
    },
    onError: (err) => {
      const message = `${t("common.failed")}: ${toReadableError(err, locale)}`;
      setResult(message);
      showToast("error", message);
    },
  });

  return (
    <section className="space-y-6">
      <div>
        <h2 className="text-xl font-semibold">{t("authorize.title")}</h2>
        <p className="mt-1 text-sm text-slate-400">
          {t("authorize.subtitle")}
        </p>
      </div>

      <form
        className="grid gap-4 rounded-xl border border-slate-800 bg-slate-900 p-4 md:grid-cols-2"
        onSubmit={(e) => {
          e.preventDefault();
          setResult("");
          const parsed = authorizeSchema.safeParse({ agentDid, singleLimit, dailyLimit, whitelist: whitelistList });
          if (!parsed.success) {
            const message = parsed.error.issues[0]?.message ?? `${t("common.failed")}`;
            setResult(message);
            showToast("error", message);
            return;
          }
          mutation.mutate();
        }}
      >
        <label className="text-sm text-slate-300">
          {t("authorize.agentDid")}
          <input
            value={agentDid}
            onChange={(e) => setAgentDid(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          {t("authorize.singleLimit")}
          <input
            value={singleLimit}
            onChange={(e) => setSingleLimit(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          {t("authorize.dailyLimit")}
          <input
            value={dailyLimit}
            onChange={(e) => setDailyLimit(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <label className="text-sm text-slate-300">
          {t("authorize.whitelist")}
          <input
            value={whitelist}
            onChange={(e) => setWhitelist(e.target.value)}
            className="mt-1 w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100"
          />
        </label>
        <div className="md:col-span-2">
          <div className="flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={mutation.isPending}
              className="rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-500 disabled:opacity-60"
            >
              {t("authorize.save")}
            </button>
            <button
              type="button"
              disabled={updateMutation.isPending}
              onClick={() => {
                const parsed = authorizeSchema.safeParse({ agentDid, singleLimit, dailyLimit, whitelist: whitelistList });
                if (!parsed.success) {
                  const message = parsed.error.issues[0]?.message ?? `${t("common.failed")}`;
                  setResult(message);
                  showToast("error", message);
                  return;
                }
                updateMutation.mutate();
              }}
              className="rounded-md border border-slate-700 px-3 py-2 text-sm font-medium text-slate-200 disabled:opacity-60"
            >
              {t("authorize.update")}
            </button>
            <button
              type="button"
              disabled={freezeMutation.isPending || !agentDid.trim()}
              onClick={() => freezeMutation.mutate()}
              className="rounded-md bg-rose-600 px-3 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {t("authorize.freeze")}
            </button>
            <button
              type="button"
              disabled={activateMutation.isPending || !agentDid.trim()}
              onClick={() => activateMutation.mutate()}
              className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {t("authorize.activate")}
            </button>
          </div>
          {result ? <p className="mt-3 text-sm text-slate-300">{result}</p> : null}
        </div>
      </form>
    </section>
  );
}
