import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest) {
  const body = (await request.json().catch(() => null)) as
    | { username?: string; password?: string }
    | null;
  const username = body?.username?.trim() ?? "";
  const password = body?.password?.trim() ?? "";

  if (!username || !password) {
    return NextResponse.json(
      { code: "AUTH-001", message: "用户名和密码不能为空" },
      { status: 400 },
    );
  }

  const response = NextResponse.json({ code: "0", message: "ok" });
  response.cookies.set("ai_pay_session", "1", {
    httpOnly: false,
    sameSite: "lax",
    secure: false,
    path: "/",
    maxAge: 60 * 60 * 24,
  });
  return response;
}
