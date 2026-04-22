import { NextResponse } from "next/server";

export async function POST() {
  const response = NextResponse.json({ code: "0", message: "ok" });
  response.cookies.set("ai_pay_session", "", {
    httpOnly: false,
    sameSite: "lax",
    secure: false,
    path: "/",
    maxAge: 0,
  });
  return response;
}
