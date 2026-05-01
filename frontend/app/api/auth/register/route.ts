import { NextRequest, NextResponse } from "next/server";
import { upsertUser } from "@/lib/auth-users";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function authMessage(request: NextRequest, code: "AUTH-001" | "AUTH-004" | "AUTH-005") {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username and password are required" : "用户名和密码不能为空";
  }
  if (code === "AUTH-004") {
    return en ? "Username already exists" : "用户名已存在";
  }
  return en
    ? "Username must be 3-32 chars and password must be at least 8 chars"
    : "用户名需 3-32 位，密码至少 8 位";
}

function isValidUsername(username: string) {
  return /^[a-zA-Z0-9_]{3,32}$/.test(username);
}

export async function POST(request: NextRequest) {
  const body = (await request.json().catch(() => null)) as
    | { username?: string; password?: string }
    | null;
  const username = body?.username?.trim() ?? "";
  const password = body?.password?.trim() ?? "";

  if (!username || !password) {
    return NextResponse.json(
      { code: "AUTH-001", message: authMessage(request, "AUTH-001") },
      { status: 400 },
    );
  }
  if (!isValidUsername(username) || password.length < 8) {
    return NextResponse.json(
      { code: "AUTH-005", message: authMessage(request, "AUTH-005") },
      { status: 400 },
    );
  }

  const result = await upsertUser({
    username,
    password,
    role: "operator",
  });
  if (!result.ok) {
    return NextResponse.json(
      { code: "AUTH-004", message: authMessage(request, "AUTH-004") },
      { status: 409 },
    );
  }
  return NextResponse.json({ code: "0", message: "ok" });
}
