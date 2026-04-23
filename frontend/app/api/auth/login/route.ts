import { NextRequest, NextResponse } from "next/server";
import {
  createSessionToken,
  SESSION_COOKIE_NAME,
  sessionCookieOptions,
} from "@/lib/session";

const ADMIN_USERNAME = process.env.AI_PAY_ADMIN_USERNAME ?? "admin";
const ADMIN_PASSWORD = process.env.AI_PAY_ADMIN_PASSWORD ?? "admin123";
const DEFAULT_ROLE = process.env.AI_PAY_DEFAULT_ROLE ?? "operator";

const loginAttempts = new Map<
  string,
  {
    count: number;
    windowStartMs: number;
  }
>();

const LOGIN_WINDOW_MS = 60 * 1000;
const MAX_ATTEMPTS_PER_WINDOW = 10;

function clientIP(request: NextRequest) {
  const xff = request.headers.get("x-forwarded-for");
  if (xff) {
    return xff.split(",")[0]?.trim() || "unknown";
  }
  return request.headers.get("x-real-ip") || "unknown";
}

function isRateLimited(ip: string) {
  const now = Date.now();
  const current = loginAttempts.get(ip);
  if (!current || now-current.windowStartMs >= LOGIN_WINDOW_MS) {
    loginAttempts.set(ip, { count: 1, windowStartMs: now });
    return false;
  }
  current.count += 1;
  loginAttempts.set(ip, current);
  return current.count > MAX_ATTEMPTS_PER_WINDOW;
}

function isEnglish(request: NextRequest) {
  const language = request.headers.get("accept-language")?.toLowerCase() ?? "";
  return language.startsWith("en");
}

function authMessage(
  request: NextRequest,
  code: "AUTH-001" | "AUTH-002" | "AUTH-003",
) {
  const en = isEnglish(request);
  if (code === "AUTH-001") {
    return en ? "Username and password are required" : "用户名和密码不能为空";
  }
  if (code === "AUTH-002") {
    return en ? "Invalid username or password" : "用户名或密码错误";
  }
  return en
    ? "Too many login attempts, please retry later"
    : "登录尝试过于频繁，请稍后重试";
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

  const ip = clientIP(request);
  if (isRateLimited(ip)) {
    return NextResponse.json(
      { code: "AUTH-003", message: authMessage(request, "AUTH-003") },
      { status: 429 },
    );
  }

  if (username !== ADMIN_USERNAME || password !== ADMIN_PASSWORD) {
    return NextResponse.json(
      { code: "AUTH-002", message: authMessage(request, "AUTH-002") },
      { status: 401 },
    );
  }

  const token = await createSessionToken(username, DEFAULT_ROLE);
  loginAttempts.delete(ip);
  const response = NextResponse.json({ code: "0", message: "ok" });
  response.cookies.set(SESSION_COOKIE_NAME, token, sessionCookieOptions());
  return response;
}
