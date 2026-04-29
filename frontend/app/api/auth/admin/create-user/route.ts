import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { registerStoredUser } from "@/lib/user-store";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function message(
  request: NextRequest,
  code: "AUTH-001" | "AUTH-004" | "AUTH-005" | "AUTH-006" | "AUTH-007",
) {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username and password are required" : "用户名和密码不能为空";
  }
  if (code === "AUTH-004") {
    return en ? "Username already exists" : "用户名已存在";
  }
  if (code === "AUTH-005") {
    return en ? "Username must be at least 3 characters" : "用户名长度至少 3 位";
  }
  if (code === "AUTH-006") {
    return en ? "Password must be at least 6 characters" : "密码长度至少 6 位";
  }
  return en ? "Admin role required" : "需要管理员权限";
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
    | { username?: string; password?: string; role?: string }
    | null;
  const username = body?.username?.trim() ?? "";
  const password = body?.password?.trim() ?? "";
  const role = body?.role?.trim() || "operator";

  if (!username || !password) {
    return NextResponse.json(
      { code: "AUTH-001", message: message(request, "AUTH-001") },
      { status: 400 },
    );
  }

  const result = await registerStoredUser({ username, password, role });
  if (!result.ok) {
    const code = result.code === "AUTH-004" ? "AUTH-004" : result.code === "AUTH-006" ? "AUTH-006" : "AUTH-005";
    return NextResponse.json(
      { code, message: message(request, code) },
      { status: 400 },
    );
  }

  return NextResponse.json({ code: "0", data: result.user });
}
