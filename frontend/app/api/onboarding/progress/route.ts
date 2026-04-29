import { NextRequest, NextResponse } from "next/server";
import { getCurrentSession } from "@/lib/current-session";
import { getStoredUserOnboarding, updateStoredUserOnboarding } from "@/lib/user-store";

function isStep(value: string) {
  return (
    value === "agent" ||
    value === "kyc" ||
    value === "recharge" ||
    value === "authorize" ||
    value === "pay" ||
    value === "selfhosted"
  );
}

export async function GET() {
  const session = await getCurrentSession();
  if (!session?.sub) {
    return NextResponse.json({ code: "AUTH-007", message: "unauthorized" }, { status: 401 });
  }
  const onboarding = await getStoredUserOnboarding(session.sub);
  if (!onboarding) {
    return NextResponse.json({ code: "AUTH-008", message: "user not found" }, { status: 404 });
  }
  return NextResponse.json({
    code: "0",
    data: {
      username: session.sub,
      agent: !!onboarding.agent,
      kyc: !!onboarding.kyc,
      recharge: !!onboarding.recharge,
      pay: !!onboarding.pay,
      authorize: !!onboarding.authorize,
      selfhosted: !!onboarding.selfhosted,
      dismissed: !!onboarding.dismissed,
      updatedAt: onboarding.updatedAt,
    },
  });
}

export async function POST(request: NextRequest) {
  const session = await getCurrentSession();
  if (!session?.sub) {
    return NextResponse.json({ code: "AUTH-007", message: "unauthorized" }, { status: 401 });
  }
  const body = (await request.json().catch(() => null)) as
    | { step?: string; done?: boolean; dismissed?: boolean }
    | null;
  const step = (body?.step ?? "").trim().toLowerCase();
  const hasDismissed = typeof body?.dismissed === "boolean";
  if (!hasDismissed && !isStep(step)) {
    return NextResponse.json({ code: "PAY-010", message: "invalid step" }, { status: 400 });
  }
  const done = body?.done !== false;
  const result = await updateStoredUserOnboarding({
    username: session.sub,
    dismissed: hasDismissed ? body?.dismissed : undefined,
    completedStep: done && isStep(step) ? step : undefined,
  });
  if (!result.ok) {
    return NextResponse.json({ code: result.code, message: "update onboarding failed" }, { status: 400 });
  }
  return NextResponse.json({
    code: "0",
    data: {
      username: session.sub,
      agent: !!result.onboarding?.agent,
      kyc: !!result.onboarding?.kyc,
      recharge: !!result.onboarding?.recharge,
      pay: !!result.onboarding?.pay,
      authorize: !!result.onboarding?.authorize,
      selfhosted: !!result.onboarding?.selfhosted,
      dismissed: !!result.onboarding?.dismissed,
      updatedAt: result.onboarding?.updatedAt,
    },
  });
}
