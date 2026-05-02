import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { findUserByUsername, updateUserPlan } from "@/lib/auth-users";
import {
  getPlanCapabilities,
  normalizePlanCode,
  type PlanCode,
} from "@/lib/plan-capabilities";

export async function GET(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  if (!claims?.sub) {
    return NextResponse.json({ code: "AUTH-001", message: "unauthorized" }, { status: 401 });
  }
  const username = claims.sub;
  const user = await findUserByUsername(username);
  let plan: PlanCode = normalizePlanCode(user?.planCode ?? claims.planCode);

  const backendBaseURL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://127.0.0.1:8080";
  try {
    const res = await fetch(`${backendBaseURL}/billing/subscription`, {
      method: "GET",
      headers: {
        "X-User-Id": username,
      },
      cache: "no-store",
    });
    if (res.ok) {
      const payload = (await res.json()) as {
        code?: string;
        data?: { subscription?: { status?: string; planCode?: string } | null };
      };
      const sub = payload?.data?.subscription;
      const active = sub?.status === "active";
      if (active && sub?.planCode) {
        const subscribedPlan = normalizePlanCode(sub.planCode);
        if (subscribedPlan !== plan) {
          await updateUserPlan(username, subscribedPlan);
          plan = subscribedPlan;
        }
      }
      // 无有效订阅时不强行写回 starter，保留用户档案中的 free/starter 等（避免误降级）
    }
  } catch {
    // keep last known plan on transient backend issues
  }

  return NextResponse.json({
    code: "0",
    data: {
      username,
      role: claims.role ?? "operator",
      tenantId: claims.tenantId ?? "tenant_default",
      subscriptionPlan: plan,
      planCapabilities: getPlanCapabilities(plan),
    },
  });
}
