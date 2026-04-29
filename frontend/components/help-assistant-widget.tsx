"use client";

import { useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { useLocale } from "@/components/locale-provider";

type HelpSuggestion = {
  label: string;
  href: string;
};

type HelpResult = {
  answer: string;
  suggestions?: HelpSuggestion[];
};

function inferPage(pathname: string) {
  if (pathname.startsWith("/agents")) return "agents";
  if (pathname.startsWith("/kyc")) return "kyc";
  if (pathname.startsWith("/recharge")) return "recharge";
  if (pathname.startsWith("/authorize")) return "authorize";
  if (pathname.startsWith("/transactions")) return "transactions";
  if (pathname.startsWith("/developer")) return "developer";
  return "general";
}

export function HelpAssistantWidget() {
  const pathname = usePathname();
  const { locale, t } = useLocale();
  const isEN = locale === "en-US";
  const [open, setOpen] = useState(false);
  const [question, setQuestion] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<HelpResult | null>(null);

  const quickQuestions = useMemo(
    () =>
      isEN
        ? [
            "How do I complete the first payment?",
            "Why did payment fail with PAY-003?",
            "How to configure authorization limits?",
          ]
        : [
            "如何完成首笔支付？",
            "支付失败 PAY-003 怎么处理？",
            "授权限额怎么配置？",
          ],
    [isEN],
  );

  async function ask(text: string) {
    const value = text.trim();
    if (!value) return;
    setLoading(true);
    setError("");
    try {
      const response = await fetch("/api/help/query", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          question: value,
          page: inferPage(pathname),
          locale,
        }),
      });
      const payload = (await response.json()) as {
        code: string;
        message?: string;
        data?: HelpResult;
      };
      if (!response.ok || payload.code !== "0" || !payload.data) {
        throw new Error(payload.message || "request failed");
      }
      setResult(payload.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "request failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      {open ? (
        <div className="fixed bottom-5 right-5 z-50 w-[360px] max-w-[calc(100vw-24px)] rounded-xl border border-slate-700 bg-slate-900 shadow-2xl">
          <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
            <p className="text-sm font-medium text-slate-100">{t("helpAssistant.title")}</p>
            <button
              type="button"
              onClick={() => setOpen(false)}
              className="text-xs text-slate-400 hover:text-slate-200"
            >
              {t("common.close")}
            </button>
          </div>
          <div className="space-y-3 p-4">
            <p className="text-xs text-slate-400">{t("helpAssistant.subtitle")}</p>
            <div className="flex flex-wrap gap-2">
              {quickQuestions.map((item) => (
                <button
                  key={item}
                  type="button"
                  onClick={() => {
                    setQuestion(item);
                    void ask(item);
                  }}
                  className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300 hover:bg-slate-800"
                >
                  {item}
                </button>
              ))}
            </div>
            <div className="space-y-2">
              <textarea
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
                placeholder={t("helpAssistant.inputPlaceholder")}
                className="h-20 w-full rounded border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 outline-none focus:border-blue-600"
              />
              <button
                type="button"
                onClick={() => void ask(question)}
                disabled={loading}
                className="rounded bg-blue-600 px-3 py-1.5 text-xs text-white disabled:opacity-60"
              >
                {loading ? t("helpAssistant.asking") : t("helpAssistant.ask")}
              </button>
            </div>
            {error ? <p className="text-xs text-rose-300">{error}</p> : null}
            {result ? (
              <div className="space-y-2 rounded border border-slate-800 bg-slate-950 p-3">
                <p className="text-xs text-slate-200">{result.answer}</p>
                {(result.suggestions ?? []).length > 0 ? (
                  <div className="flex flex-wrap gap-2">
                    {(result.suggestions ?? []).map((item) => (
                      <a
                        key={`${item.href}-${item.label}`}
                        href={item.href}
                        className="rounded border border-blue-700/60 px-2 py-1 text-xs text-blue-200 hover:bg-blue-950/50"
                      >
                        {item.label}
                      </a>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
      ) : null}

      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="fixed bottom-5 right-5 z-40 rounded-full bg-blue-600 px-4 py-2 text-sm text-white shadow-lg hover:bg-blue-500"
      >
        {t("helpAssistant.button")}
      </button>
    </>
  );
}
