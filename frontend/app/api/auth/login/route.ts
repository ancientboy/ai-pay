import { NextRequest, NextResponse } from "next/server";
import {
  createSessionToken,
  SESSION_COOKIE_NAME,
  sessionCookieOptions,
} from "@/lib/session";

const ADMIN_USERNAME = process.env.AI_PAY_ADMIN_USERNAME ?? "admin";
const ADMIN_PASSWORD = process.env.AI_PAY_ADMIN_PASSWORD ?? "admin123";

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

  const ip = clientIP(request);
  if (isRateLimited(ip)) {
    return NextResponse.json(
      { code: "AUTH-003", message: "登录尝试过于频繁，请稍后重试" },
      { status: 429 },
    );
  }

  if (username !== ADMIN_USERNAME || password !== ADMIN_PASSWORD) {
    return NextResponse.json(
      { code: "AUTH-002", message: "用户名或密码错误" },
      { status: 401 },
    );
  }

  const token = await createSessionToken(username);
  loginAttempts.delete(ip);
  const response = NextResponse.json({ code: "0", message: "ok" });
  response.cookies.set(SESSION_COOKIE_NAME, token, sessionCookieOptions());
  return response;
}
