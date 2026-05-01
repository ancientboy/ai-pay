import { NextRequest, NextResponse } from "next/server";
import { parseSessionToken, SESSION_COOKIE_NAME } from "@/lib/session";

export async function GET(request: NextRequest) {
  const token = request.cookies.get(SESSION_COOKIE_NAME)?.value;
  const claims = await parseSessionToken(token);
  if (!claims?.sub) {
    return NextResponse.json({ code: "AUTH-001", message: "unauthorized" }, { status: 401 });
  }
  return NextResponse.json({
    code: "0",
    data: {
      username: claims.sub,
      role: claims.role ?? "operator",
      tenantId: claims.tenantId ?? "tenant_default",
      subscriptionPlan: claims.planCode ?? "starter",
    },
  });
}
