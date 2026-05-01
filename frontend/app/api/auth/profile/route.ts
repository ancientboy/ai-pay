import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";
import { findUserByUsername, updateUserPlan } from "@/lib/auth-users";
import { getPlanCapabilities, type PlanCode } from "@/lib/plan-capabilities";

function normalizeSubscriptionPlan(planCode?: string | null): PlanCode {
  if (planCode === "growth" || planCode === "enterprise") {
    return planCode;
  }
  return "starter";
}

export async function GET(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  if (!claims?.sub) {
    return NextResponse.json({ code: "AUTH-001", message: "unauthorized" }, { status: 401 });
  }
  const username = claims.sub;
  let plan = normalizeSubscriptionPlan(claims.planCode);
  const user = await findUserByUsername(username);
  if (user?.planCode) {
    plan = normalizeSubscriptionPlan(user.planCode);
  }

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
      const subscribedPlan = normalizeSubscriptionPlan(sub?.planCode);
      const syncedPlan = active ? subscribedPlan : "starter";
      if (syncedPlan !== plan) {
        await updateUserPlan(username, syncedPlan);
        plan = syncedPlan;
      }
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
