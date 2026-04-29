import { NextRequest, NextResponse } from "next/server";
import { appendAdminAuditLog } from "@/lib/admin-audit-store";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { resetStoredUserPassword } from "@/lib/user-store";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function message(
  request: NextRequest,
  code: "AUTH-001" | "AUTH-006" | "AUTH-007" | "AUTH-008",
) {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username and password are required" : "用户名和密码不能为空";
  }
  if (code === "AUTH-006") {
    return en ? "Password must be at least 6 characters" : "密码长度至少 6 位";
  }
  if (code === "AUTH-007") {
    return en ? "Admin role required" : "需要管理员权限";
  }
  return en ? "User not found" : "用户不存在";
}

export async function POST(request: NextRequest) {
  const sessionToken = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(sessionToken);
  if (!claims || (claims.role || "").toLowerCase() !== "admin") {
    return NextResponse.json(
      { code: "AUTH-007", message: message(request, "AUTH-007") },
      { status: 403 },
    );
  }

  const body = (await request.json().catch(() => null)) as
    | { username?: string; password?: string }
    | null;
  const username = body?.username?.trim() ?? "";
  const password = body?.password?.trim() ?? "";
  if (!username || !password) {
    return NextResponse.json(
      { code: "AUTH-001", message: message(request, "AUTH-001") },
      { status: 400 },
    );
  }

  const result = await resetStoredUserPassword(username, password);
  if (!result.ok) {
    const code = result.code === "AUTH-006" ? "AUTH-006" : "AUTH-008";
    return NextResponse.json(
      { code, message: message(request, code) },
      { status: 400 },
    );
  }

  await appendAdminAuditLog({
    actor: claims.sub,
    action: "admin.user.reset_password",
    targetUsername: username,
    detail: { via: "api.auth.admin.reset-password" },
  });

  return NextResponse.json({ code: "0", data: result.user });
}
