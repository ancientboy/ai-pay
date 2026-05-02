import type { SessionClaims } from "@/lib/session";

const ALL_CONSOLE_ROUTES = [
  "/dashboard",
  "/billing",
  "/agents",
  "/authorize",
  "/recharge",
  "/transactions",
  "/developer",
  "/settings",
] as const;

type Role = "admin" | "operator" | "readonly";
type Feature =
  | "billing.subscription.checkout"
  | "billing.payment_link.checkout"
  | "billing.reconciliation.export"
  | "billing.checkout.create"
  | "billing.bridge.admin"
  | "billing.admin_full";

function normalizePlan(plan?: string): "starter" | "growth" | "enterprise" {
  if (plan === "growth" || plan === "enterprise") {
    return plan;
  }
  return "starter";
}

function normalizeRole(role?: string): Role {
  if (role === "admin" || role === "readonly") {
    return role;
  }
  return "operator";
}

export function canAccessPath(role: string | undefined, path: string) {
  const normalized = normalizeRole(role);
  if (normalized === "admin") {
    return true;
  }
  if (normalized === "operator") {
    return !path.startsWith("/developer");
  }
  // readonly
  if (path.startsWith("/developer")) {
    return false;
  }
  if (path.startsWith("/authorize")) {
    return false;
  }
  return true;
}

export function visibleConsoleNav(session: SessionClaims) {
  const role = normalizeRole(session.role);
  return ALL_CONSOLE_ROUTES.filter((href) => canAccessPath(role, href));
}

export function canUseFeature(
  role: string | undefined,
  plan: string | undefined,
  feature: Feature,
) {
  const normalized = normalizeRole(role);
  if (normalized === "admin") {
    return true;
  }
  // Tenant-facing operators never manage org-wide billing checkout / Bridge admin flows.
  if (
    feature === "billing.subscription.checkout" ||
    feature === "billing.payment_link.checkout" ||
    feature === "billing.checkout.create" ||
    feature === "billing.bridge.admin" ||
    feature === "billing.admin_full"
  ) {
    return false;
  }
  if (feature === "billing.reconciliation.export") {
    return normalized !== "readonly";
  }
  return false;
}

export function canMutateBackendPath(
  role: string | undefined,
  plan: string | undefined,
  method: string,
  path: string[],
) {
  const normalizedRole = normalizeRole(role);
  if (method === "GET" || method === "HEAD") {
    return true;
  }
  if (normalizedRole === "admin") {
    return true;
  }

  const route = `/${path.join("/")}`;
  const normalizedPlan = normalizePlan(plan);

  if (normalizedRole === "readonly") {
    return false;
  }

  // operator
  if (route.startsWith("/developer/")) {
    return false;
  }
  if (route.startsWith("/billing/provider/bridge/")) {
    return false;
  }
  if (route === "/payment/unfreeze" || route === "/payment/refund") {
    return false;
  }
  if (route === "/fund/transfer" || route === "/fund/withdraw") {
    return false;
  }
  if (route === "/payment/x402/transfer") {
    return false;
  }
  if (route === "/billing/checkout/create" && normalizedPlan === "starter") {
    return false;
  }
  return true;
}
