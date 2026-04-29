import { NextRequest, NextResponse } from "next/server";
import { listAdminAuditLogs } from "@/lib/admin-audit-store";
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
  const limit = Number.parseInt(limitRaw, 10);
  const logs = await listAdminAuditLogs(Number.isFinite(limit) ? limit : 50);
  return NextResponse.json({ code: "0", data: logs });
}
