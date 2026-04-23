import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { verifySessionToken } from "@/lib/session";

const protectedPaths = [
  "/dashboard",
  "/agents",
  "/authorize",
  "/recharge",
  "/transactions",
  "/settings",
  "/developer",
];

export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (!protectedPaths.some((path) => pathname.startsWith(path))) {
    return NextResponse.next();
  }

  const session = request.cookies.get("ai_pay_session")?.value;
  return (async () => {
    if (await verifySessionToken(session)) {
      return NextResponse.next();
    }
    const loginURL = new URL("/login", request.url);
    loginURL.searchParams.set("next", pathname);
    return NextResponse.redirect(loginURL);
  })();
}

export const config = {
  matcher: [
    "/dashboard/:path*",
    "/agents/:path*",
    "/authorize/:path*",
    "/recharge/:path*",
    "/transactions/:path*",
    "/settings/:path*",
    "/developer/:path*",
  ],
};
