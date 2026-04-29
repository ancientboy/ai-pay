import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { setStoredUserStatus } from "@/lib/user-store";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function message(request: NextRequest, code: "AUTH-001" | "AUTH-007" | "AUTH-008") {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username is required" : "用户名不能为空";
  }
  if (code === "AUTH-008") {
    return en ? "User not found" : "用户不存在";
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
    | { username?: string; status?: "active" | "disabled" }
    | null;
  const username = body?.username?.trim() ?? "";
  if (!username) {
    return NextResponse.json(
      { code: "AUTH-001", message: message(request, "AUTH-001") },
      { status: 400 },
    );
  }
  const status = (body?.status ?? "").toString().trim().toLowerCase();
  if (status !== "active" && status !== "disabled") {
    return NextResponse.json(
      { code: "AUTH-005", message: isEnglish(request) ? "Invalid status value" : "状态值非法" },
      { status: 400 },
    );
  }
  const ok = await setStoredUserStatus(username, status as "active" | "disabled");
  if (!ok) {
    return NextResponse.json(
      { code: "AUTH-008", message: message(request, "AUTH-008") },
      { status: 404 },
    );
  }
  return NextResponse.json({ code: "0", data: { username, status } });
}
