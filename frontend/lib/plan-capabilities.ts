export type PlanCode = "free" | "starter" | "growth" | "enterprise";

export type CapabilityKey =
  | "billing.subscription_checkout"
  | "billing.payment_link_topup"
  /** 付费档 unlock 控制台发起 Bridge KYC / VA；合规上仍需用户完成 Bridge KYC */
  | "billing.bridge_onboarding";

const PLAN_CAPABILITIES: Record<PlanCode, CapabilityKey[]> = {
  /** 无需付费：可走 VA 充值 + Agent 支付 API；不能在平台创建 Stripe 订阅/支付链接结账；不含 Bridge 开户资格 */
  free: [],
  starter: ["billing.subscription_checkout", "billing.bridge_onboarding"],
  growth: [
    "billing.subscription_checkout",
    "billing.payment_link_topup",
    "billing.bridge_onboarding",
  ],
  enterprise: [
    "billing.subscription_checkout",
    "billing.payment_link_topup",
    "billing.bridge_onboarding",
  ],
};

export function isValidPlan(plan?: string | null): plan is PlanCode {
  return (
    !!plan &&
    (plan === "free" ||
      plan === "starter" ||
      plan === "growth" ||
      plan === "enterprise")
  );
}

/** Legacy sessions without plan default to starter (paid pilot); new signups use `free`. */
export function normalizePlanCode(input?: string | null): PlanCode {
  const p = (input ?? "").trim().toLowerCase();
  if (p === "free" || p === "starter" || p === "growth" || p === "enterprise") {
    return p;
  }
  return "starter";
}

export function getDefaultPlanForSignup(): PlanCode {
  return "free";
}

export function getPlanCapabilities(plan?: string | null): CapabilityKey[] {
  const normalized = normalizePlanCode(plan);
  return PLAN_CAPABILITIES[normalized];
}

export function hasPlanCapability(
  plan: string | null | undefined,
  capability: CapabilityKey,
) {
  return getPlanCapabilities(plan).includes(capability);
}

/** Free 档：允许注册并维护的 Agent 数量上限（通过控制台代理统计）。 */
export const FREE_TIER_MAX_AGENTS = 1;
