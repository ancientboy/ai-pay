import { NextRequest, NextResponse } from "next/server";
import { registerStoredUser } from "@/lib/user-store";

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function message(
  request: NextRequest,
  code: "AUTH-001" | "AUTH-004" | "AUTH-005",
) {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username and password are required" : "用户名和密码不能为空";
  }
  if (code === "AUTH-004") {
    return en ? "Username already exists" : "用户名已存在";
  }
  return en ? "Username must be at least 3 characters" : "用户名长度至少 3 位";
}

export async function POST(request: NextRequest) {
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
    if (result.code === "AUTH-004") {
      return NextResponse.json(
        { code: "AUTH-004", message: message(request, "AUTH-004") },
        { status: 400 },
      );
    }
    if (result.message === "username too short") {
      return NextResponse.json(
        { code: "AUTH-005", message: message(request, "AUTH-005") },
        { status: 400 },
      );
    }
    return NextResponse.json(
      { code: "AUTH-001", message: message(request, "AUTH-001") },
      { status: 400 },
    );
  }
  return NextResponse.json({ code: "0", data: result.user });
}
