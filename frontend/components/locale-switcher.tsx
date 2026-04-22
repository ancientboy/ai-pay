"use client";

import { useLocale } from "@/components/locale-provider";

export function LocaleSwitcher() {
  const { locale, setLocale, t } = useLocale();
  return (
    <label className="inline-flex items-center gap-2 text-xs text-slate-300">
      <span>{t("app.language")}:</span>
      <select
        value={locale}
        onChange={(e) => setLocale(e.target.value as "zh-CN" | "en-US")}
        className="rounded border border-slate-700 bg-slate-900 px-2 py-1 text-xs text-slate-200"
      >
        <option value="zh-CN">中文</option>
        <option value="en-US">English</option>
      </select>
    </label>
  );
}
