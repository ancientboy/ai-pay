import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

type HelpSuggestion = {
  label: string;
  href: string;
};

type HelpResponse = {
  answer: string;
  suggestions: HelpSuggestion[];
};

function isEN(locale?: string) {
  return (locale ?? "").toLowerCase().startsWith("en");
}

function normalizeText(v: unknown) {
  return String(v ?? "").trim().toLowerCase();
}

function billingAnswer(question: string, en: boolean): HelpResponse {
  const q = normalizeText(question);
  const zh = !en;
  if (q.includes("订阅") || q.includes("subscription")) {
    return {
      answer: zh
        ? "订阅用于固定周期套餐；充值（payment_link）用于自定义金额给 VA 入账。"
        : "Subscriptions are for recurring plans; payment_link is for custom VA top-up amounts.",
      suggestions: [
        { label: zh ? "打开账单页" : "Open Billing", href: "/billing" },
        { label: zh ? "查看对账" : "View Reconciliation", href: "/billing" },
      ],
    };
  }
  if (q.includes("充值") || q.includes("topup") || q.includes("payment_link")) {
    return {
      answer: zh
        ? "充值流程：选择 payment_link，填写金额和 VA 账户，完成 Stripe 支付后 webhook 自动入账 VA。"
        : "Top-up flow: choose payment_link, provide amount and VA account, Stripe webhook credits VA automatically.",
      suggestions: [
        { label: zh ? "去充值" : "Go Top-up", href: "/billing" },
        { label: zh ? "查看 VA 余额" : "Check VA Balance", href: "/agents" },
      ],
    };
  }
  if (q.includes("退款") || q.includes("争议") || q.includes("refund") || q.includes("dispute")) {
    return {
      answer: zh
        ? "退款/争议事件会触发 VA 反向扣减；可在对账表里按异常筛选核对。"
        : "Refund/dispute events trigger VA reversal; use anomaly filters in reconciliation table to verify.",
      suggestions: [
        { label: zh ? "打开对账筛选" : "Open Reconciliation Filters", href: "/billing" },
        { label: zh ? "导出 CSV" : "Export CSV", href: "/billing" },
      ],
    };
  }
  if (q.includes("对账") || q.includes("csv") || q.includes("reconciliation")) {
    return {
      answer: zh
        ? "对账支持按类型、状态、VA、仅异常筛选，并可导出 CSV。"
        : "Reconciliation supports filters by type/status/VA/anomaly and CSV export.",
      suggestions: [
        { label: zh ? "打开账单页" : "Open Billing", href: "/billing" },
        { label: zh ? "查看交易" : "View Transactions", href: "/transactions" },
      ],
    };
  }
  return {
    answer: zh
      ? "可以问我：订阅与充值区别、如何给 VA 充值、退款争议如何回退、如何使用对账筛选与导出。"
      : "You can ask: subscription vs top-up, how to credit VA, refund/dispute reversal, and reconciliation filters/export.",
    suggestions: [{ label: zh ? "打开账单页" : "Open Billing", href: "/billing" }],
  };
}

type BillingContext = {
  checkoutType?: string;
  status?: string;
  vaAccountId?: string;
  anomalyOnly?: boolean;
};

function billingFixLink(ctx: BillingContext) {
  const q = new URLSearchParams();
  if (ctx.checkoutType) q.set("reconCheckoutType", ctx.checkoutType);
  if (ctx.status) q.set("reconStatus", ctx.status);
  if (ctx.vaAccountId) q.set("reconVA", ctx.vaAccountId);
  if (ctx.anomalyOnly) q.set("reconAnomalyOnly", "true");
  const suffix = q.toString();
  return `/billing${suffix ? `?${suffix}` : ""}`;
}

function billingFixAnswer(question: string, en: boolean, ctx: BillingContext): HelpResponse {
  const q = normalizeText(question);
  const zh = !en;
  const fixHref = billingFixLink({
    checkoutType: ctx.checkoutType || "payment_link",
    status: ctx.status || "completed",
    vaAccountId: ctx.vaAccountId,
    anomalyOnly: ctx.anomalyOnly ?? true,
  });

  if (q.includes("异常") || q.includes("anomaly") || q.includes("对账") || q.includes("reconciliation")) {
    return {
      answer: zh
        ? "已为你生成修复向导入口：打开后会自动带上对账筛选参数，优先定位异常 payment_link 记录。"
        : "I generated a fix wizard entry with prefilled reconciliation filters to locate anomalous payment_link records first.",
      suggestions: [
        { label: zh ? "打开修复向导" : "Open Fix Wizard", href: fixHref },
        { label: zh ? "导出当前筛选 CSV" : "Export Filtered CSV", href: fixHref },
      ],
    };
  }

  if (q.includes("充值") || q.includes("topup") || q.includes("退款") || q.includes("refund")) {
    return {
      answer: zh
        ? "建议先用修复向导筛选 VA 和状态，确认 Stripe 事件与 VA 记账是否一致，再决定补记账或回滚。"
        : "Use the fix wizard to filter by VA/status first, verify Stripe events vs VA ledger consistency, then decide credit replay or rollback.",
      suggestions: [
        { label: zh ? "打开修复向导" : "Open Fix Wizard", href: fixHref },
        { label: zh ? "查看交易页" : "Open Transactions", href: "/transactions" },
      ],
    };
  }

  return billingAnswer(question, en);
}

export async function POST(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  if (!claims?.sub) {
    return NextResponse.json({ code: "AUTH-001", message: "unauthorized" }, { status: 401 });
  }
  let body: {
    question?: string;
    locale?: string;
    page?: string;
    context?: BillingContext;
  } = {};
  try {
    body = (await request.json()) as {
      question?: string;
      locale?: string;
      page?: string;
      context?: BillingContext;
    };
  } catch {
    body = {};
  }
  const en = isEN(body.locale);
  const page = normalizeText(body.page);
  const data =
    page === "billing" || body.context
      ? billingFixAnswer(body.question ?? "", en, body.context ?? {})
      : billingAnswer(body.question ?? "", en);
  return NextResponse.json({ code: "0", data });
}
