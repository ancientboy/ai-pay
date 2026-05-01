export type PlanCode = "starter" | "growth" | "enterprise";
export type CapabilityKey = "billing.subscription_checkout" | "billing.payment_link_topup";

const PLAN_CAPABILITIES: Record<PlanCode, CapabilityKey[]> = {
  starter: ["billing.subscription_checkout"],
  growth: ["billing.subscription_checkout", "billing.payment_link_topup"],
  enterprise: ["billing.subscription_checkout", "billing.payment_link_topup"],
};

export function isValidPlan(plan?: string | null): plan is PlanCode {
  return !!plan && (plan === "starter" || plan === "growth" || plan === "enterprise");
}

export function getDefaultPlan() {
  return "starter" as const;
}

export function getPlanCapabilities(plan?: PlanCode | null): CapabilityKey[] {
  const normalized: PlanCode = plan && isValidPlan(plan) ? plan : getDefaultPlan();
  return PLAN_CAPABILITIES[normalized];
}

export function hasPlanCapability(plan: PlanCode | null | undefined, capability: CapabilityKey) {
  return getPlanCapabilities(plan).includes(capability);
}
