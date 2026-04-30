"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { useLocale } from "@/components/locale-provider";

type HelpSuggestion = {
  label: string;
  href: string;
};

type HelpAnswer = {
  answer: string;
  suggestions: HelpSuggestion[];
};

function inferPage(pathname: string) {
  if (pathname.startsWith("/billing")) return "billing";
  if (pathname.startsWith("/recharge")) return "recharge";
  if (pathname.startsWith("/transactions")) return "transactions";
  if (pathname.startsWith("/agents")) return "agents";
  return "general";
}

export function HelpAssistantWidget() {
  const pathname = usePathname();
  const { locale, t } = useLocale();
  const [open, setOpen] = useState(false);
  const [question, setQuestion] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<HelpAnswer | null>(null);
  const [error, setError] = useState("");

  const quickQuestions = useMemo(
    () =>
      locale === "en-US"
        ? [
            "How do I top up VA with custom amount?",
            "How do I export reconciliation CSV?",
            "What happens on refund or dispute?",
          ]
        : [
            "如何自定义金额充值到 VA？",
            "如何导出对账 CSV？",
            "退款或争议后会发生什么？",
          ],
    [locale],
  );

  async function ask(text: string) {
    const q = text.trim();
    if (!q) return;
    setLoading(true);
    setError("");
    try {
      const resp = await fetch("/api/help/query", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          question: q,
          page: inferPage(pathname),
          locale,
        }),
      });
      const payload = (await resp.json()) as {
        code: string;
        message?: string;
        data?: HelpAnswer;
      };
      if (!resp.ok || payload.code !== "0" || !payload.data) {
        throw new Error(payload.message || "query failed");
      }
      setResult(payload.data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "query failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      {open ? (
        <section className="fixed bottom-20 right-6 z-40 w-[360px] rounded-xl border border-slate-800 bg-slate-900 p-4 shadow-2xl">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-100">{t("helpAssistant.title")}</h3>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="text-xs text-slate-400 hover:text-slate-200"
            >
              {t("common.close")}
            </button>
          </div>

          <p className="mb-2 text-xs text-slate-400">{t("helpAssistant.subtitle")}</p>

          <div className="mb-3 flex flex-wrap gap-2">
            {quickQuestions.map((q) => (
              <button
                key={q}
                type="button"
                onClick={() => {
                  setQuestion(q);
                  void ask(q);
                }}
                className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300 hover:bg-slate-800"
              >
                {q}
              </button>
            ))}
          </div>

          <div className="flex gap-2">
            <input
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              placeholder={t("helpAssistant.inputPlaceholder")}
              className="w-full rounded border border-slate-700 bg-slate-950 px-2 py-2 text-xs text-slate-100"
            />
            <button
              type="button"
              onClick={() => void ask(question)}
              disabled={loading}
              className="rounded bg-blue-600 px-3 py-2 text-xs text-white disabled:opacity-60"
            >
              {loading ? t("helpAssistant.asking") : t("helpAssistant.ask")}
            </button>
          </div>

          {error ? <p className="mt-3 text-xs text-rose-300">{error}</p> : null}

          {result ? (
            <div className="mt-3 space-y-2">
              <p className="text-xs text-slate-200">{result.answer}</p>
              <div className="flex flex-wrap gap-2">
                {result.suggestions.map((s) => (
                  <Link
                    key={`${s.href}-${s.label}`}
                    href={s.href}
                    className="rounded border border-slate-700 px-2 py-1 text-xs text-blue-200 hover:bg-slate-800"
                    onClick={() => setOpen(false)}
                  >
                    {s.label}
                  </Link>
                ))}
              </div>
            </div>
          ) : null}
        </section>
      ) : null}

      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="fixed bottom-6 right-6 z-40 rounded-full bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-lg hover:bg-blue-500"
      >
        {t("helpAssistant.entry")}
      </button>
    </>
  );
}
