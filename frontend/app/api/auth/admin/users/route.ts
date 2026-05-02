import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import {
  listUsers,
  resetUserPassword,
  setUserDisabled,
  updateUserRole,
  updateUserPlan,
  upsertUser,
} from "@/lib/auth-users";
import {
  getDefaultPlanForSignup,
  getPlanCapabilities,
  isValidPlan,
  type PlanCode,
} from "@/lib/plan-capabilities";
import { appendAdminAuditLog, listAdminAuditLogs } from "@/lib/admin-audit-log";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function authMessage(request: NextRequest, code: "AUTH-006" | "AUTH-004" | "AUTH-005") {
  const en = isEnglish(request);
  if (code === "AUTH-006") {
    return en ? "Admin permission required" : "需要管理员权限";
  }
  if (code === "AUTH-004") {
    return en ? "Username already exists" : "用户名已存在";
  }
  return en
    ? "Username must be 3-32 chars and password must be at least 8 chars"
    : "用户名需 3-32 位，密码至少 8 位";
}

function authMessagePlan(request: NextRequest) {
  return isEnglish(request)
    ? "Plan must be one of: free, starter, growth, enterprise"
    : "套餐必须是 free、starter、growth、enterprise 之一";
}

function isValidUsername(username: string) {
  return /^[a-zA-Z0-9_]{3,32}$/.test(username);
}

async function requireAdmin(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  return claims?.role === "admin" ? claims.sub : null;
}

export async function GET(request: NextRequest) {
  const adminActor = await requireAdmin(request);
  if (!adminActor) {
    return NextResponse.json(
      { code: "AUTH-006", message: authMessage(request, "AUTH-006") },
      { status: 403 },
    );
  }
  const view = request.nextUrl.searchParams.get("view");
  if (view === "audit") {
    const logs = await listAdminAuditLogs();
    return NextResponse.json({ code: "0", data: { logs } });
  }
  const users = await listUsers();
  return NextResponse.json({ code: "0", data: { users } });
}

export async function POST(request: NextRequest) {
  const adminActor = await requireAdmin(request);
  if (!adminActor) {
    return NextResponse.json(
      { code: "AUTH-006", message: authMessage(request, "AUTH-006") },
      { status: 403 },
    );
  }
  const body = (await request.json().catch(() => null)) as
    | {
        action?: "create" | "set_disabled" | "reset_password" | "set_role";
        username?: string;
        password?: string;
        newPassword?: string;
        disabled?: boolean;
        role?: "admin" | "operator" | "readonly";
        tenantId?: string;
        plan?: PlanCode;
      }
    | null;
  const action = body?.action;
  const username = body?.username?.trim() ?? "";

  if (!action || !username || !isValidUsername(username)) {
    return NextResponse.json(
      { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
      { status: 400 },
    );
  }

  if (action === "create") {
    const password = body?.password?.trim() ?? "";
    const role = body?.role === "readonly" ? "readonly" : "operator";
    const tenantId = body?.tenantId?.trim() || "default";
    const plan = body?.plan?.trim() as PlanCode | undefined;
    if (password.length < 8) {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
        { status: 400 },
      );
    }
    if (plan && !isValidPlan(plan)) {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessagePlan(request) },
        { status: 400 },
      );
    }
    const created = await upsertUser({
      username,
      password,
      role,
      tenantId,
      plan: plan ?? getDefaultPlanForSignup(),
    });
    if (!created.ok) {
      return NextResponse.json(
        { code: "AUTH-004", message: authMessage(request, "AUTH-004") },
        { status: 409 },
      );
    }
    await appendAdminAuditLog({
      actor: adminActor,
      action: "admin.user.create",
      targetUsername: username,
      detail: { role, tenantId, plan: plan ?? getDefaultPlanForSignup() },
    });
    return NextResponse.json({ code: "0", message: "ok" });
  }

  if (action === "set_disabled") {
    await setUserDisabled(username, !!body?.disabled);
    await appendAdminAuditLog({
      actor: adminActor,
      action: body?.disabled ? "admin.user.disable" : "admin.user.enable",
      targetUsername: username,
      detail: { disabled: !!body?.disabled },
    });
    return NextResponse.json({ code: "0", message: "ok" });
  }

  if (action === "reset_password") {
    const newPassword = body?.newPassword?.trim() ?? "";
    if (newPassword.length < 8) {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
        { status: 400 },
      );
    }
    await resetUserPassword(username, newPassword);
    await appendAdminAuditLog({
      actor: adminActor,
      action: "admin.user.reset_password",
      targetUsername: username,
      detail: { via: "api.auth.admin.users" },
    });
    return NextResponse.json({ code: "0", message: "ok" });
  }

  if (action === "set_role") {
    const role = body?.role;
    if (role !== "admin" && role !== "operator" && role !== "readonly") {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
        { status: 400 },
      );
    }
    await updateUserRole(username, role);
    await appendAdminAuditLog({
      actor: adminActor,
      action: "admin.user.set_role",
      targetUsername: username,
      detail: { role },
    });
    return NextResponse.json({ code: "0", message: "ok" });
  }

  if (action === "set_plan") {
    const plan = body?.plan?.trim() as PlanCode | undefined;
    if (!plan || !isValidPlan(plan)) {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessagePlan(request) },
        { status: 400 },
      );
    }
    const updated = await updateUserPlan(username, plan);
    if (!updated.ok) {
      return NextResponse.json(
        { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
        { status: 400 },
      );
    }
    await appendAdminAuditLog({
      actor: adminActor,
      action: "admin.user.set_plan",
      targetUsername: username,
      detail: { plan, capabilities: getPlanCapabilities(plan) },
    });
    return NextResponse.json({ code: "0", message: "ok", data: getPlanCapabilities(plan) });
  }

  return NextResponse.json(
    { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
    { status: 400 },
  );
}
