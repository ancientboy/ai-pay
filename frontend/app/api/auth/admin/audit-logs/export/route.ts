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

function toCsvValue(raw: unknown) {
  const value = String(raw ?? "");
  if (value.includes(",") || value.includes("\"") || value.includes("\n")) {
    return `"${value.replaceAll("\"", "\"\"")}"`;
  }
  return value;
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
  const actionRaw = (request.nextUrl.searchParams.get("action") ?? "").trim();
  const actor = (request.nextUrl.searchParams.get("actor") ?? "").trim();
  const targetUsername = (request.nextUrl.searchParams.get("targetUsername") ?? "").trim();
  const action = parseAdminAuditAction(actionRaw);
  const result = await listAdminAuditLogs({
    limit: 500,
    offset: 0,
    action,
    actor: actor || undefined,
    targetUsername: targetUsername || undefined,
  });
  const lines = ["id,createdAt,actor,action,targetUsername,detailJson"];
  for (const item of result.items) {
    lines.push(
      [
        toCsvValue(item.id),
        toCsvValue(item.createdAt),
        toCsvValue(item.actor),
        toCsvValue(item.action),
        toCsvValue(item.targetUsername),
        toCsvValue(JSON.stringify(item.detail ?? {})),
      ].join(","),
    );
  }
  const content = `${lines.join("\n")}\n`;
  return new NextResponse(content, {
    status: 200,
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Content-Disposition": "attachment; filename=admin_audit_logs.csv",
      "Cache-Control": "no-store",
    },
  });
}
