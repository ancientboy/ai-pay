"use client";

import { useLocale } from "@/components/locale-provider";

export function DetailModal({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const { t } = useLocale();
  if (!open) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-xl border border-slate-700 bg-slate-900 p-4"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-100">{title}</h3>
          <button
            className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300"
            onClick={onClose}
          >
            {t("common.close")}
          </button>
        </div>
        <div className="text-sm text-slate-200">{children}</div>
      </div>
    </div>
  );
}
