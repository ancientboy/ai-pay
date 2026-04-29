import { NextRequest, NextResponse } from "next/server";
import {
  type AdminAuditAction,
  listAdminAuditLogs,
} from "@/lib/admin-audit-store";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function message(request: NextRequest, code: "AUTH-007") {
  const en = isEnglish(request);
  if (code === "AUTH-007") {
    return en ? "Admin role required" : "需要管理员权限";
  }
  return en ? "forbidden" : "无权限";
}

function parseAdminAuditAction(value: string): AdminAuditAction | undefined {
  if (
    value === "admin.user.create" ||
    value === "admin.user.enable" ||
    value === "admin.user.disable" ||
    value === "admin.user.reset_password"
  ) {
    return value;
  }
  return undefined;
}

export async function GET(request: NextRequest) {
  const sessionToken = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(sessionToken);
  if (!claims || (claims.role || "").toLowerCase() !== "admin") {
    return NextResponse.json(
      { code: "AUTH-007", message: message(request, "AUTH-007") },
      { status: 403 },
    );
  }
  const limitRaw = request.nextUrl.searchParams.get("limit") ?? "";
  const offsetRaw = request.nextUrl.searchParams.get("offset") ?? "";
  const actionRaw = (request.nextUrl.searchParams.get("action") ?? "").trim();
  const actor = (request.nextUrl.searchParams.get("actor") ?? "").trim();
  const targetUsername = (request.nextUrl.searchParams.get("targetUsername") ?? "").trim();
  const limit = Number.parseInt(limitRaw, 10);
  const offset = Number.parseInt(offsetRaw, 10);
  const action = parseAdminAuditAction(actionRaw);
  const result = await listAdminAuditLogs({
    limit: Number.isFinite(limit) ? limit : 50,
    offset: Number.isFinite(offset) ? offset : 0,
    action,
    actor: actor || undefined,
    targetUsername: targetUsername || undefined,
  });
  return NextResponse.json({
    code: "0",
    data: result.items,
    meta: { total: result.total, limit: result.limit, offset: result.offset },
  });
}
