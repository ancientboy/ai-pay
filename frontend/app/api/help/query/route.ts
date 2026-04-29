import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

type HelpInput = {
  query?: string;
  page?: string;
};

type HelpAnswer = {
  answer: string;
  actions: Array<{ label: string; href: string }>;
  references: string[];
};

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function normalizeText(input?: string) {
  return (input ?? "").trim().toLowerCase();
}

function buildContextActions(page: string, en: boolean) {
  if (page.startsWith("/transactions")) {
    return [
      { label: en ? "Open Recharge" : "去充值", href: "/recharge" },
      { label: en ? "Open Authorize Rules" : "去授权规则", href: "/authorize" },
    ];
  }
  if (page.startsWith("/recharge")) {
    return [
      { label: en ? "Open Agents" : "去 Agent 管理", href: "/agents" },
      { label: en ? "Open Transactions" : "去交易", href: "/transactions" },
    ];
  }
  if (page.startsWith("/authorize")) {
    return [
      { label: en ? "Open Agents" : "去 Agent 管理", href: "/agents" },
      { label: en ? "Open Transactions" : "去交易", href: "/transactions" },
    ];
  }
  return [
    { label: en ? "Open Onboarding" : "打开新手引导", href: "/onboarding" },
    { label: en ? "Open API Docs" : "打开 API 文档", href: "/api-docs" },
  ];
}

function resolveAnswer(input: HelpInput, en: boolean): HelpAnswer {
  const q = normalizeText(input.query);
  const page = normalizeText(input.page);

  if (!q) {
    return {
      answer: en
        ? "Tell me what you want to do, for example: how to complete first payment, why payment failed, or how to configure recharge."
        : "你可以告诉我你想完成什么，例如：如何完成首笔支付、支付失败原因、如何配置充值。",
      actions: buildContextActions(page, en),
      references: ["/onboarding", "/incident-center", "/api-docs"],
    };
  }

  if (q.includes("first payment") || q.includes("首笔支付") || q.includes("快速上手")) {
    return {
      answer: en
        ? "Recommended path: 1) create Agent and VA, 2) set authorize rules, 3) recharge, 4) submit a small payment in Transactions, 5) check status with requestId."
        : "建议路径：1）创建 Agent 与 VA；2）配置授权规则；3）充值；4）在交易页发起小额支付；5）使用 requestId 核对状态。",
      actions: [
        { label: en ? "Open Onboarding" : "打开新手引导", href: "/onboarding" },
        { label: en ? "Open Transactions" : "去交易页", href: "/transactions" },
      ],
      references: ["/onboarding", "/transactions", "/incident-center"],
    };
  }

  if (q.includes("kyc") || q.includes("认证") || q.includes("bridge")) {
    return {
      answer: en
        ? "If KYC is not completed, deposit/recharge addresses may be unavailable. Complete KYC first, then retry recharge."
        : "若 KYC 未完成，充值地址可能不可用。请先完成 KYC，再重试充值流程。",
      actions: [
        { label: en ? "Open KYC" : "去 KYC 页", href: "/kyc" },
        { label: en ? "Open Recharge" : "去充值页", href: "/recharge" },
      ],
      references: ["/kyc", "/recharge", "/onboarding"],
    };
  }

  if (q.includes("fail") || q.includes("失败") || q.includes("pay-")) {
    return {
      answer: en
        ? "For payment failures, copy the error code and requestId, then locate action by code in Incident Center. Retry with a new idempotency key."
        : "支付失败时请先复制错误码和 requestId，在失败处置中心按错误码查动作，并使用新的幂等键重试。",
      actions: [
        { label: en ? "Open Incident Center" : "打开失败处置中心", href: "/incident-center" },
        { label: en ? "Open Transactions" : "回到交易页", href: "/transactions" },
      ],
      references: ["/incident-center", "/transactions"],
    };
  }

  if (q.includes("api") || q.includes("文档") || q.includes("curl")) {
    return {
      answer: en
        ? "You can find common curl templates and external provider docs in API Docs. Replace BASE URL, user id, and payload before execution."
        : "你可以在 API 文档页查看常用 curl 模板与外部服务商文档，执行前替换 BASE URL、用户ID与请求体。",
      actions: [{ label: en ? "Open API Docs" : "打开 API 文档", href: "/api-docs" }],
      references: ["/api-docs", "/developer"],
    };
  }

  return {
    answer: en
      ? "I can help with onboarding, recharge, authorization, transaction failures, and API calls. Try asking with keywords like recharge, PAY-003, or first payment."
      : "我可以协助你处理新手引导、充值、授权规则、交易失败与 API 调用。可尝试关键词：充值、PAY-003、首笔支付。",
    actions: buildContextActions(page, en),
    references: ["/onboarding", "/incident-center", "/api-docs"],
  };
}

export async function POST(request: NextRequest) {
  const sessionToken = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(sessionToken);
  if (!claims?.sub) {
    return NextResponse.json(
      {
        code: "AUTH-001",
        message: isEnglish(request) ? "Login required" : "请先登录",
      },
      { status: 401 },
    );
  }

  const body = (await request.json().catch(() => null)) as HelpInput | null;
  const en = isEnglish(request);
  const result = resolveAnswer(body ?? {}, en);
  return NextResponse.json({
    code: "0",
    data: result,
  });
}
