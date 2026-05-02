import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { canUseFeature } from "@/lib/rbac";

function requireAdmin(claims: Awaited<ReturnType<typeof parseSessionToken>>) {
  if (claims?.role !== "admin") {
    return NextResponse.json(
      { code: "AUTH-008", message: "仅管理员可执行此操作" },
      { status: 403 },
    );
  }
  return null;
}

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";

function resolveBaseURL(request: NextRequest) {
  const override = request.headers.get("x-api-base-url");
  if (override && /^https?:\/\//.test(override)) {
    return override;
  }
  return API_BASE_URL;
}

async function proxy(request: NextRequest, path: string[]) {
  const baseURL = resolveBaseURL(request);
  const url = new URL(`${baseURL}/${path.join("/")}`);
  request.nextUrl.searchParams.forEach((value, key) => {
    url.searchParams.set(key, value);
  });

  const headers = new Headers();
  ["content-type", "idempotency-key", "x-sign-timestamp", "x-request-id"].forEach(
    (name) => {
      const value = request.headers.get(name);
      if (value) {
        headers.set(name, value);
      }
    },
  );

  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  const userId = claims?.sub?.trim();
  if (userId && !headers.has("x-user-id")) {
    headers.set("X-User-Id", userId);
  }
  const tenantId = claims?.tenantId?.trim();
  if (tenantId && !headers.has("x-tenant-id")) {
    headers.set("X-Tenant-Id", tenantId);
  }

  const body =
    request.method === "GET" || request.method === "HEAD"
      ? undefined
      : await request.text();

  // Action-level RBAC guard on write APIs.
  if (request.method !== "GET" && request.method !== "HEAD") {
    const apiPath = `/${path.join("/")}`;
    const role = claims?.role;
    const forbiddenByRole =
      role === "readonly" &&
      (apiPath.startsWith("/authorize/") ||
        apiPath.startsWith("/developer/") ||
        apiPath === "/fund/recharge" ||
        apiPath === "/account/va/transfer" ||
        apiPath.startsWith("/payment/"));
    if (forbiddenByRole) {
      return NextResponse.json(
        { code: "AUTH-009", message: "当前角色仅可读，无法执行写操作" },
        { status: 403 },
      );
    }
  }

  // Org-wide billing checkout: administrators only (tenant operators use payment features elsewhere).
  if (
    request.method === "POST" &&
    path.length === 3 &&
    path[0] === "billing" &&
    path[1] === "checkout" &&
    path[2] === "create"
  ) {
    const denied = requireAdmin(claims);
    if (denied) {
      return denied;
    }
    let checkoutType = "";
    try {
      const parsed = body ? (JSON.parse(body) as { checkoutType?: string }) : null;
      checkoutType = (parsed?.checkoutType ?? "").trim();
    } catch {
      checkoutType = "";
    }
    const feature =
      checkoutType === "payment_link"
        ? "billing.payment_link.checkout"
        : "billing.subscription.checkout";
    if (!canUseFeature(claims?.role, claims?.planCode, feature)) {
      return NextResponse.json(
        {
          code: "AUTH-008",
          message:
            feature === "billing.payment_link.checkout"
              ? "当前套餐暂不支持支付链接充值，请升级套餐"
              : "当前套餐暂不支持创建订阅结账，请升级套餐",
        },
        { status: 403 },
      );
    }
  }

  if (
    path.length >= 3 &&
    path[0] === "billing" &&
    path[1] === "provider" &&
    path[2] === "bridge"
  ) {
    const denied = requireAdmin(claims);
    if (denied) {
      return denied;
    }
  }

  const response = await fetch(url.toString(), {
    method: request.method,
    headers,
    body,
    cache: "no-store",
  });

  const text = await response.text();
  return new NextResponse(text, {
    status: response.status,
    headers: {
      "content-type": response.headers.get("content-type") ?? "application/json",
      "x-request-id": response.headers.get("x-request-id") ?? "",
    },
  });
}

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> },
) {
  const { path } = await params;
  return proxy(request, path);
}
