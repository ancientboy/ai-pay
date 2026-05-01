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
